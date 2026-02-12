package transformer

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	k8sv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/apimachinery/pkg/runtime"

	virtv1 "kubevirt.io/api/core/v1"
	"kubevirt.io/kubevirt/pkg/util"
	"kubevirt.io/kubevirt/pkg/virt-api/webhooks/mutating-webhook/mutators"
	"kubevirt.io/kubevirt/pkg/network/vmispec"
	"kubevirt.io/kubevirt/pkg/testutils"
	"kubevirt.io/kubevirt/pkg/virt-controller/services"
	vmCtrl "kubevirt.io/kubevirt/pkg/virt-controller/watch/vm"
	virtconfig "kubevirt.io/kubevirt/pkg/virt-config"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/tools/cache"
	"kubevirt.io/kubevirt/pkg/defaults"

    extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"


)

type VMToPodTransformer struct {
	ClusterConfig 		*virtconfig.ClusterConfig
	TemplateSvc   		*services.TemplateService
	LauncherImage 		string
	InstancetypeFile 	string
	PreferenceFile   	string
	AddConsoleProxy 	bool
	ProxyImage      	string
	ProxyPort       	int
}

type TransformerOption func(*VMToPodTransformer)

func WithLauncherImage(image string) TransformerOption {
	return func(t *VMToPodTransformer) {
		t.LauncherImage = image
	}
}

func WithInstancetypeFile(file string) TransformerOption {
	return func(t *VMToPodTransformer) {
		t.InstancetypeFile = file
	}
}

func WithPreferenceFile(file string) TransformerOption {
	return func(t *VMToPodTransformer) {
		t.PreferenceFile = file
	}
}

func WithAddConsoleProxy(enabled bool, image string, port int) TransformerOption {
	return func(t *VMToPodTransformer) {
		t.AddConsoleProxy = enabled
		t.ProxyImage = image
		t.ProxyPort = port
	}
}

func NewVMToPodTransformer(opts ...TransformerOption) *VMToPodTransformer {
	kv := &virtv1.KubeVirt{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubevirt",
			Namespace: "kubevirt",
		},
		Spec: virtv1.KubeVirtSpec{
			Configuration: virtv1.KubeVirtConfiguration{
				DeveloperConfiguration: &virtv1.DeveloperConfiguration{},
				VirtualMachineOptions: &virtv1.VirtualMachineOptions{
					DisableSerialConsoleLog: &virtv1.DisableSerialConsoleLog{},
				},
			},
		},
		Status: virtv1.KubeVirtStatus{
			Phase: virtv1.KubeVirtPhaseDeploying,
		},
	}
	kv.Spec.Configuration.DeveloperConfiguration.FeatureGates = []string{"ImageVolume"}

	crdInformer, _ := testutils.NewFakeInformerFor(&extv1.CustomResourceDefinition{})
	kvInformer, _ := testutils.NewFakeInformerFor(kv)
	config, _ := virtconfig.NewClusterConfig(crdInformer, kvInformer, "default")

    pvcCache := cache.NewIndexer(cache.DeletionHandlingMetaNamespaceKeyFunc, nil)
    resourceQuotaStore := cache.NewStore(cache.DeletionHandlingMetaNamespaceKeyFunc)
    namespaceStore := cache.NewStore(cache.DeletionHandlingMetaNamespaceKeyFunc)

	launcherImage := "quay.io/kubevirt/virt-launcher:latest"

	templateSvc := services.NewTemplateService(
		launcherImage,
		240,
		"/var/run/kubevirt",
		"/var/run/kubevirt-ephemeral-disks",
		"/var/run/kubevirt/container-disks",
		virtv1.HotplugDiskDir,
		"pull-secret-1",
		pvcCache,
		nil,
		config,
		107,
		"quay.io/kubevirt/vm-export:latest",
		resourceQuotaStore,
		namespaceStore,
	)

	t := &VMToPodTransformer{
		ClusterConfig: config,
		TemplateSvc:   templateSvc,
		LauncherImage: launcherImage,
	}

	for _, opt := range opts {
		opt(t)
	}

	return t
}

