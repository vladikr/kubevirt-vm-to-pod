#!/bin/bash
# Demo: Run a KubeVirt VM as a standalone Pod with Podman
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
echo " KubeVirt VM-to-Pod: Standalone VM Demo"
echo " Run KubeVirt VMs without Kubernetes!"
echo "=============================================="
sleep $DELAY

echo ""
echo "--- Step 1: Show the VM definition ---"
type_cmd "cat test-vm.yaml"
sleep $DELAY

echo ""
echo "--- Step 2: Generate Pod manifest from VM ---"
type_cmd "$BIN --vm-file test-vm.yaml --mount-devices | tee pod.yaml"
sleep $DELAY

echo ""
echo "--- Step 3: Run the Pod with podman kube play ---"
type_cmd "podman kube play pod.yaml"

echo ""
echo "  Waiting for VM to boot..."
sleep 60

echo ""
echo "--- Step 4: Check container status ---"
type_cmd "podman ps --filter pod=virt-launcher-testvm --format 'table {{.Names}}\t{{.Status}}'"
sleep $DELAY

echo ""
echo "--- Step 5: Connect to VM console ---"
echo -e "\033[1;32m\$ $BIN console testvm\033[0m"
sleep 1

python3 << 'PYSCRIPT'
import pexpect, sys, time

child = pexpect.spawn('./kubevirt-vm-to-pod console testvm', encoding='utf-8', timeout=20)
child.logfile_read = sys.stdout

child.expect('Connected to serial console', timeout=15)

# Drain the buffered boot log silently
child.logfile_read = None
time.sleep(2)
try:
    while True:
        child.read_nonblocking(size=4096, timeout=1)
except pexpect.TIMEOUT:
    pass

# Resume logging and interact with a clean prompt
child.logfile_read = sys.stdout
sys.stdout.write('\r\n')
child.sendline('')
child.expect('login:', timeout=15)
child.sendline('cirros')
child.expect('Password:', timeout=10)
child.sendline('gocubsgo')
child.expect(r'\$\s', timeout=10)
child.sendline('uname -a')
child.expect(r'\$\s', timeout=10)
time.sleep(1)
# Ctrl+]
child.send(chr(0x1d))
child.expect('Disconnected', timeout=5)
PYSCRIPT

sleep $DELAY

echo ""
echo "--- Step 6: Cleanup ---"
type_cmd "podman kube down pod.yaml"

echo ""
echo "=============================================="
echo " Demo complete!"
echo " VM ran as a standalone Pod using Podman"
echo " with Passt networking and serial console."
echo "=============================================="
