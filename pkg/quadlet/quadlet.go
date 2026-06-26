package quadlet

import (
	"fmt"
	"strings"

	k8sv1 "k8s.io/api/core/v1"
)

func Generate(pod *k8sv1.Pod, yamlFilename string) string {
	vmName := pod.Name
	if vmName == "" {
		vmName = "kubevirt-vm"
	}

	var b strings.Builder

	fmt.Fprintf(&b, "[Unit]\n")
	fmt.Fprintf(&b, "Description=KubeVirt VM: %s\n", vmName)
	fmt.Fprintf(&b, "After=network-online.target\n")

	fmt.Fprintf(&b, "\n[Kube]\n")
	fmt.Fprintf(&b, "Yaml=%s\n", yamlFilename)

	for _, c := range pod.Spec.Containers {
		for _, p := range c.Ports {
			if p.ContainerPort != 0 {
				fmt.Fprintf(&b, "PublishPort=%d:%d\n", p.ContainerPort, p.ContainerPort)
			}
		}
	}

	fmt.Fprintf(&b, "\n[Service]\n")
	fmt.Fprintf(&b, "Restart=always\n")
	fmt.Fprintf(&b, "TimeoutStartSec=900\n")

	fmt.Fprintf(&b, "\n[Install]\n")
	fmt.Fprintf(&b, "WantedBy=default.target\n")

	return b.String()
}