func (t *VMToPodTransformer) Transform(vmFile string) (*k8sv1.Pod, error) {
	data, err := ioutil.ReadFile(vmFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read VM file: %v", err)
	}
	vm := &virtv1.VirtualMachine{}
	if err := yaml.Unmarshal(data, vm); err != nil {
		return nil, fmt.Errorf("failed to unmarshal VM: %v", err)
	}

	if vm.ObjectMeta.Namespace == "" {
		vm.ObjectMeta.Namespace = "default"
	}

	// Apply VM defaults
	defaults.SetVirtualMachineDefaults(vm, t.ClusterConfig, nil)

	vmi := vmCtrl.SetupVMIFromVM(vm)

	if err := defaults.SetDefaultVirtualMachineInstance(t.ClusterConfig, vmi); err != nil {
		return nil, fmt.Errorf("failed to set VMI defaults: %v", err)
	}
	if err := mutators.ApplyNewVMIMutations(vmi, t.ClusterConfig); err != nil {
		return nil, fmt.Errorf("failed to apply VMI mutations: %v", err)
	}

	if err := vmispec.SetDefaultNetworkInterface(t.ClusterConfig, &vmi.Spec); err != nil {
		return nil, fmt.Errorf("failed to set default network: %v", err)
	}

	util.SetDefaultVolumeDisk(&vmi.Spec)
	vmCtrl.AutoAttachInputDevice(vmi)

	pod, err := t.TemplateSvc.RenderLaunchManifest(vmi)
	if err != nil {
		return nil, fmt.Errorf("failed to render Pod: %v", err)
	}
	
	// add type
	pod.TypeMeta = metav1.TypeMeta{
        Kind:       "Pod",
        APIVersion: "v1",
    }

	if t.AddConsoleProxy {
		addConsoleProxySidecar(pod, t.ProxyImage, t.ProxyPort)
	}

	vmiJSON, err := json.Marshal(vmi)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal VMI: %v", err)
	}
	for i, c := range pod.Spec.Containers {
		if c.Name == "compute" {
			pod.Spec.Containers[i].Env = append(c.Env, k8sv1.EnvVar{Name: "STANDALONE_VMI", Value: string(vmiJSON)})
			break
		}
	}

	return pod, nil
}

func addConsoleProxySidecar(pod *k8sv1.Pod, proxyImage string, proxyPort int) {
	// Shared volume for kubevirt-private
	pod.Spec.Volumes = append(pod.Spec.Volumes, k8sv1.Volume{
		Name: "kubevirt-private",
		VolumeSource: k8sv1.VolumeSource{
			EmptyDir: &k8sv1.EmptyDirVolumeSource{},
		},
	})

	// Mount in virt-launcher
	for i, c := range pod.Spec.Containers {
		if c.Name == "virt-launcher" {
			pod.Spec.Containers[i].VolumeMounts = append(c.VolumeMounts, k8sv1.VolumeMount{
				Name:      "kubevirt-private",
				MountPath: "/var/run/kubevirt-private",
			})
		}
	}

	// Add proxy as a sidecar
	pod.Spec.Containers = append(pod.Spec.Containers, k8sv1.Container{
		Name:    "console-proxy",
		Image:   proxyImage,
		Command: []string{"/proxy", fmt.Sprintf("-port=%d", proxyPort)},
		Ports: []k8sv1.ContainerPort{
			{ContainerPort: int32(proxyPort), Protocol: "TCP"},
		},
		VolumeMounts: []k8sv1.VolumeMount{
			{Name: "kubevirt-private", MountPath: "/var/run/kubevirt-private"},
		},
		SecurityContext: &k8sv1.SecurityContext{
			Capabilities: &k8sv1.Capabilities{Drop: []k8sv1.Capability{"ALL"}},
		},
	})

	// Expose port on host
	pod.Spec.Containers[len(pod.Spec.Containers)-1].Ports = append(pod.Spec.Containers[len(pod.Spec.Containers)-1].Ports, k8sv1.ContainerPort{HostPort: int32(proxyPort)})
}

var codec = serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()
