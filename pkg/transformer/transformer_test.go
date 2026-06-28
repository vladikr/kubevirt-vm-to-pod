package transformer

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	k8sv1 "k8s.io/api/core/v1"
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
		require.Len(t, pod.Spec.Containers, 1) // compute only (ImageVolume mode)
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

		require.Len(t, pod.Spec.Containers, 2) // compute + console-proxy
		var proxyContainer k8sv1.Container
		for _, c := range pod.Spec.Containers {
			if c.Name == "console-proxy" {
				proxyContainer = c
				break
			}
		}
		require.Equal(t, "console-proxy", proxyContainer.Name)
		require.Equal(t, "test-proxy-image", proxyContainer.Image)
		require.Contains(t, proxyContainer.Command[0], "/console-proxy")
		require.Contains(t, proxyContainer.Command[1], "-port=8080")
		require.Contains(t, proxyContainer.Command[2], "-listen=unix")

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
		require.Len(t, pod.Spec.Containers, 1) // compute only
	})
}

func TestVolumeSupport(t *testing.T) {
	findVolume := func(pod *k8sv1.Pod, name string) *k8sv1.Volume {
		for i := range pod.Spec.Volumes {
			if pod.Spec.Volumes[i].Name == name {
				return &pod.Spec.Volumes[i]
			}
		}
		return nil
	}

	findMount := func(pod *k8sv1.Pod, containerName, volumeName string) *k8sv1.VolumeMount {
		for i := range pod.Spec.Containers {
			if pod.Spec.Containers[i].Name != containerName {
				continue
			}
			for j := range pod.Spec.Containers[i].VolumeMounts {
				if pod.Spec.Containers[i].VolumeMounts[j].Name == volumeName {
					return &pod.Spec.Containers[i].VolumeMounts[j]
				}
			}
		}
		return nil
	}

	t.Run("PVC volume produces persistentVolumeClaim and mount in compute", func(t *testing.T) {
		vmYAML := []byte(`
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: testvm-pvc
spec:
  template:
    spec:
      domain:
        devices:
          disks:
          - disk:
              bus: virtio
            name: datadisk
      volumes:
      - persistentVolumeClaim:
          claimName: test-vm-data
        name: datadisk
`)
		tmpFile, err := os.CreateTemp("", "vm-pvc.yaml")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		_, err = tmpFile.Write(vmYAML)
		require.NoError(t, err)

		pod, err := NewVMToPodTransformer().Transform(tmpFile.Name())
		require.NoError(t, err)

		vol := findVolume(pod, "datadisk")
		require.NotNil(t, vol, "datadisk volume should be present in pod spec")
		require.NotNil(t, vol.PersistentVolumeClaim, "volume source should be persistentVolumeClaim")
		require.Equal(t, "test-vm-data", vol.PersistentVolumeClaim.ClaimName)

		mount := findMount(pod, "compute", "datadisk")
		require.NotNil(t, mount, "datadisk should be mounted in compute container")
		require.Contains(t, mount.MountPath, "vmi-disks/datadisk")
	})

	t.Run("hostDisk volume produces hostPath and mount in compute", func(t *testing.T) {
		vmYAML := []byte(`
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: testvm-hostdisk
spec:
  template:
    spec:
      domain:
        devices:
          disks:
          - disk:
              bus: virtio
            name: hostdisk
      volumes:
      - hostDisk:
          path: /var/lib/vms/disk.img
          type: DiskOrCreate
          capacity: 1Gi
        name: hostdisk
`)
		tmpFile, err := os.CreateTemp("", "vm-hostdisk.yaml")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		_, err = tmpFile.Write(vmYAML)
		require.NoError(t, err)

		pod, err := NewVMToPodTransformer().Transform(tmpFile.Name())
		require.NoError(t, err)

		vol := findVolume(pod, "hostdisk")
		require.NotNil(t, vol, "hostdisk volume should be present in pod spec")
		require.NotNil(t, vol.HostPath, "volume source should be hostPath")
		require.Equal(t, "/var/lib/vms", vol.HostPath.Path)

		mount := findMount(pod, "compute", "hostdisk")
		require.NotNil(t, mount, "hostdisk should be mounted in compute container")
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
			require.NotNil(t, iface.PasstBinding, "Interface %s should have Passt binding", iface.Name)
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
			require.NotNil(t, iface.PasstBinding, "Interface %s should have Passt binding", iface.Name)
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
		require.Nil(t, vmi.Spec.Domain.Devices.Interfaces[0].PasstBinding, "Should not have Passt binding")
	})
}

