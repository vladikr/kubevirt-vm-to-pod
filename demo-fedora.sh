#!/bin/bash
# Demo: Run a Fedora KubeVirt VM as a standalone Pod with Podman
set -e

DELAY=2
BIN="./kubevirt-vm-to-pod"

type_cmd() {
    echo ""
    echo -e "\033[1;32m\$ $1\033[0m"
    sleep $DELAY
    eval "$1"
}

echo "=============================================="
echo " KubeVirt VM-to-Pod: Fedora VM Demo"
echo " Run KubeVirt VMs without Kubernetes!"
echo "=============================================="
sleep $DELAY

echo ""
echo "--- Step 1: Show the Fedora VM definition ---"
type_cmd "cat test-vm-fedora.yaml"
sleep $DELAY

echo ""
echo "--- Step 2: Generate Pod manifest from VM ---"
type_cmd "$BIN --vm-file test-vm-fedora.yaml --mount-devices | tee pod-fedora.yaml"
sleep $DELAY

echo ""
echo "--- Step 3: Run the Pod with podman kube play ---"
type_cmd "podman kube play pod-fedora.yaml"

echo ""
echo "  Waiting for Fedora to boot (cloud-init takes ~90s)..."
sleep 90

echo ""
echo "--- Step 4: Check container status ---"
type_cmd "podman ps --filter pod=virt-launcher-fedora-vm --format 'table {{.Names}}\t{{.Status}}'"
sleep $DELAY

echo ""
echo "--- Step 5: Connect to VM console ---"
echo -e "\033[1;32m\$ $BIN console fedora-vm\033[0m"
sleep 1

python3 << 'PYSCRIPT'
import pexpect, sys, time

child = pexpect.spawn('./kubevirt-vm-to-pod console fedora-vm', encoding='utf-8', timeout=20)
child.logfile_read = sys.stdout

child.expect('Connected to serial console', timeout=15)

# Drain buffered boot log silently
child.logfile_read = None
time.sleep(2)
try:
    while True:
        child.read_nonblocking(size=4096, timeout=1)
except pexpect.TIMEOUT:
    pass

# Resume logging and interact
child.logfile_read = sys.stdout
child.sendline('')
child.expect('login:', timeout=15)
child.sendline('fedora')
child.expect('Password:', timeout=10)
child.sendline('fedora')
child.expect(r'[\$#]', timeout=10)
child.sendline('uname -a')
child.expect(r'[\$#]', timeout=10)
child.sendline('cat /etc/fedora-release')
child.expect(r'[\$#]', timeout=10)
child.sendline('ip addr show eth0')
child.expect(r'[\$#]', timeout=10)
time.sleep(1)
# Ctrl+]
child.send(chr(0x1d))
child.expect('Disconnected', timeout=5)
PYSCRIPT

sleep $DELAY

echo ""
echo "--- Step 6: Cleanup ---"
type_cmd "podman kube down pod-fedora.yaml"

echo ""
echo "=============================================="
echo " Demo complete!"
echo " Fedora 40 VM ran as a standalone Pod"
echo " with Passt networking and serial console."
echo "=============================================="
