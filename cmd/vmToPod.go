package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"sigs.k8s.io/yaml"

	"github.com/vladikr/kubevirt-vm-to-pod/pkg/transformer"
)

var (
	vmFile           string
	output           string
	launcherImage    string
	instancetypeFile string
	preferenceFile   string
	addConsoleProxy  bool
	proxyImage       string
	proxyPort        int
	noPasst          bool
	mountDevices     bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "kubevirt-vm-to-pod",
		Short: "Generate Pod YAML from a KubeVirt VirtualMachine YAML",
		RunE: func(cmd *cobra.Command, args []string) error {
			if vmFile == "" {
				return fmt.Errorf("vm-file is required")
			}
			if output != "yaml" && output != "json" {
				return fmt.Errorf("output must be 'yaml' or 'json'")
			}
			if launcherImage == "" {
				launcherImage = "quay.io/kubevirt/virt-launcher:v1.8.0"
			}
			if addConsoleProxy && proxyImage == "" {
				proxyImage = "quay.io/vladikr/kubevirt-console-proxy:latest"
			}

			t := transformer.NewVMToPodTransformer(
				transformer.WithLauncherImage(launcherImage),
				transformer.WithInstancetypeFile(instancetypeFile),
				transformer.WithPreferenceFile(preferenceFile),
				transformer.WithAddConsoleProxy(addConsoleProxy, proxyImage, proxyPort),
				transformer.WithForcePasst(!noPasst),
				transformer.WithMountDevices(mountDevices),
			)
			pod, err := t.Transform(vmFile)
			if err != nil {
				return fmt.Errorf("failed to transform VM to Pod: %v", err)
			}

			var outputBytes []byte
			if output == "yaml" {
				outputBytes, err = yaml.Marshal(pod)
			} else {
				outputBytes, err = json.MarshalIndent(pod, "", "  ")
			}
			if err != nil {
				return fmt.Errorf("failed to marshal Pod: %v", err)
			}

			fmt.Println(string(outputBytes))
			return nil
		},
	}

	rootCmd.Flags().StringVar(&vmFile, "vm-file", "", "Path to VirtualMachine YAML file (required)")
	rootCmd.Flags().StringVar(&output, "output", "yaml", "Output format: yaml or json")
	rootCmd.Flags().StringVar(&launcherImage, "launcher-image", "", "Virt-launcher image (default: quay.io/kubevirt/virt-launcher:v1.8.0)")
	rootCmd.Flags().StringVar(&instancetypeFile, "instancetype-file", "", "Path to Instancetype YAML file (optional)")
	rootCmd.Flags().StringVar(&preferenceFile, "preference-file", "", "Path to Preference YAML file (optional)")
	rootCmd.Flags().BoolVar(&addConsoleProxy, "add-console-proxy", false, "Add console proxy sidecar to the Pod")
	rootCmd.Flags().StringVar(&proxyImage, "proxy-image", "", "Console proxy image (default: quay.io/vladikr/kubevirt-console-proxy:latest)")
	rootCmd.Flags().IntVar(&proxyPort, "proxy-port", 8080, "Port for the console proxy to listen on")
	rootCmd.Flags().BoolVar(&noPasst, "no-passt", false, "Preserve original network bindings instead of converting to Passt (requires CNI plugins)")
	rootCmd.Flags().BoolVar(&mountDevices, "mount-devices", false, "Mount KVM devices (/dev/kvm, /dev/vhost-net, /dev/net/tun) for standalone execution")

	consoleCmd := &cobra.Command{
		Use:   "console <vm-name>",
		Short: "Attach to the serial console of a running VM",
		Long:  "Opens an interactive serial console to a VM running in a podman pod.\nCopies itself into the container and connects to the serial socket directly.\nPress Ctrl+] to disconnect.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vmName := args[0]
			containerName := fmt.Sprintf("virt-launcher-%s-compute", vmName)

			// Copy ourselves into the container
			self, err := os.Executable()
			if err != nil {
				return fmt.Errorf("failed to find executable path: %v", err)
			}

			copyCmd := exec.Command("podman", "cp", self, containerName+":/tmp/vm-console")
			copyCmd.Stderr = os.Stderr
			if err := copyCmd.Run(); err != nil {
				return fmt.Errorf("failed to copy binary into container (is %s running?): %v", containerName, err)
			}

			// Exec ourselves inside the container in attach mode
			execCmd := exec.Command("podman", "exec", "-it", containerName,
				"/tmp/vm-console", "attach",
				"--socket", "/var/run/kubevirt-private/virt-serial0")
			execCmd.Stdin = os.Stdin
			execCmd.Stdout = os.Stdout
			execCmd.Stderr = os.Stderr
			return execCmd.Run()
		},
	}

	// attach subcommand — runs inside the container, connects stdin/stdout to a Unix socket
	attachCmd := &cobra.Command{
		Use:    "attach",
		Short:  "Connect stdin/stdout to a serial socket (runs inside container)",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			socketPath, _ := cmd.Flags().GetString("socket")
			return runAttach(socketPath)
		},
	}
	attachCmd.Flags().String("socket", "/var/run/kubevirt-private/virt-serial0", "Path to serial Unix socket")

	rootCmd.AddCommand(consoleCmd)
	rootCmd.AddCommand(attachCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runAttach(socketPath string) error {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %v", socketPath, err)
	}
	defer conn.Close()

	// Put terminal in raw mode
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("failed to set raw terminal: %v", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	// Handle signals to restore terminal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		term.Restore(int(os.Stdin.Fd()), oldState)
		os.Exit(0)
	}()

	fmt.Fprintf(os.Stdout, "Connected to serial console. Press Ctrl+] to disconnect.\r\n")

	errCh := make(chan error, 2)

	// socket -> stdout
	go func() {
		_, err := io.Copy(os.Stdout, conn)
		errCh <- err
	}()

	// stdin -> socket (with Ctrl+] detection)
	go func() {
		buf := make([]byte, 256)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				errCh <- err
				return
			}
			for i := 0; i < n; i++ {
				if buf[i] == 0x1d { // Ctrl+]
					fmt.Fprintf(os.Stdout, "\r\nDisconnected.\r\n")
					errCh <- nil
					return
				}
			}
			if _, err := conn.Write(buf[:n]); err != nil {
				errCh <- err
				return
			}
		}
	}()

	<-errCh
	return nil
}
