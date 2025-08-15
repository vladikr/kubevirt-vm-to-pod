module github.com/vladikr/kubevirt-vm-to-pod

go 1.23.0

toolchain go1.24.5

require (
	github.com/spf13/cobra v1.8.1
	kubevirt.io/kubevirt v1.6.0 // Transitive deps will handle k8s.io and others
	sigs.k8s.io/yaml v1.4.0
)

require (
	github.com/gorilla/websocket v1.5.0
	github.com/stretchr/testify v1.9.0
	k8s.io/api v0.32.5
	k8s.io/apimachinery v0.32.5
	k8s.io/client-go v12.0.0+incompatible
	kubevirt.io/api v0.0.0-00010101000000-000000000000
)

require (
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/coreos/go-semver v0.3.1 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/emicklei/go-restful/v3 v3.11.0 // indirect
	github.com/evanphx/json-patch v5.6.0+incompatible // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/fxamacker/cbor/v2 v2.7.0 // indirect
	github.com/go-kit/log v0.2.1 // indirect
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-openapi/analysis v0.23.0 // indirect
	github.com/go-openapi/errors v0.22.0 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.21.0 // indirect
	github.com/go-openapi/loads v0.22.0 // indirect
	github.com/go-openapi/spec v0.21.0 // indirect
	github.com/go-openapi/strfmt v0.23.0 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/go-openapi/validate v0.24.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/btree v1.1.3 // indirect
	github.com/google/gnostic-models v0.6.8 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/imdario/mergo v0.3.16 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/insomniacslk/dhcp v0.0.0-20230908212754-65c27093e38a // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/josharian/native v1.1.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/k8snetworkplumbingwg/network-attachment-definition-client v1.3.0 // indirect
	github.com/krolaw/dhcp4 v0.0.0-20180925202202-7cead472c414 // indirect
	github.com/kubernetes-csi/external-snapshotter/client/v4 v4.2.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/oklog/ulid v1.3.1 // indirect
	github.com/onsi/gomega v1.36.2 // indirect
	github.com/opencontainers/selinux v1.11.0 // indirect
	github.com/openshift/api v0.0.0 // indirect
	github.com/openshift/client-go v0.0.0 // indirect
	github.com/openshift/custom-resource-status v1.1.2 // indirect
	github.com/openshift/library-go v0.0.0-20211220195323-eca2c467c492 // indirect
	github.com/pierrec/lz4/v4 v4.1.18 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.68.0 // indirect
	github.com/prometheus/client_golang v1.19.1 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.55.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/rhobs/operator-observability-toolkit v0.0.29 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/u-root/uio v0.0.0-20230220225925-ffce2a382923 // indirect
	github.com/vishvananda/netlink v1.3.0 // indirect
	github.com/vishvananda/netns v0.0.4 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.mongodb.org/mongo-driver v1.14.0 // indirect
	go.uber.org/mock v0.5.1 // indirect
	golang.org/x/net v0.38.0 // indirect
	golang.org/x/oauth2 v0.27.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
	golang.org/x/term v0.30.0 // indirect
	golang.org/x/text v0.23.0 // indirect
	golang.org/x/time v0.7.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240826202546-f6391c0de4c7 // indirect
	google.golang.org/grpc v1.65.0 // indirect
	google.golang.org/protobuf v1.36.1 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.12.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apiextensions-apiserver v0.32.5 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/kube-aggregator v0.26.4 // indirect
	k8s.io/kube-openapi v0.31.0 // indirect
	k8s.io/kubectl v0.0.0-00010101000000-000000000000 // indirect
	k8s.io/utils v0.0.0-20241104100929-3ea5e8cea738 // indirect
	kubevirt.io/client-go v0.0.0-00010101000000-000000000000 // indirect
	kubevirt.io/containerized-data-importer-api v1.60.3-0.20241105012228-50fbed985de9 // indirect
	kubevirt.io/controller-lifecycle-operator-sdk/api v0.0.0-20220329064328-f3cc58c6ed90 // indirect
	sigs.k8s.io/controller-runtime v0.20.2 // indirect
	sigs.k8s.io/json v0.0.0-20241010143419-9aa6b5e7a4b3 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.3 // indirect
)

replace kubevirt.io/kubevirt => ../kubevirt

replace kubevirt.io/client-go => ../kubevirt/staging/src/kubevirt.io/client-go

replace kubevirt.io/api => ../kubevirt/staging/src/kubevirt.io/api

// From KubeVirt's go.mod to handle OpenShift and old operator deps
replace github.com/openshift/api => github.com/openshift/api v0.0.0-20210105115604-44119421ec6b

replace github.com/openshift/client-go => github.com/openshift/client-go v0.0.0-20210112165513-ebc401615f47

replace github.com/operator-framework/operator-lifecycle-manager => github.com/operator-framework/operator-lifecycle-manager v0.0.0-20190128024246-5eb7ae5bdb7a

// From KubeVirt's go.mod to pin k8s.io deps to v0.32.5 (fixes latest v0.33.3 pull and missing packages)
replace k8s.io/api => k8s.io/api v0.32.5

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.32.5

replace k8s.io/apimachinery => k8s.io/apimachinery v0.32.5

replace k8s.io/apiserver => k8s.io/apiserver v0.32.5

replace k8s.io/client-go => k8s.io/client-go v0.32.5

replace k8s.io/cloud-provider => k8s.io/cloud-provider v0.32.5

replace k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.32.5

replace k8s.io/code-generator => k8s.io/code-generator v0.32.5

replace k8s.io/component-base => k8s.io/component-base v0.32.5

replace k8s.io/cri-api => k8s.io/cri-api v0.32.5

replace k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.32.5

replace k8s.io/klog => k8s.io/klog v0.4.0

replace k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.32.5

replace k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.32.5

replace k8s.io/kube-proxy => k8s.io/kube-proxy v0.32.5

replace k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.32.5

replace k8s.io/kubectl => k8s.io/kubectl v0.32.5

replace k8s.io/kubelet => k8s.io/kubelet v0.32.5

replace k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.32.5

replace k8s.io/metrics => k8s.io/metrics v0.32.5

replace k8s.io/node-api => k8s.io/node-api v0.32.5

replace k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.32.5

replace k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.32.5

replace k8s.io/sample-controller => k8s.io/sample-controller v0.32.5

// From KubeVirt's go.mod to pin kube-openapi (fixes unknown revision)
replace k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20240430033511-f0e62f92d13f
