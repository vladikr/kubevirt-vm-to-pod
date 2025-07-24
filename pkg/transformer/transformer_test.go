package transformer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/testing"

	v1 "kubevirt.io/api/core/v1"
	instancetypev1beta1 "kubevirt.io/api/instancetype/v1beta1"
	instancetypeVMWebhooks "kubevirt.io/kubevirt/pkg/instancetype/webhooks/vm"
	"kubevirt.io/kubevirt/pkg/testutils"
	"kubevirt.io/kubevirt/pkg/util"
	"kubevirt.io/kubevirt/pkg/virt-api/webhooks/mutating-webhook/mutators"
	"kubevirt.io/kubevirt/pkg/network/vmispec"
	"kubevirt.io/kubevirt/pkg/virt-api/webhooks"
	"kubevirt.io/kubevirt/pkg/virt-controller/watch"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sv1 "k8s.io/api/core/v1"
)

func TestTransform(t *testing.T) {
	t.Run("basic VM transformation", func(t *testing.T) {
		vmYAML := []byte(`
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: testvm
spec:
  template:
    spec:
      domain:
        resources:
          requests:
            memory: 64Mi
        devices: {}
      volumes: []
`)

		tmpFile, err := ioutil.TempFile("", "vm.yaml")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		_, err = tmpFile.Write(vmYAML)
		require.NoError(t, err)

		transformer := NewVMToPodTransformer()
		pod, err := transformer.Transform(tmpFile.Name())
		require.NoError(t, err)

		require.NotNil(t, pod)
		require.Equal(t, "virt-launcher-testvm-", pod.GenerateName)
		require.Len(t, pod.Spec.Containers, 1) // Basic compute container
	})

	t.Run("with instancetype and preference", func(t *testing.T) {
		vmYAML := []byte(`
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: testvm
spec:
  instancetype:
    name: small
  preference:
    name: linux
  template:
    spec:
      domain: {}
`)

		instYAML := []byte(`
apiVersion: instancetype.kubevirt.io/v1beta1
kind: VirtualMachineInstancetype
metadata:
  name: small
spec:
  cpu:
    guest: 2
  memory:
    guest: 2Gi
`)

		prefYAML := []byte(`
apiVersion: instancetype.kubevirt.io/v1beta1
kind: VirtualMachinePreference
metadata:
  name: linux
spec:
  devices:
    preferredAutoattachInputDevice: true
`)

		vmFile, err := ioutil.TempFile("", "vm.yaml")
		require.NoError(t, err)
		defer os.Remove(vmFile.Name())
		_, err = vmFile.Write(vmYAML)
		require.NoError(t, err)

		instFile, err := ioutil.TempFile("", "inst.yaml")
		require.NoError(t, err)
		defer os.Remove(instFile.Name())
		_, err = instFile.Write(instYAML)
		require.NoError(t, err)

		prefFile, err := ioutil.TempFile("", "pref.yaml")
		require.NoError(t, err)
		defer os.Remove(prefFile.Name())
		_, err = prefFile.Write(prefYAML)
		require.NoError(t, err)

		transformer := NewVMToPodTransformer(
			WithInstancetypeFile(instFile.Name()),
			WithPreferenceFile(prefFile.Name()),
		)
		pod, err := transformer.Transform(vmFile.Name())
		require.NoError(t, err)

		require.NotNil(t, pod)
		// Check if preferences applied: e.g., input device auto-attached
		vmiJSON := ""
		for _, env := range pod.Spec.Containers[0].Env {
			if env.Name == "VMI_OBJ" {
				vmiJSON = env.Value
				break
			}
		}
		require.NotEmpty(t, vmiJSON)

		var vmi v1.VirtualMachineInstance
		err = json.Unmarshal([]byte(vmiJSON), &vmi)
		require.NoError(t, err)

		require.Len(t, vmi.Spec.Domain.Devices.Inputs, 1) // Auto-attached by preference
		require.Equal(t, uint32(2), vmi.Spec.Domain.CPU.Guest) // From instancetype
		require.Equal(t, "2Gi", vmi.Spec.Domain.Memory.Guest.String()) // From instancetype
	})

	t.Run("error on invalid files", func(t *testing.T) {
		transformer := NewVMToPodTransformer(
			WithInstancetypeFile("/nonexistent"),
		)
		_, err := transformer.Transform("/fake/vm.yaml")
		require.Error(t, err)
	})
}
