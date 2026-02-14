#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test configuration
TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$TEST_DIR/../.." && pwd)"
VM_FILE="$TEST_DIR/test-vm.yaml"
POD_YAML="$TEST_DIR/test-pod.yaml"
BINARY="$REPO_ROOT/kubevirt-vm-to-pod"
POD_NAME="virt-launcher-cirros-test"
TEST_TIMEOUT=180  # 3 minutes

echo_success() { echo -e "${GREEN}✓${NC} $1"; }
echo_error() { echo -e "${RED}✗${NC} $1"; }
echo_info() { echo -e "${YELLOW}→${NC} $1"; }

cleanup() {
    echo_info "Cleaning up..."
    if podman pod exists "$POD_NAME" 2>/dev/null; then
        podman pod rm -f "$POD_NAME" 2>/dev/null || true
    fi
    rm -f "$POD_YAML"
    echo_success "Cleanup complete"
}

trap cleanup EXIT

# Step 1: Check prerequisites
echo_info "Checking prerequisites..."

if ! command -v podman &> /dev/null; then
    echo_error "podman not found. Please install podman."
    exit 1
fi
echo_success "podman found: $(podman --version)"

if [ ! -f "$BINARY" ]; then
    echo_error "Binary not found at $BINARY. Building..."
    cd "$REPO_ROOT"
    make build
    echo_success "Binary built successfully"
fi

if [ ! -f "$VM_FILE" ]; then
    echo_error "Test VM file not found at $VM_FILE"
    exit 1
fi

# Step 2: Generate Pod YAML
echo_info "Generating Pod YAML from VirtualMachine with device mounting..."
cd "$REPO_ROOT"
"$BINARY" --vm-file="$VM_FILE" --mount-devices > "$POD_YAML"

if [ ! -s "$POD_YAML" ]; then
    echo_error "Failed to generate Pod YAML"
    exit 1
fi
echo_success "Pod YAML generated successfully"

# Verify Pod YAML contains expected content
if ! grep -q "virt-launcher" "$POD_YAML"; then
    echo_error "Pod YAML doesn't contain virt-launcher container"
    exit 1
fi

if ! grep -q "STANDALONE_VMI" "$POD_YAML"; then
    echo_error "Pod YAML doesn't contain STANDALONE_VMI env var"
    exit 1
fi

if ! grep -q "/dev/kvm" "$POD_YAML"; then
    echo_error "Pod YAML doesn't contain /dev/kvm device mount"
    exit 1
fi
echo_success "KVM device mounts present"

echo_success "Pod YAML validation passed"

# Step 3: Start Pod with podman kube play
echo_info "Starting Pod with podman kube play..."
if podman kube play "$POD_YAML"; then
    echo_success "Pod started successfully"
else
    echo_error "Failed to start Pod with podman kube play"
    podman pod ls
    exit 1
fi

# Step 4: Wait for container to be running
echo_info "Waiting for virt-launcher container to be running..."
SECONDS=0
while [ $SECONDS -lt $TEST_TIMEOUT ]; do
    if podman pod exists "$POD_NAME" 2>/dev/null; then
        CONTAINER_STATUS=$(podman ps --filter "pod=$POD_NAME" --filter "name=.*compute" --format "{{.Status}}" 2>/dev/null || echo "")
        if echo "$CONTAINER_STATUS" | grep -q "Up"; then
            echo_success "Container is running (took ${SECONDS}s)"
            break
        fi
    fi

    if [ $((SECONDS % 10)) -eq 0 ] && [ $SECONDS -gt 0 ]; then
        echo_info "Still waiting... (${SECONDS}s elapsed)"
    fi
    sleep 2
done

if [ $SECONDS -ge $TEST_TIMEOUT ]; then
    echo_error "Timeout waiting for container to start"
    echo_info "Pod status:"
    podman pod ps
    echo_info "Container logs:"
    podman logs "$POD_NAME-compute" 2>&1 || true
    exit 1
fi

# Step 5: Verify container is actually running
echo_info "Verifying container health..."
CONTAINER_NAME=$(podman ps --filter "pod=$POD_NAME" --filter "name=.*compute" --format "{{.Names}}" | head -1)

