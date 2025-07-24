```markdown
# kubevirt-vm-to-pod-tool

A standalone tool to transform KubeVirt VirtualMachine YAML into Pod YAML for offline use (e.g., with Podman).

## Installation

```bash
go install github.com/vladikr/kubevirt-vm-to-pod-tool@latest
```

## Build Locally

```bash
make build  # Produces ./kubevirt-vm-to-pod binary
make test   # Runs unit tests
```

## Containerized Version

Build and push to Quay.io:

```bash
make podman-build
make podman-push  
```

Run the container:

```bash
podman run --rm quay.io/vladikr/kubevirt-vm-to-pod-tool:latest --vm-file=/path/to/myvm.yaml > pod.yaml
# Mount local file: podman run --rm -v $(pwd):/workspace quay.io/vladikr/kubevirt-vm-to-pod-tool:latest --vm-file=/workspace/myvm.yaml
# With instancetype/preference: podman run --rm -v $(pwd):/workspace quay.io/vladikr/kubevirt-vm-to-pod-tool:latest --vm-file=/workspace/myvm.yaml --instancetype-file=/workspace/inst.yaml --preference-file=/workspace/pref.yaml
```

Then run the Pod: `podman kube play pod.yaml`


# more to come...
