package main

import (
	"fmt"
	"os"
	"encoding/json"
	"github.com/spf13/cobra"
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
	forcePasst       bool
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
				launcherImage = "quay.io/kubevirt/virt-launcher:v1.7.0"
			}
			if addConsoleProxy && proxyImage == "" {
				proxyImage = "quay.io/vladikr/kubevirt-console-proxy:latest"
			}

			t := transformer.NewVMToPodTransformer(
				transformer.WithLauncherImage(launcherImage),
				transformer.WithInstancetypeFile(instancetypeFile),
				transformer.WithPreferenceFile(preferenceFile),
				transformer.WithAddConsoleProxy(addConsoleProxy, proxyImage, proxyPort),
				transformer.WithForcePasst(forcePasst),
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
	rootCmd.Flags().StringVar(&launcherImage, "launcher-image", "", "Virt-launcher image (default: quay.io/kubevirt/virt-launcher:v1.7.0)")
	rootCmd.Flags().StringVar(&instancetypeFile, "instancetype-file", "", "Path to Instancetype YAML file (optional)")
	rootCmd.Flags().StringVar(&preferenceFile, "preference-file", "", "Path to Preference YAML file (optional)")
	rootCmd.Flags().BoolVar(&addConsoleProxy, "add-console-proxy", false, "Add console proxy sidecar to the Pod")
	rootCmd.Flags().StringVar(&proxyImage, "proxy-image", "", "Console proxy image (default: quay.io/vladikr/kubevirt-console-proxy:latest)")
	rootCmd.Flags().IntVar(&proxyPort, "proxy-port", 8080, "Port for the console proxy to listen on")
	rootCmd.Flags().BoolVar(&forcePasst, "force-passt", false, "Force all network interfaces to use Passt binding")
	rootCmd.Flags().BoolVar(&mountDevices, "mount-devices", false, "Mount KVM devices (/dev/kvm, /dev/vhost-net, /dev/net/tun) for standalone execution")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
