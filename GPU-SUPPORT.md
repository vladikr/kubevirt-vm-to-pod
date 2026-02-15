# GPU and Host Device Support

The `--mount-devices` flag automatically detects and mounts GPU and PCI hostdevices for standalone execution with Podman.

## Supported Device Types

### 1. KVM Devices (Always Mounted)
- `/dev/kvm` - KVM virtualization
- `/dev/vhost-net` - vhost networking acceleration
- `/dev/net/tun` - TUN/TAP devices

### 2. GPU Devices (Auto-detected from VM spec)

#### NVIDIA GPUs
When a VM requests NVIDIA GPUs (e.g., `nvidia.com/GPU`, `nvidia.com/TESLA_P40`), the following devices are mounted:

**Per-GPU devices:**
- `/dev/nvidia0`, `/dev/nvidia1`, etc.

**Shared control devices (mounted once):**
- `/dev/nvidiactl` - NVIDIA control device
- `/dev/nvidia-uvm` - Unified Virtual Memory
- `/dev/nvidia-uvm-tools` - UVM tools
- `/dev/nvidia-modeset` - Mode setting

#### AMD/Intel GPUs
When a VM requests AMD or Intel GPUs, the following DRI devices are mounted:

**Per-GPU devices:**
- `/dev/dri/card0`, `/dev/dri/card1`, etc. - Primary GPU device
- `/dev/dri/renderD128`, `/dev/dri/renderD129`, etc. - Render nodes

### 3. PCI Host Devices
When a VM requests PCI hostdevices, VFIO devices are mounted:
- `/dev/vfio/vfio` - VFIO container device

**Note:** Full PCI passthrough requires additional host configuration (IOMMU groups, device binding).

## Usage Examples

### NVIDIA GPU Passthrough

**VirtualMachine YAML:**
```yaml
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: gpu-vm
spec:
  template:
    spec:
      domain:
        resources:
          requests:
            memory: 4Gi
            cpu: 4
        devices:
          gpus:
          - deviceName: nvidia.com/GPU
            name: gpu1
      volumes:
      - name: containerdisk
        containerDisk:
          image: your-gpu-enabled-image:latest
```

**Generate and run:**
```bash
./kubevirt-vm-to-pod --vm-file=gpu-vm.yaml --mount-devices > pod.yaml
podman kube play pod.yaml
```

**Devices mounted:**
```
/dev/nvidia0
/dev/nvidiactl
/dev/nvidia-uvm
/dev/nvidia-uvm-tools
/dev/nvidia-modeset
```

### AMD GPU Passthrough

**VirtualMachine YAML:**
```yaml
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: amd-gpu-vm
spec:
  template:
    spec:
      domain:
        devices:
          gpus:
          - deviceName: amd.com/gpu
            name: gpu1
          - deviceName: amd.com/gpu
            name: gpu2
```

**Generate and run:**
```bash
./kubevirt-vm-to-pod --vm-file=amd-gpu-vm.yaml --mount-devices > pod.yaml
podman kube play pod.yaml
```

**Devices mounted:**
```
/dev/dri/card0
/dev/dri/renderD128
/dev/dri/card1
/dev/dri/renderD129
```

### Multiple GPUs

The tool automatically detects and mounts all requested GPUs:

```yaml
devices:
  gpus:
  - deviceName: nvidia.com/TESLA_V100
    name: gpu0
  - deviceName: nvidia.com/TESLA_V100
    name: gpu1
  - deviceName: nvidia.com/TESLA_V100
    name: gpu2
```

**Mounts:**
- `/dev/nvidia0`
- `/dev/nvidia1`
- `/dev/nvidia2`
- `/dev/nvidiactl` (shared)
- `/dev/nvidia-uvm` (shared)
- etc.

## Vendor Detection

The tool automatically detects the GPU vendor from the `deviceName`:

