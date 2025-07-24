module github.com/vladikr/kubevirt-vm-to-pod

go 1.21

require (
	github.com/kubevirt/kubevirt
	github.com/spf13/cobra v1.8.0
	k8s.io/api v0.32
	k8s.io/apimachinery v0.32
	k8s.io/client-go v0.32
	sigs.k8s.io/yaml v1.4.0
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	// ... (other indirect dependencies pulled by kubevirt/kubevirt)
)
