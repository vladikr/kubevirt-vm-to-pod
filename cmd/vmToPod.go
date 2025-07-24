package main

import (
	"fmt"
	"os"

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
				launcherImage = "quay.io/kubevirt/virt-launcher:latest"
			}

			t := transformer.NewVMToPodTransformer(
				transformer.WithLauncherImage(launcherImage),
				transformer.WithInstancetypeFile(instancetypeFile),
				transformer.WithPreferenceFile(preferenceFile),
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
	rootCmd.Flags().StringVar(&launcherImage, "launcher-image", "", "Virt-launcher image (default: quay.io/kubevirt/virt-launcher:latest)")
	rootCmd.Flags().StringVar(&instancetypeFile, "instancetype-file", "", "Path to Instancetype YAML file (optional)")
	rootCmd.Flags().StringVar(&preferenceFile, "preference-file", "", "Path to Preference YAML file (optional)")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