func TestVMHealthCheck(t *testing.T) {
	t.Run("adds liveness probe to compute container", func(t *testing.T) {
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

		pod, err := NewVMToPodTransformer().Transform(tmpFile.Name())
		require.NoError(t, err)

		var compute k8sv1.Container
		for _, c := range pod.Spec.Containers {
			if c.Name == "compute" {
				compute = c
				break
			}
		}
		require.NotNil(t, compute.LivenessProbe, "compute container should have a liveness probe")
		require.NotNil(t, compute.LivenessProbe.Exec, "liveness probe should use exec")
		require.Contains(t, compute.LivenessProbe.Exec.Command[2], "virsh domstate default_testvm")
		require.Contains(t, compute.LivenessProbe.Exec.Command[2], "grep -q running")
		require.Equal(t, int32(60), compute.LivenessProbe.InitialDelaySeconds)
		require.Equal(t, int32(10), compute.LivenessProbe.PeriodSeconds)
	})

	t.Run("uses correct namespace in domain name", func(t *testing.T) {
		vmYAML := []byte(`
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: testvm
  namespace: mynamespace
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

		pod, err := NewVMToPodTransformer().Transform(tmpFile.Name())
		require.NoError(t, err)

		var compute k8sv1.Container
		for _, c := range pod.Spec.Containers {
			if c.Name == "compute" {
				compute = c
				break
			}
		}
		require.Contains(t, compute.LivenessProbe.Exec.Command[2], "virsh domstate mynamespace_testvm")
	})
}

func TestDataVolumeError(t *testing.T) {
	t.Run("DataVolume volume produces clear error with alternatives", func(t *testing.T) {
		vmYAML := []byte(`
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: testvm-datavolume
spec:
  template:
    spec:
      domain:
        devices:
          disks:
          - disk:
              bus: virtio
            name: datadisk
      volumes:
      - dataVolume:
          name: test-dv
        name: datadisk
`)
		tmpFile, err := os.CreateTemp("", "vm-datavolume.yaml")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		_, err = tmpFile.Write(vmYAML)
		require.NoError(t, err)

		_, err = NewVMToPodTransformer().Transform(tmpFile.Name())
		require.Error(t, err, "Should fail with DataVolume error")
		require.Contains(t, err.Error(), "DataVolume")
		require.Contains(t, err.Error(), "hostDisk")
		require.Contains(t, err.Error(), "persistentVolumeClaim")
		require.Contains(t, err.Error(), "Recommended alternatives")
	})
}

func TestPersistenceWarnings(t *testing.T) {
	t.Run("PVC volume adds persistence warning annotation", func(t *testing.T) {
		vmYAML := []byte(`
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: testvm-pvc
spec:
  template:
    spec:
      domain:
        devices:
          disks:
          - disk:
              bus: virtio
            name: datadisk
      volumes:
      - persistentVolumeClaim:
          claimName: test-vm-data
        name: datadisk
`)
		tmpFile, err := os.CreateTemp("", "vm-pvc-warning.yaml")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		_, err = tmpFile.Write(vmYAML)
		require.NoError(t, err)

		pod, err := NewVMToPodTransformer().Transform(tmpFile.Name())
		require.NoError(t, err)

		warning, ok := pod.Annotations["kubevirt-vm-to-pod/persistence-warning"]
		require.True(t, ok, "Should have persistence warning annotation")
		require.Contains(t, warning, "PVC volumes")
		require.Contains(t, warning, "Podman named volumes")
		require.Contains(t, warning, "THIS host only")
	})

	t.Run("hostDisk volume adds persistence warning annotation", func(t *testing.T) {
		vmYAML := []byte(`
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: testvm-hostdisk
spec:
  template:
    spec:
      domain:
        devices:
          disks:
          - disk:
              bus: virtio
            name: hostdisk
      volumes:
      - hostDisk:
          path: /var/lib/vms/disk.img
          type: DiskOrCreate
          capacity: 1Gi
        name: hostdisk
`)
		tmpFile, err := os.CreateTemp("", "vm-hostdisk-warning.yaml")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		_, err = tmpFile.Write(vmYAML)
		require.NoError(t, err)

		pod, err := NewVMToPodTransformer().Transform(tmpFile.Name())
		require.NoError(t, err)

		warning, ok := pod.Annotations["kubevirt-vm-to-pod/persistence-warning"]
		require.True(t, ok, "Should have persistence warning annotation")
		require.Contains(t, warning, "hostDisk volumes")
		require.Contains(t, warning, "exist on the host filesystem")
		require.Contains(t, warning, "DiskOrCreate")
	})

	t.Run("VM with both PVC and hostDisk adds combined warning", func(t *testing.T) {
		vmYAML := []byte(`
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: testvm-both
spec:
  template:
    spec:
      domain:
        devices:
          disks:
          - disk:
              bus: virtio
            name: pvc-disk
          - disk:
              bus: virtio
            name: host-disk
      volumes:
      - persistentVolumeClaim:
          claimName: test-pvc
        name: pvc-disk
      - hostDisk:
          path: /data/disk.img
          type: Disk
        name: host-disk
`)
		tmpFile, err := os.CreateTemp("", "vm-both.yaml")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		_, err = tmpFile.Write(vmYAML)
		require.NoError(t, err)

		pod, err := NewVMToPodTransformer().Transform(tmpFile.Name())
		require.NoError(t, err)

		warning, ok := pod.Annotations["kubevirt-vm-to-pod/persistence-warning"]
		require.True(t, ok, "Should have persistence warning annotation")
		require.Contains(t, warning, "PVC volumes")
		require.Contains(t, warning, "hostDisk volumes")
		require.Contains(t, warning, "|") // Separator between warnings
	})

	t.Run("VM without persistent volumes has no warning annotation", func(t *testing.T) {
		vmYAML := []byte(`
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: testvm-ephemeral
spec:
  template:
    spec:
      domain:
        devices: {}
      volumes: []
`)
		tmpFile, err := os.CreateTemp("", "vm-ephemeral.yaml")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		_, err = tmpFile.Write(vmYAML)
		require.NoError(t, err)

		pod, err := NewVMToPodTransformer().Transform(tmpFile.Name())
		require.NoError(t, err)

		_, ok := pod.Annotations["kubevirt-vm-to-pod/persistence-warning"]
		require.False(t, ok, "Should not have persistence warning annotation for ephemeral VM")
	})
}
