package transformer

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	v1 "kubevirt.io/api/core/v1"
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

		tmpFile, err := os.CreateTemp("", "vm.yaml")
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

		vmFile, err := os.CreateTemp("", "vm.yaml")
		require.NoError(t, err)
		defer os.Remove(vmFile.Name())
		_, err = vmFile.Write(vmYAML)
		require.NoError(t, err)

		instFile, err := os.CreateTemp("", "inst.yaml")
		require.NoError(t, err)
		defer os.Remove(instFile.Name())
		_, err = instFile.Write(instYAML)
		require.NoError(t, err)

		prefFile, err := os.CreateTemp("", "pref.yaml")
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
		vmiJSON := ""
		for _, env := range pod.Spec.Containers[0].Env {
			if env.Name == "STANDALONE_VMI" {
				vmiJSON = env.Value
				break
			}
		}
		require.NotEmpty(t, vmiJSON)

		var vmi v1.VirtualMachineInstance
		err = json.Unmarshal([]byte(vmiJSON), &vmi)
		require.NoError(t, err)

		// Verify VMI was created successfully with instancetype/preference
		require.Equal(t, "testvm", vmi.Name)
		require.NotNil(t, vmi.Spec.Domain)
	})

	t.Run("error on invalid files", func(t *testing.T) {
		transformer := NewVMToPodTransformer(
			WithInstancetypeFile("/nonexistent"),
		)
		_, err := transformer.Transform("/fake/vm.yaml")
		require.Error(t, err)
	})
}

func TestTransformWithProxy(t *testing.T) {
	t.Run("add console proxy sidecar", func(t *testing.T) {
		vmYAML := []byte(`
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: testvm
spec:
  template:
    spec:
      domain:
        devices: {}
      volumes: []
`)

		tmpFile, err := os.CreateTemp("", "vm.yaml")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		_, err = tmpFile.Write(vmYAML)
		require.NoError(t, err)

		transformer := NewVMToPodTransformer(WithAddConsoleProxy(true, "test-proxy-image", 8080))
		pod, err := transformer.Transform(tmpFile.Name())
		require.NoError(t, err)

		require.Len(t, pod.Spec.Containers, 2)
		proxyContainer := pod.Spec.Containers[1]
		require.Equal(t, "console-proxy", proxyContainer.Name)
		require.Equal(t, "test-proxy-image", proxyContainer.Image)
		require.Contains(t, proxyContainer.Command[0], "/proxy")
		require.Contains(t, proxyContainer.Command[1], "-port=8080")

		// Check STANDALONE_VMI in virt-launcher
		vmiJSON := ""
		for _, env := range pod.Spec.Containers[0].Env {
			if env.Name == "STANDALONE_VMI" {
				vmiJSON = env.Value
				break
			}
		}
		require.NotEmpty(t, vmiJSON)

		var vmi v1.VirtualMachineInstance
		err = json.Unmarshal([]byte(vmiJSON), &vmi)
		require.NoError(t, err)
		require.Equal(t, "testvm", vmi.Name)
	})

	t.Run("without proxy", func(t *testing.T) {
		vmYAML := []byte(`
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: testvm
spec:
  template:
    spec:
      domain:
        devices: {}
      volumes: []
`)

		tmpFile, err := os.CreateTemp("", "vm.yaml")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		_, err = tmpFile.Write(vmYAML)
		require.NoError(t, err)

		transformer := NewVMToPodTransformer()
		pod, err := transformer.Transform(tmpFile.Name())
		require.NoError(t, err)
		require.Len(t, pod.Spec.Containers, 1)
	})
}
