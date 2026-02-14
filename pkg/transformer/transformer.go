package transformer

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

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
	ForcePasst      	bool
	MountDevices    	bool
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

func WithForcePasst(enabled bool) TransformerOption {
	return func(t *VMToPodTransformer) {
		t.ForcePasst = enabled
	}
}

func WithMountDevices(enabled bool) TransformerOption {
	return func(t *VMToPodTransformer) {
		t.MountDevices = enabled
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

	launcherImage := "quay.io/kubevirt/virt-launcher:v1.7.0"

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

	if t.ForcePasst {
		forcePasstBinding(&vmi.Spec)
	}

	pod, err := t.TemplateSvc.RenderLaunchManifest(vmi)
	if err != nil {
		return nil, fmt.Errorf("failed to render Pod: %v", err)
	}

	// add type
	pod.TypeMeta = metav1.TypeMeta{
        Kind:       "Pod",
        APIVersion: "v1",
    }

	// Convert generateName to name for standalone pods (required by podman kube play)
	if pod.ObjectMeta.GenerateName != "" && pod.ObjectMeta.Name == "" {
		pod.ObjectMeta.Name = pod.ObjectMeta.GenerateName[:len(pod.ObjectMeta.GenerateName)-1]
		pod.ObjectMeta.GenerateName = ""
	}

	if t.AddConsoleProxy {
		addConsoleProxySidecar(pod, t.ProxyImage, t.ProxyPort)
	}

	if t.MountDevices {
		mountHostDevices(pod, vmi)
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

func mountHostDevices(pod *k8sv1.Pod, vmi *virtv1.VirtualMachineInstance) {
	hostPathCharDev := k8sv1.HostPathCharDev

	// Always mount KVM devices
	kvmDevices := []struct {
		name string
		path string
	}{
		{"kvm", "/dev/kvm"},
		{"tun", "/dev/net/tun"},
		{"vhost-net", "/dev/vhost-net"},
	}

	for _, dev := range kvmDevices {
		mountDevice(pod, dev.name, dev.path, &hostPathCharDev)
	}

	// Mount GPU devices if requested in VMI
	if vmi != nil && vmi.Spec.Domain.Devices.GPUs != nil {
		for i, gpu := range vmi.Spec.Domain.Devices.GPUs {
			// Detect vendor from deviceName
			vendor := detectGPUVendor(gpu.DeviceName)

			switch vendor {
			case "nvidia":
				// Mount NVIDIA GPU devices
				// Primary GPU device (nvidia0, nvidia1, etc.)
				mountDevice(pod, fmt.Sprintf("nvidia%d", i), fmt.Sprintf("/dev/nvidia%d", i), &hostPathCharDev)

				// Control devices (only mount once, not per GPU)
				if i == 0 {
					mountDevice(pod, "nvidiactl", "/dev/nvidiactl", &hostPathCharDev)
					mountDevice(pod, "nvidia-uvm", "/dev/nvidia-uvm", &hostPathCharDev)
					mountDevice(pod, "nvidia-uvm-tools", "/dev/nvidia-uvm-tools", &hostPathCharDev)
					mountDevice(pod, "nvidia-modeset", "/dev/nvidia-modeset", &hostPathCharDev)
				}

			case "amd", "intel":
				// Mount DRI devices for AMD/Intel GPUs
				// These use /dev/dri/card* and /dev/dri/renderD*
				mountDevice(pod, fmt.Sprintf("dri-card%d", i), fmt.Sprintf("/dev/dri/card%d", i), &hostPathCharDev)
				mountDevice(pod, fmt.Sprintf("dri-render%d", i), fmt.Sprintf("/dev/dri/renderD%d", 128+i), &hostPathCharDev)

			default:
				// Generic GPU - try to mount common devices
				fmt.Fprintf(os.Stderr, "Warning: Unknown GPU vendor for device %s, mounting generic DRI devices\n", gpu.DeviceName)
				mountDevice(pod, fmt.Sprintf("dri-card%d", i), fmt.Sprintf("/dev/dri/card%d", i), &hostPathCharDev)
			}
		}
	}

	// Mount PCI hostdevices if requested in VMI
	if vmi != nil && vmi.Spec.Domain.Devices.HostDevices != nil {
		for i, hostdev := range vmi.Spec.Domain.Devices.HostDevices {
			// For PCI hostdevices, we need to mount the vfio device
			// Format: /dev/vfio/X where X is the IOMMU group number
			// This is complex and requires parsing PCI addresses
			fmt.Fprintf(os.Stderr, "Warning: PCI hostdevice %s detected. Mounting /dev/vfio/* requires manual configuration\n", hostdev.Name)

			// Mount vfio devices (common for SR-IOV and GPU passthrough)
			if i == 0 {
				// Mount /dev/vfio/vfio (VFIO container)
				mountDevice(pod, "vfio", "/dev/vfio/vfio", &hostPathCharDev)
			}
		}
	}
}

func mountDevice(pod *k8sv1.Pod, volumeName, devicePath string, pathType *k8sv1.HostPathType) {
	// Add volume
	pod.Spec.Volumes = append(pod.Spec.Volumes, k8sv1.Volume{
		Name: volumeName,
		VolumeSource: k8sv1.VolumeSource{
			HostPath: &k8sv1.HostPathVolumeSource{
				Path: devicePath,
				Type: pathType,
			},
		},
	})

	// Mount in compute container
	for i, c := range pod.Spec.Containers {
		if c.Name == "compute" {
			pod.Spec.Containers[i].VolumeMounts = append(c.VolumeMounts, k8sv1.VolumeMount{
				Name:      volumeName,
				MountPath: devicePath,
			})
			break
		}
	}
}

func detectGPUVendor(deviceName string) string {
	deviceLower := strings.ToLower(deviceName)

	if strings.Contains(deviceLower, "nvidia") {
		return "nvidia"
	}
	if strings.Contains(deviceLower, "amd") || strings.Contains(deviceLower, "radeon") {
		return "amd"
	}
	if strings.Contains(deviceLower, "intel") {
		return "intel"
	}
	return "unknown"
}

func forcePasstBinding(spec *virtv1.VirtualMachineInstanceSpec) {
	// Ensure at least one pod network
	hasPodNetwork := false
	for _, net := range spec.Networks {
		if net.Pod != nil {
			hasPodNetwork = true
			break
		}
	}
	if !hasPodNetwork {
		// Add default pod network if none exists
		spec.Networks = append([]virtv1.Network{virtv1.Network{
			Name: "default",
			NetworkSource: virtv1.NetworkSource{
				Pod: &virtv1.PodNetwork{},
			},
		}}, spec.Networks...)
	}

	// Force all interfaces to Passt
	for i := range spec.Domain.Devices.Interfaces {
		iface := &spec.Domain.Devices.Interfaces[i]
		iface.InterfaceBindingMethod = virtv1.InterfaceBindingMethod{
			DeprecatedPasst: &virtv1.DeprecatedInterfacePasst{},
		}
		// Clear other bindings
		iface.Masquerade = nil
		iface.Bridge = nil
		iface.DeprecatedSlirp = nil
		iface.SRIOV = nil
	}

	// Match interfaces to pod networks
	for i := range spec.Domain.Devices.Interfaces {
		iface := &spec.Domain.Devices.Interfaces[i]
		found := false
		for j := range spec.Networks {
			net := &spec.Networks[j]
			if net.Name == iface.Name {
				net.Pod = &virtv1.PodNetwork{}
				net.Multus = nil
				found = true
				break
			}
		}
		if !found {
			// Link to default pod network
			iface.Name = "default"
			// Check if default network already exists
			defaultExists := false
			for _, net := range spec.Networks {
				if net.Name == "default" {
					defaultExists = true
					break
				}
			}
			if !defaultExists {
				spec.Networks = append(spec.Networks, virtv1.Network{
					Name: "default",
					NetworkSource: virtv1.NetworkSource{Pod: &virtv1.PodNetwork{}},
				})
			}
		}
	}
}

var codec = serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()