| Device Name Pattern | Detected Vendor | Devices Mounted |
|---------------------|-----------------|-----------------|
| Contains "nvidia" | NVIDIA | /dev/nvidia*, /dev/nvidiactl, etc. |
| Contains "amd" or "radeon" | AMD | /dev/dri/card*, /dev/dri/renderD* |
| Contains "intel" | Intel | /dev/dri/card*, /dev/dri/renderD* |
| Other | Unknown | /dev/dri/* (generic DRI) |

## Host Requirements

### NVIDIA GPUs
1. **NVIDIA drivers installed** on the host
2. **Device nodes present:**
   ```bash
   ls -la /dev/nvidia*
   ```
3. **Permissions:** User/container must have access to GPU devices
   ```bash
   sudo chmod 666 /dev/nvidia*
   # Or add user to video/render group
   sudo usermod -aG video,render $USER
   ```

### AMD/Intel GPUs
1. **GPU drivers installed** (amdgpu, i915, etc.)
2. **DRI devices present:**
   ```bash
   ls -la /dev/dri/
   ```
3. **Permissions:**
   ```bash
   sudo chmod 666 /dev/dri/*
   # Or add user to video/render group
   sudo usermod -aG video,render $USER
   ```

### PCI Hostdevices
1. **IOMMU enabled** in BIOS and kernel
   ```bash
   # Check IOMMU
   dmesg | grep -i iommu
   ```
2. **VFIO driver loaded:**
   ```bash
   modprobe vfio-pci
   ls -la /dev/vfio/
   ```
3. **Device bound to vfio-pci:**
   ```bash
   # Bind PCI device to vfio-pci
   echo "0000:01:00.0" > /sys/bus/pci/drivers/vfio-pci/bind
   ```

## Kubernetes vs Podman

### In Kubernetes (Device Plugins)

KubeVirt uses device plugins to inject devices:
```yaml
resources:
  limits:
    nvidia.com/GPU: "1"  # Device plugin handles injection
```

The kubevirt device plugin automatically:
- Discovers available GPUs
- Mounts device files
- Sets up permissions
- Configures VFIO

### In Podman (Manual Device Mounts)

Podman doesn't support device plugins, so we mount devices explicitly:
```yaml
volumes:
- name: nvidia0
  hostPath:
    path: /dev/nvidia0
    type: CharDevice

containers:
- volumeMounts:
  - name: nvidia0
    mountPath: /dev/nvidia0
```

The `--mount-devices` flag automates this conversion.

## Troubleshooting

### GPU devices not accessible in VM

**Check host devices:**
```bash
ls -la /dev/nvidia*  # For NVIDIA
ls -la /dev/dri/     # For AMD/Intel
```

**Check container devices:**
```bash
podman exec <container> ls -la /dev/nvidia*
podman exec <container> ls -la /dev/dri/
```

**Fix permissions:**
```bash
sudo chmod 666 /dev/nvidia*
sudo chmod 666 /dev/dri/*
```

### vGPU (Mediated Devices)

vGPU support requires additional configuration:
```bash
# List available mdev types
ls /sys/class/mdev_bus/*/mdev_supported_types/

# Create mediated device
echo "<uuid>" > /sys/bus/mdev/devices/<mdev-type>/create
```

**Note:** Full vGPU support is complex and may require additional implementation.

### Permission denied errors

**Add user to groups:**
```bash
sudo usermod -aG video,render,kvm $USER
# Re-login for changes to take effect
```

**Or run with elevated permissions:**
```bash
sudo podman kube play pod.yaml
```

### Verification

**Verify GPU in container:**
```bash
# For NVIDIA
podman exec <container> nvidia-smi

# For AMD/Intel
podman exec <container> ls -la /dev/dri/
podman exec <container> clinfo
```

## Limitations

1. **vGPU (mediated devices):** Basic support only; full vGPU requires additional configuration
2. **PCI passthrough:** Requires IOMMU setup and may need manual VFIO binding
3. **Dynamic discovery:** Device paths are fixed; doesn't auto-discover available GPUs on host
4. **SR-IOV:** Virtual functions (VFs) require additional VFIO configuration

## Future Enhancements

- [ ] Dynamic GPU discovery from host
- [ ] Full vGPU (mdev) support with automatic device creation
- [ ] SR-IOV VF automatic detection and binding
- [ ] GPU resource validation (verify devices exist on host)
- [ ] Multi-GPU topology configuration
- [ ] GPU memory isolation and limits

## References

- [KubeVirt GPU Support](https://kubevirt.io/user-guide/virtual_machines/host-devices/)
- [KubeVirt GPU Example](https://github.com/kubevirt/kubevirt/blob/main/examples/vmi-gpu.yaml)
- [NVIDIA Container Toolkit](https://github.com/NVIDIA/nvidia-container-toolkit)
- [VFIO Documentation](https://www.kernel.org/doc/html/latest/driver-api/vfio.html)