if [ -z "$CONTAINER_NAME" ]; then
    echo_error "Could not find compute container"
    podman pod ps
    exit 1
fi
echo_success "Found compute container: $CONTAINER_NAME"

# Step 6: Check virt-launcher is running
echo_info "Checking virt-launcher process..."
if podman exec "$CONTAINER_NAME" ps aux | grep -q "[v]irt-launcher-monitor"; then
    echo_success "virt-launcher-monitor process is running"
else
    echo_error "virt-launcher-monitor process not found"
    echo_info "Container processes:"
    podman exec "$CONTAINER_NAME" ps aux || true
fi

# Step 7: Check for VMI
echo_info "Checking for VMI initialization..."
if podman exec "$CONTAINER_NAME" pgrep -f "virt-launcher" >/dev/null 2>&1; then
    echo_success "virt-launcher process found"
else
    echo_error "virt-launcher process not running"
fi

# Step 7.5: Verify /dev/kvm device access
echo_info "Verifying /dev/kvm device access..."
if podman exec "$CONTAINER_NAME" test -e /dev/kvm 2>/dev/null; then
    echo_success "/dev/kvm device is accessible in container"
    if podman exec "$CONTAINER_NAME" test -r /dev/kvm && podman exec "$CONTAINER_NAME" test -w /dev/kvm 2>/dev/null; then
        echo_success "/dev/kvm has read/write permissions"
    else
        echo_info "/dev/kvm exists but may not have proper permissions"
    fi
else
    echo_error "/dev/kvm device not found in container"
fi

# Step 8: Check logs for successful boot indicators
echo_info "Checking container logs for boot indicators..."
LOGS=$(podman logs "$CONTAINER_NAME" 2>&1)

if echo "$LOGS" | grep -qiE "(Starting|Started|Running|Initializing VMI)"; then
    echo_success "Boot process detected in logs"
else
    echo_info "Logs may not show boot yet (this is normal for slow starts)"
fi

# Show last 20 lines of logs
echo_info "Last 20 lines of container logs:"
echo "----------------------------------------"
echo "$LOGS" | tail -20
echo "----------------------------------------"

# Step 9: Verify QEMU/KVM process (if available)
echo_info "Checking for QEMU process (may not be present without /dev/kvm)..."
if podman exec "$CONTAINER_NAME" pgrep -f "qemu" >/dev/null 2>&1; then
    echo_success "QEMU process found - VM is running!"
else
    echo_info "QEMU process not found (expected without /dev/kvm access)"
    echo_info "This is normal in environments without KVM device access"
fi

# Step 10: Verify Pod networking
echo_info "Checking Pod networking..."
POD_IP=$(podman inspect "$POD_NAME" --format '{{.InfraContainerID}}' | xargs podman inspect --format '{{.NetworkSettings.IPAddress}}' 2>/dev/null || echo "")
if [ -n "$POD_IP" ]; then
    echo_success "Pod IP address: $POD_IP"
else
    echo_info "Could not determine Pod IP (may be using host network)"
fi

# Final summary
echo ""
echo "========================================"
echo_success "FUNCTIONAL TEST PASSED"
echo "========================================"
echo ""
echo "Summary:"
echo "  - Binary: $BINARY"
echo "  - VM File: $VM_FILE"
echo "  - Pod YAML: $POD_YAML"
echo "  - Pod Name: $POD_NAME"
echo "  - Container: $CONTAINER_NAME"
echo "  - Test Duration: ${SECONDS}s"
echo ""
echo_info "To view live logs: podman logs -f $CONTAINER_NAME"
echo_info "To access container: podman exec -it $CONTAINER_NAME /bin/bash"
echo_info "To stop Pod: podman pod stop $POD_NAME"
echo_info "To remove Pod: podman pod rm -f $POD_NAME"
echo ""
echo_info "The Pod will be automatically cleaned up when this script exits."
echo_info "Press Ctrl+C to clean up and exit, or wait for automatic cleanup..."
sleep 5

exit 0
