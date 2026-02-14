# Functional Tests

End-to-end functional tests for kubevirt-vm-to-pod tool using Podman.

## Prerequisites

- **Podman** installed and accessible in PATH
- **Root or sudo access** (Podman may require it for certain operations)
- **Built kubevirt-vm-to-pod binary** (run `make build` from repo root)

## Test Files

### 1. Basic Functional Test (`run-functional-test.sh`)

Tests the core functionality of converting a VM to a Pod and running it with Podman.

**What it tests:**
- ✅ VM to Pod YAML conversion
- ✅ Pod YAML validation (virt-launcher container, STANDALONE_VMI env)
- ✅ Podman kube play execution
- ✅ Container startup and health
- ✅ virt-launcher process running
- ✅ Pod networking

**Run:**
```bash
./tests/functional/run-functional-test.sh
# OR
make functional-test
```

### 2. Console Proxy Test (`run-console-proxy-test.sh`)

Tests the console proxy sidecar and force-passt features.

**What it tests:**
- ✅ Console proxy sidecar container
- ✅ Force-passt network binding conversion
- ✅ Shared volume mounting (kubevirt-private)
- ✅ Multi-container Pod coordination
- ✅ Proxy port listening

**Run:**
```bash
./tests/functional/run-console-proxy-test.sh
# OR
make functional-test-proxy
```

### 3. Run All Tests

```bash
make functional-test-all
```

## Test VM

The tests use a simple Cirros VM defined in `test-vm.yaml`:
- **Memory:** 128Mi
- **CPU:** 1 core
- **Disk:** Cirros container disk (quay.io/kubevirt/cirros-container-disk-demo:latest)
- **Network:** Default Pod network with masquerade binding

## Test Flow

```
1. Check prerequisites (podman, binary)
   ↓
2. Generate Pod YAML from VM
   ↓
3. Validate Pod YAML content
   ↓
4. Run: podman kube play pod.yaml
   ↓
5. Wait for containers to start
   ↓
6. Verify container health
   ↓
7. Check virt-launcher process
   ↓
8. Verify networking
   ↓
9. Show logs and summary
   ↓
10. Cleanup (automatic on exit)
```

## Expected Output

### Successful Test Run

```
→ Checking prerequisites...
✓ podman found: podman version 4.x.x
✓ Binary built successfully
→ Generating Pod YAML from VirtualMachine...
✓ Pod YAML generated successfully
✓ Pod YAML validation passed
→ Starting Pod with podman kube play...
✓ Pod started successfully
→ Waiting for virt-launcher container to be running...
✓ Container is running (took 15s)
✓ Found compute container: virt-launcher-cirros-test-compute
✓ virt-launcher-monitor process is running
✓ virt-launcher process found

========================================
✓ FUNCTIONAL TEST PASSED
========================================
```

## Cleanup

Tests automatically cleanup on exit (both success and failure):
- Removes created Pods
- Deletes generated Pod YAML files

To manually cleanup after a test:
```bash
podman pod rm -f virt-launcher-cirros-test
```

## Troubleshooting

### Podman permission errors
```bash
# Run with sudo if needed
sudo ./tests/functional/run-functional-test.sh
```

### Container fails to start
Check logs:
```bash
podman pod ps
podman logs virt-launcher-cirros-test-compute
```

### KVM not available
The tests work without /dev/kvm access. The QEMU process check will report this as expected:
```
→ QEMU process not found (expected without /dev/kvm access)
```

### Port conflicts (console proxy test)
If port 8080 is in use, the console proxy container may fail to start. Check with:
```bash
netstat -tln | grep 8080
```

## Limitations

- Tests require Podman (not Docker/containerd)
- VM won't fully boot without KVM device access
- Tests verify container startup and virt-launcher process, not full VM boot
- Console access requires working /dev/kvm and proper VM networking

## Future Enhancements

- [ ] Add actual console connection test (vnc/serial)
- [ ] Test with different VM configurations (instancetype/preference)
- [ ] Test volume mounting scenarios
- [ ] Add performance benchmarks
- [ ] Test migration scenarios
- [ ] Add CI/CD integration
