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
		require.Equal(t, "virt-launcher-testvm", pod.Name)
		require.Empty(t, pod.GenerateName, "GenerateName should be empty for standalone pods")
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

func TestForcePasst(t *testing.T) {
	t.Run("force-passt replaces bindings", func(t *testing.T) {
		vmYAML := []byte(`
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: testvm-passt
spec:
  template:
    spec:
      domain:
        devices:
          interfaces:
          - name: default
            masquerade: {}
          - name: eth1
            bridge: {}
      networks:
      - name: default
        pod: {}
      - name: eth1
        multus:
          networkName: mynet
`)

		tmpFile, err := os.CreateTemp("", "vm-passt.yaml")
		require.NoError(t, err)
		_, err = tmpFile.Write(vmYAML)
		require.NoError(t, err)
		tmpFile.Close()
		defer os.Remove(tmpFile.Name())

		transformer := NewVMToPodTransformer(WithForcePasst(true))
		pod, err := transformer.Transform(tmpFile.Name())
		require.NoError(t, err)

		// Extract STANDALONE_VMI
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

		// All interfaces should be Passt
		require.Len(t, vmi.Spec.Domain.Devices.Interfaces, 2)
		for _, iface := range vmi.Spec.Domain.Devices.Interfaces {
			require.NotNil(t, iface.DeprecatedPasst, "Interface %s should have Passt binding", iface.Name)
			require.Nil(t, iface.Masquerade, "Interface %s should not have Masquerade", iface.Name)
			require.Nil(t, iface.Bridge, "Interface %s should not have Bridge", iface.Name)
			require.Nil(t, iface.DeprecatedSlirp, "Interface %s should not have Slirp", iface.Name)
		}

		// Networks should be pod only
		require.Len(t, vmi.Spec.Networks, 2)
		for _, net := range vmi.Spec.Networks {
			require.NotNil(t, net.Pod, "Network %s should be Pod network", net.Name)
			require.Nil(t, net.Multus, "Network %s should not have Multus", net.Name)
		}
	})

	t.Run("force-passt adds default network when missing", func(t *testing.T) {
		vmYAML := []byte(`
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: testvm-no-net
spec:
  template:
    spec:
      domain:
        devices:
          interfaces:
          - name: eth0
            bridge: {}
      networks: []
`)

		tmpFile, err := os.CreateTemp("", "vm-no-net.yaml")
		require.NoError(t, err)
		_, err = tmpFile.Write(vmYAML)
		require.NoError(t, err)
		tmpFile.Close()
		defer os.Remove(tmpFile.Name())

		transformer := NewVMToPodTransformer(WithForcePasst(true))
		pod, err := transformer.Transform(tmpFile.Name())
		require.NoError(t, err)

		// Extract STANDALONE_VMI
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

		// Should have default pod network added
		require.GreaterOrEqual(t, len(vmi.Spec.Networks), 1)
		foundDefault := false
		for _, net := range vmi.Spec.Networks {
			if net.Name == "default" && net.Pod != nil {
				foundDefault = true
				break
			}
		}
		require.True(t, foundDefault, "Should have default pod network")

		// All interfaces should be Passt
		for _, iface := range vmi.Spec.Domain.Devices.Interfaces {
			require.NotNil(t, iface.DeprecatedPasst, "Interface %s should have Passt binding", iface.Name)
		}
	})

	t.Run("without force-passt preserves original bindings", func(t *testing.T) {
		vmYAML := []byte(`
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: testvm-orig
spec:
  template:
    spec:
      domain:
        devices:
          interfaces:
          - name: default
            masquerade: {}
      networks:
      - name: default
        pod: {}
`)

		tmpFile, err := os.CreateTemp("", "vm-orig.yaml")
		require.NoError(t, err)
		_, err = tmpFile.Write(vmYAML)
		require.NoError(t, err)
		tmpFile.Close()
		defer os.Remove(tmpFile.Name())

		transformer := NewVMToPodTransformer(WithForcePasst(false))
		pod, err := transformer.Transform(tmpFile.Name())
		require.NoError(t, err)

		// Extract STANDALONE_VMI
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

		// Should preserve original binding (masquerade)
		require.Len(t, vmi.Spec.Domain.Devices.Interfaces, 1)
		require.NotNil(t, vmi.Spec.Domain.Devices.Interfaces[0].Masquerade, "Should preserve Masquerade binding")
		require.Nil(t, vmi.Spec.Domain.Devices.Interfaces[0].DeprecatedPasst, "Should not have Passt binding")
	})
}
