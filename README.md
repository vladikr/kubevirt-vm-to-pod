# kubevirt-vm-to-pod

A standalone tool to transform KubeVirt VirtualMachine YAML into Pod YAML for offline/standalone execution (e.g., with Podman).

## Overview

Convert KubeVirt VirtualMachine definitions into standalone Pods that can run outside a Kubernetes cluster using container runtimes like Podman. The tool applies KubeVirt defaults, mutations, and generates a fully-functional Pod specification with the VMI embedded as an environment variable.

**Key Features:**
- ✅ Converts VM to standalone Pod YAML
- ✅ Supports Instancetype and Preference expansion
- ✅ Optional console proxy sidecar for VM access
- ✅ Force Passt network binding for standalone execution
- ✅ Mount KVM devices for hardware virtualization
- ✅ Compatible with Podman kube play
- ✅ Uses KubeVirt v1.7.0 APIs

## Installation

### From Source

```bash
git clone https://github.com/vladikr/kubevirt-vm-to-pod.git
cd kubevirt-vm-to-pod
make build
```

Binary will be created as `./kubevirt-vm-to-pod`

### Using Go Install

```bash
go install github.com/vladikr/kubevirt-vm-to-pod/cmd@latest
```

## Quick Start

### Basic Usage

```bash
# Convert VM to Pod
./kubevirt-vm-to-pod --vm-file=myvm.yaml > pod.yaml

# Run with Podman
podman kube play pod.yaml
```

### Recommended for Standalone Execution

```bash
# Generate Pod with device mounts for KVM access
./kubevirt-vm-to-pod --vm-file=myvm.yaml --mount-devices > pod.yaml

# Run with Podman
podman kube play pod.yaml

# Check VM status
podman ps
podman logs <pod-name>-compute
```

## Command-Line Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--vm-file` | Path to VirtualMachine YAML file | **(required)** |
| `--mount-devices` | Mount KVM devices (/dev/kvm, /dev/vhost-net, /dev/net/tun) for standalone execution | `false` |
| `--force-passt` | Force all network interfaces to use Passt binding | `false` |
| `--add-console-proxy` | Add console proxy sidecar to the Pod | `false` |
| `--launcher-image` | Virt-launcher container image | `quay.io/kubevirt/virt-launcher:v1.7.0` |
| `--instancetype-file` | Path to VirtualMachineInstancetype YAML file (optional) | - |
| `--preference-file` | Path to VirtualMachinePreference YAML file (optional) | - |
| `--proxy-image` | Console proxy container image | `quay.io/vladikr/kubevirt-console-proxy:latest` |
| `--proxy-port` | Port for console proxy to listen on | `8080` |
| `--output` | Output format: yaml or json | `yaml` |

## Usage Examples

### 1. Basic VM with KVM Devices

```bash
./kubevirt-vm-to-pod \
  --vm-file=myvm.yaml \
  --mount-devices \
  > pod.yaml

podman kube play pod.yaml
```

### 2. VM with Instancetype and Preference

```bash
./kubevirt-vm-to-pod \
  --vm-file=myvm.yaml \
  --instancetype-file=instancetype.yaml \
  --preference-file=preference.yaml \
  --mount-devices \
  > pod.yaml
```

### 3. VM with Console Proxy and Passt Networking

```bash
./kubevirt-vm-to-pod \
  --vm-file=myvm.yaml \
  --add-console-proxy \
  --force-passt \
  --mount-devices \
  > pod.yaml

# Access console proxy on port 8080
podman kube play pod.yaml
curl http://localhost:8080/console
```

### 4. VM with All Features

```bash
./kubevirt-vm-to-pod \
  --vm-file=myvm.yaml \
  --instancetype-file=instancetype.yaml \
  --preference-file=preference.yaml \
  --add-console-proxy \
  --force-passt \
  --mount-devices \
  --launcher-image=quay.io/kubevirt/virt-launcher:v1.7.0 \
  > pod.yaml
```

## Feature Details

### Device Mounting (`--mount-devices`)

Mounts KVM devices as hostPath volumes for hardware virtualization support. **Recommended for standalone execution.**

Without this flag, the Pod requests device plugin resources (`devices.kubevirt.io/kvm`), which work in Kubernetes but not in Podman.

**Devices mounted:**
- `/dev/kvm` - KVM virtualization
- `/dev/vhost-net` - vhost networking acceleration
- `/dev/net/tun` - TUN/TAP devices for networking
- **GPU devices** (auto-detected from VM spec):
  - NVIDIA: `/dev/nvidia*`, `/dev/nvidiactl`, `/dev/nvidia-uvm`, etc.
  - AMD/Intel: `/dev/dri/card*`, `/dev/dri/renderD*`
- **PCI hostdevices**: `/dev/vfio/*` for device passthrough

**When to use:**
- ✅ Always use when running with Podman
- ✅ When you want actual VM boot with KVM
- ✅ When VM requests GPU passthrough
- ❌ Not needed in Kubernetes (device plugins handle this)

**GPU Support:**
The `--mount-devices` flag automatically detects and mounts GPU devices from the VM spec:
- Detects NVIDIA, AMD, and Intel GPUs from `deviceName` field
- Mounts appropriate device files (`/dev/nvidia*`, `/dev/dri/*`)
- Supports multiple GPUs per VM
- See [GPU-SUPPORT.md](GPU-SUPPORT.md) for detailed documentation

### Force Passt (`--force-passt`)

Converts all network interface bindings to Passt and all networks to Pod networks.

**Use cases:**
- Convert VMs with Multus/bridge networking to pod networking
- Ensure compatibility with standalone execution
- Simplify networking for local testing

