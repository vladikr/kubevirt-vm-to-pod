package quadlet

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	k8sv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGenerate(t *testing.T) {
	t.Run("basic pod produces valid quadlet", func(t *testing.T) {
		pod := &k8sv1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "virt-launcher-myvm",
			},
		}

		result := Generate(pod, "virt-launcher-myvm-pod.yaml")

		require.Contains(t, result, "[Unit]")
		require.Contains(t, result, "Description=KubeVirt VM: virt-launcher-myvm")
		require.Contains(t, result, "After=network-online.target")

		require.Contains(t, result, "[Kube]")
		require.Contains(t, result, "Yaml=virt-launcher-myvm-pod.yaml")

		require.Contains(t, result, "[Service]")
		require.Contains(t, result, "Restart=always")
		require.Contains(t, result, "TimeoutStartSec=900")

		require.Contains(t, result, "[Install]")
		require.Contains(t, result, "WantedBy=default.target")
	})

	t.Run("pod with container ports publishes them", func(t *testing.T) {
		pod := &k8sv1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "virt-launcher-myvm",
			},
			Spec: k8sv1.PodSpec{
				Containers: []k8sv1.Container{
					{
						Name: "console-proxy",
						Ports: []k8sv1.ContainerPort{
							{ContainerPort: 8080},
							{ContainerPort: 15900},
						},
					},
				},
			},
		}

		result := Generate(pod, "myvm-pod.yaml")

		require.Contains(t, result, "PublishPort=8080:8080")
		require.Contains(t, result, "PublishPort=15900:15900")
	})

	t.Run("pod without name uses fallback", func(t *testing.T) {
		pod := &k8sv1.Pod{}

		result := Generate(pod, "pod.yaml")

		require.Contains(t, result, "Description=KubeVirt VM: kubevirt-vm")
	})

	t.Run("sections are in correct order", func(t *testing.T) {
		pod := &k8sv1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "test-vm"},
		}

		result := Generate(pod, "test-vm-pod.yaml")

		unitIdx := strings.Index(result, "[Unit]")
		kubeIdx := strings.Index(result, "[Kube]")
		serviceIdx := strings.Index(result, "[Service]")
		installIdx := strings.Index(result, "[Install]")

		require.True(t, unitIdx < kubeIdx, "[Unit] should come before [Kube]")
		require.True(t, kubeIdx < serviceIdx, "[Kube] should come before [Service]")
		require.True(t, serviceIdx < installIdx, "[Service] should come before [Install]")
	})
}
