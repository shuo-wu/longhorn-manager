module github.com/longhorn/longhorn-manager

go 1.24.0

toolchain go1.24.5

// Replace directives are required for dependencies in this section because:
// - This module imports k8s.io/kubernetes.
// - The development for all of these dependencies is done at kubernetes/staging and then synced to other repos.
// - The go.mod file for k8s.io/kubernetes imports these dependencies with version v0.0.0 (which does not exist) and \
//   uses its own replace directives to load the appropriate code from kubernetes/staging.
// - Go is not able to find a version v0.0.0 for these dependencies and cannot meaningfully follow replace directives in
//   another go.mod file.
//
// The solution (which is used by all projects that import k8s.io/kubernetes) is to add replace directives for all
// k8s.io dependencies of k8s.io/kubernetes that k8s.io/kubernetes itself replaces in its go.mod file. The replace
// directives should pin the version of each dependency to the version of k8s.io/kubernetes that is imported. For
// example, if we import k8s.io/kubernetes v1.28.5, we should use v0.28.5 of all the replace directives. Depending on
// the portions of k8s.io/kubernetes code this module actually uses, not all of the replace directives may strictly be
// necessary. However, it is better to include all of them for consistency.

replace (
	k8s.io/api => k8s.io/api v0.33.3
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.33.3
	k8s.io/apimachinery => k8s.io/apimachinery v0.33.3
	k8s.io/apiserver => k8s.io/apiserver v0.33.3
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.33.3
	k8s.io/client-go => k8s.io/client-go v0.33.3
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.33.3
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.33.3
	k8s.io/code-generator => k8s.io/code-generator v0.33.3
	k8s.io/component-base => k8s.io/component-base v0.33.3
	k8s.io/component-helpers => k8s.io/component-helpers v0.33.3
	k8s.io/controller-manager => k8s.io/controller-manager v0.33.3
	k8s.io/cri-api => k8s.io/cri-api v0.33.3
	k8s.io/cri-client => k8s.io/cri-client v0.33.3
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.33.3
	k8s.io/dynamic-resource-allocation => k8s.io/dynamic-resource-allocation v0.33.3
	k8s.io/endpointslice => k8s.io/endpointslice v0.33.3
	k8s.io/externaljwt => k8s.io/externaljwt v0.33.3
	k8s.io/kms => k8s.io/kms v0.33.3
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.33.3
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.33.3
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.33.3
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.33.3
	k8s.io/kubectl => k8s.io/kubectl v0.33.3
	k8s.io/kubelet => k8s.io/kubelet v0.33.3
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.30.14
	k8s.io/metrics => k8s.io/metrics v0.33.3
	k8s.io/mount-utils => k8s.io/mount-utils v0.33.3
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.33.3
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.33.3
	k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.33.3
	k8s.io/sample-controller => k8s.io/sample-controller v0.33.3
)

require (
	github.com/container-storage-interface/spec v1.11.0
	github.com/docker/go-connections v0.5.0
	github.com/go-co-op/gocron v1.37.0
	github.com/google/uuid v1.6.0
	github.com/gorilla/handlers v1.5.2
	github.com/gorilla/mux v1.8.1
	github.com/gorilla/websocket v1.5.4-0.20250319132907-e064f32e3674
	github.com/jinzhu/copier v0.4.0
	github.com/kubernetes-csi/csi-lib-utils v0.22.0
	github.com/longhorn/backing-image-manager v1.9.0
	github.com/longhorn/backupstore v0.0.0-20250716050439-d920cc13cf0f
	github.com/longhorn/go-common-libs v0.0.0-20250712065607-11215ac4de96
	github.com/longhorn/go-iscsi-helper v0.0.0-20250713130221-69ce6f3960fa
	github.com/longhorn/go-spdk-helper v0.0.3-0.20250712161648-42d38592f838
	github.com/longhorn/longhorn-engine v1.9.0
	github.com/longhorn/longhorn-instance-manager v1.10.0-dev-20250629.0.20250711075830-f3729b840178
	github.com/longhorn/longhorn-share-manager v1.9.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.22.0
	github.com/rancher/dynamiclistener v0.7.0
	github.com/rancher/go-rancher v0.1.1-0.20220412083059-ff12399dd57b
	github.com/rancher/wrangler/v3 v3.2.2
	github.com/robfig/cron v1.2.0
	github.com/sirupsen/logrus v1.9.3
	github.com/stretchr/testify v1.10.0
	github.com/urfave/cli v1.22.17
	golang.org/x/mod v0.26.0
	golang.org/x/net v0.42.0
	golang.org/x/sys v0.34.0
	golang.org/x/time v0.12.0
	google.golang.org/grpc v1.73.0
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.33.3
	k8s.io/apiextensions-apiserver v0.33.3
	k8s.io/apimachinery v0.33.3
	k8s.io/cli-runtime v0.33.3
	k8s.io/client-go v0.33.3
	k8s.io/kubernetes v1.33.3
	k8s.io/metrics v0.33.3
	k8s.io/mount-utils v0.33.3
	k8s.io/utils v0.0.0-20250604170112-4c0f3b243397
	sigs.k8s.io/controller-runtime v0.21.0
)

require (
	github.com/Azure/go-ansiterm v0.0.0-20230124172434-306776ec8161 // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/fxamacker/cbor/v2 v2.7.0 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/google/gnostic-models v0.6.9 // indirect
	github.com/longhorn/types v0.0.0-20250710112743-e3a1e9e2a9c1 // indirect
	github.com/mitchellh/go-ps v1.0.0 // indirect
	github.com/moby/term v0.5.0 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/shirou/gopsutil/v3 v3.24.5 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	golang.org/x/exp v0.0.0-20250711185948-6ae5c78190dc // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250324211829-b45e905df463 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.12.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
)

require (
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/avast/retry-go v3.0.0+incompatible
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/c9s/goprocinfo v0.0.0-20210130143923-c95fcf8c64a8 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/emicklei/go-restful/v3 v3.12.1 // indirect
	github.com/evanphx/json-patch v5.9.11+incompatible // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/gammazero/deque v1.0.0 // indirect
	github.com/gammazero/workerpool v1.1.3 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.21.0 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/gorilla/context v1.1.2 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/liggitt/tabwriter v0.0.0-20181228230101-89fcab3d43de // indirect
	github.com/mailru/easyjson v0.9.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/sys/mountinfo v0.7.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.62.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/rancher/lasso v0.2.3 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/rogpeppe/go-internal v1.13.1 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/slok/goresilience v0.2.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	go.opentelemetry.io/otel v1.35.0 // indirect
	go.opentelemetry.io/otel/trace v1.35.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0
	golang.org/x/crypto v0.40.0 // indirect
	golang.org/x/oauth2 v0.28.0 // indirect
	golang.org/x/sync v0.16.0
	golang.org/x/term v0.33.0 // indirect
	golang.org/x/text v0.27.0
	google.golang.org/protobuf v1.36.6
	gopkg.in/inf.v0 v0.9.1 // indirect
	k8s.io/apiserver v0.33.3 // indirect
	k8s.io/component-base v0.33.3 // indirect
	k8s.io/component-helpers v0.33.0 // indirect
	k8s.io/controller-manager v0.33.0 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/kube-aggregator v0.33.1 // indirect
	k8s.io/kube-openapi v0.0.0-20250318190949-c8a335a9a2ff // indirect
	k8s.io/kubelet v0.0.0 // indirect
	sigs.k8s.io/json v0.0.0-20241014173422-cfa47c3a1cc8 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.7.0
	sigs.k8s.io/yaml v1.4.0 // indirect
)
