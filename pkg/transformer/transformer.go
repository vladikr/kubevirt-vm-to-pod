package transformer

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/testing"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/apimachinery/pkg/runtime"

	virtv1 "kubevirt.io/api/core/v1"
	instancetypev1beta1 "kubevirt.io/api/instancetype/v1beta1"
	instancetypeVMWebhooks "kubevirt.io/kubevirt/pkg/instancetype/webhooks/vm"
	"kubevirt.io/kubevirt/pkg/util"
	"kubevirt.io/kubevirt/pkg/virt-api/webhooks/mutating-webhook/mutators"
	"kubevirt.io/kubevirt/pkg/network/vmispec"
	"kubevirt.io/kubevirt/pkg/testutils"
	"kubevirt.io/kubevirt/pkg/virt-api/webhooks"
	"kubevirt.io/kubevirt/pkg/virt-controller/services"
	"kubevirt.io/kubevirt/pkg/virt-controller/watch"
	virtconfig "kubevirt.io/kubevirt/pkg/virt-config"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

type VMToPodTransformer struct {
	ClusterConfig *virtconfig.ClusterConfig
	TemplateSvc   services.TemplateService
	LauncherImage string
	InstancetypeFile string
	PreferenceFile   string
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

func NewVMToPodTransformer(opts ...TransformerOption) *VMToPodTransformer {
	crdInformer, _ := testutils.NewFakeInformerFor(&extv1.CustomResourceDefinition{})
	kvInformer, _ := testutils.NewFakeInformerFor(&virtv1.KubeVirt{})
	config, _ := virtconfig.NewClusterConfig(crdInformer, kvInformer, "default")

	pvcCache := testutils.NewFakeIndexerFor(&v1.PersistentVolumeClaim{})
	resourceQuotaStore := testutils.NewFakeStoreFor(&v1.ResourceQuota{})
	namespaceStore := testutils.NewFakeStoreFor(&v1.Namespace{})

	launcherImage := "quay.io/kubevirt/virt-launcher:latest"

	templateSvc := services.NewTemplateService(
		launcherImage,
		240,
		"/var/run/kubevirt",
		"/var/lib/kubevirt",
		"/var/run/kubevirt-ephemeral-disks",
		"/var/run/kubevirt/container-disks",
		virtv1.HotplugDiskDir,
		"",
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

func (t *VMToPodTransformer) Transform(vmFile string) (*v1.Pod, error) {
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
	if err := webhooks.SetDefaultVirtualMachine(t.ClusterConfig, vm); err != nil {
		return nil, fmt.Errorf("failed to set VM defaults: %v", err)
	}

	// Apply instancetypes/preferences if provided
	if t.InstancetypeFile != "" || t.PreferenceFile != "" {
		if err := t.applyInstancetypeAndPreferences(vm); err != nil {
			return nil, err
		}
	}

	vmi := watch.SetupVMIFromVM(vm)

	if err := webhooks.SetDefaultVirtualMachineInstance(t.ClusterConfig, vmi); err != nil {
		return nil, fmt.Errorf("failed to set VMI defaults: %v", err)
	}
	if err := mutators.ApplyNewVMIMutations(vmi, t.ClusterConfig); err != nil {
		return nil, fmt.Errorf("failed to apply VMI mutations: %v", err)
	}

	if err := vmispec.SetDefaultNetworkInterface(t.ClusterConfig, &vmi.Spec); err != nil {
		return nil, fmt.Errorf("failed to set default network: %v", err)
	}

	// Call the defaults you moved back
	util.SetDefaultVolumeDisk(&vmi.Spec)
	autoAttachInputDevice(vmi)

	pod, err := t.TemplateSvc.RenderLaunchManifest(vmi)
	if err != nil {
		return nil, fmt.Errorf("failed to render Pod: %v", err)
	}

	vmiJSON, err := json.Marshal(vmi)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal VMI: %v", err)
	}
	for i, c := range pod.Spec.Containers {
		if c.Name == "compute" {
			pod.Spec.Containers[i].Env = append(c.Env, v1.EnvVar{Name: "VMI_OBJ", Value: string(vmiJSON)})
			break
		}
	}

	return pod, nil
}

func (t *VMToPodTransformer) applyInstancetypeAndPreferences(vm *virtv1.VirtualMachine) error {
	fakeClient := fake.NewSimpleClientset()

	if t.InstancetypeFile != "" {
		instData, err := ioutil.ReadFile(t.InstancetypeFile)
		if err != nil {
			return err
		}
		inst := &instancetypev1beta1.VirtualMachineInstancetype{}
		if err := yaml.Unmarshal(instData, inst); err != nil {
			return err
		}
		fakeClient.Tracker.Add(inst)
	}

	if t.PreferenceFile != "" {
		prefData, err := ioutil.ReadFile(t.PreferenceFile)
		if err != nil {
			return err
		}
		pref := &instancetypev1beta1.VirtualMachinePreference{}
		if err := yaml.Unmarshal(prefData, pref); err != nil {
			return err
		}
		fakeClient.Tracker.Add(pref)
	}

	mutator := instancetypeVMWebhooks.NewMutator(fakeClient)

	vmJSON, err := json.Marshal(vm)
	if err != nil {
		return err
	}

	ar := &admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
			Resource: metav1.GroupVersionResource{Group: virtv1.GroupName, Version: "v1", Resource: "virtualmachines"},
			Object: runtime.RawExtension{Raw: vmJSON},
		},
	}

	resp := mutator.Mutate(ar)
	if resp.Allowed && len(resp.Patch) > 0 {
		patcher := patch.NewPatcher(resp.PatchType, resp.Patch)
		unstr := &unstructured.Unstructured{}
		unstr.SetUnstructuredContent(map[string]interface{}{})
		patched, err := patcher.Patch(unstr, codec)
		if err != nil {
			return err
		}
		patchedJSON, err := json.Marshal(patched)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(patchedJSON, vm); err != nil {
			return err
		}
	}

	return nil
}

var codec = serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()