**Example transformation:**
```yaml
# Before
interfaces:
  - name: default
    masquerade: {}
  - name: net1
    bridge: {}
networks:
  - name: default
    pod: {}
  - name: net1
    multus:
      networkName: mynet

# After (with --force-passt)
interfaces:
  - name: default
    passt: {}
  - name: net1
    passt: {}
networks:
  - name: default
    pod: {}
  - name: net1
    pod: {}
```

### Console Proxy (`--add-console-proxy`)

Adds a console proxy sidecar container for accessing the VM console.

**Features:**
- WebSocket-based console access
- Shared volume with virt-launcher for socket communication
- Configurable port (default: 8080)

**Access the console:**
```bash
# After starting the Pod
curl http://localhost:8080/console
# Or use VNC/serial console clients
```

## Sample VirtualMachine YAML

```yaml
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: cirros-vm
spec:
  running: true
  template:
    spec:
      domain:
        resources:
          requests:
            memory: 128Mi
            cpu: 1
        devices:
          interfaces:
          - name: default
            masquerade: {}
      networks:
      - name: default
        pod: {}
      volumes:
      - name: containerdisk
        containerDisk:
          image: quay.io/kubevirt/cirros-container-disk-demo:latest
```

**Convert and run:**
```bash
./kubevirt-vm-to-pod --vm-file=cirros-vm.yaml --mount-devices > pod.yaml
podman kube play pod.yaml
```

## Testing

### Run Unit Tests

```bash
make test
```

### Run Functional Tests

```bash
# Quick validation tests (fast, no containers)
make functional-test-quick

# Full end-to-end test with Podman
make functional-test

# Console proxy test
make functional-test-proxy

# All functional tests
make functional-test-all
```

**Test requirements:**
- Podman installed
- /dev/kvm accessible (for full VM boot)

See [tests/functional/README.md](tests/functional/README.md) for detailed testing documentation.

## Build and Development

### Build Binary

```bash
make build
```

### Build Container Image

```bash
make podman-build
```

### Push to Registry

```bash
make podman-push
```

### Development Mode

Build with local KubeVirt source:

```bash
DEV_MODE=true make podman-build-dev
```

## How It Works

1. **Parse VM YAML** - Reads VirtualMachine specification
2. **Apply Defaults** - Uses KubeVirt defaults and mutations
3. **Expand Instancetype/Preference** - If provided, merges into VM spec
4. **Generate VMI** - Creates VirtualMachineInstance from VM template
5. **Render Pod** - Uses KubeVirt's TemplateService to create Pod spec
6. **Apply Options** - Adds console proxy, device mounts, networking changes
7. **Embed VMI** - Injects VMI JSON into STANDALONE_VMI env var
8. **Output** - Generates final Pod YAML

The generated Pod contains:
- **compute container**: virt-launcher running the VM
- **STANDALONE_VMI env var**: Embedded VMI specification
- **Device mounts** (if `--mount-devices`): /dev/kvm, /dev/vhost-net, /dev/net/tun
- **Console proxy sidecar** (if `--add-console-proxy`): WebSocket console access
- **Volume containers**: For container disks and ephemeral storage

## Architecture

```
VirtualMachine YAML
        ↓
    [Parser]
        ↓
   [Defaults & Mutations]
        ↓
  [Instancetype/Preference Expansion]
        ↓
    [VMI Generation]
        ↓
   [Pod Rendering]
        ↓
 [Optional: Console Proxy]
 [Optional: Force Passt]
 [Optional: Device Mounts]
        ↓
    Pod YAML
        ↓
  podman kube play
        ↓
   Running VM Pod
```

## Compatibility

- **KubeVirt Version**: v1.7.0
- **Kubernetes APIs**: v0.33.5
- **Container Runtime**: Podman 4.x+, Docker (experimental)
- **Host OS**: Linux with KVM support (for full VM boot)

## Limitations

- **No live migration**: Pods are standalone and don't support migration
- **No KubeVirt controllers**: No automatic reconciliation or state management
- **Limited networking**: Best with pod networking or Passt binding
- **Device access**: Requires `/dev/kvm` access on host for hardware virtualization
- **Podman specific**: Some features optimized for Podman kube play

## Troubleshooting

### VM doesn't boot

**Check device access:**
```bash
podman exec <container> ls -la /dev/kvm
# Should show: crw-rw-rw- 1 root root 10, 232 /dev/kvm
```

**Solution:** Use `--mount-devices` flag

### Permission denied on /dev/kvm

**Check host permissions:**
```bash
ls -la /dev/kvm
# Ensure your user has access
sudo chmod 666 /dev/kvm  # or add user to kvm group
```

### Podman kube play fails with "pod does not have a name"

**Issue:** Old version of tool generating `generateName` instead of `name`

**Solution:** Update to latest version (commit 7eea693+)

### Network errors with Multus

**Issue:** Multus networks require CNI configuration not available in Podman

**Solution:** Use `--force-passt` to convert to pod networking

## Contributing

Contributions welcome! Please:
1. Fork the repository
2. Create a feature branch
3. Add tests for new features
4. Ensure all tests pass (`make test`)
5. Submit a pull request

## License

[Apache 2.0](LICENSE)

## Related Projects

- [KubeVirt](https://kubevirt.io/) - Kubernetes Virtualization API
- [Podman](https://podman.io/) - Daemonless container engine
- [virt-launcher](https://github.com/kubevirt/kubevirt) - KubeVirt's VM launcher

## Credits

This tool uses KubeVirt's internal APIs to generate Pod specifications compatible with standalone execution.
