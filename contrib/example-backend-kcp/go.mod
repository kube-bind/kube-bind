module github.com/kube-bind/kube-bind/contrib/example-backend-kcp

go 1.23.4

// These are just for local development and example. You can remove them and point to the actual releases.
replace (
	github.com/kube-bind/kube-bind => ../../
	github.com/kube-bind/kube-bind/sdk/apis => ../../sdk/apis
	github.com/kube-bind/kube-bind/sdk/kcp => ../../sdk/kcp
)

// These replace follows kcp version. Currently it is 0.26.1.
replace (
	k8s.io/api => github.com/kcp-dev/kubernetes/staging/src/k8s.io/api v0.0.0-20240918143026-ab5c3a6448cb
	k8s.io/apiextensions-apiserver => github.com/kcp-dev/kubernetes/staging/src/k8s.io/apiextensions-apiserver v0.0.0-20240918143026-ab5c3a6448cb
	k8s.io/apimachinery => github.com/kcp-dev/kubernetes/staging/src/k8s.io/apimachinery v0.0.0-20240918143026-ab5c3a6448cb
	k8s.io/apiserver => github.com/kcp-dev/kubernetes/staging/src/k8s.io/apiserver v0.0.0-20240918143026-ab5c3a6448cb
	k8s.io/cli-runtime => github.com/kcp-dev/kubernetes/staging/src/k8s.io/cli-runtime v0.0.0-20240918143026-ab5c3a6448cb
	k8s.io/client-go => github.com/kcp-dev/kubernetes/staging/src/k8s.io/client-go v0.0.0-20240918143026-ab5c3a6448cb
	k8s.io/cloud-provider => github.com/kcp-dev/kubernetes/staging/src/k8s.io/cloud-provider v0.0.0-20240918143026-ab5c3a6448cb
	k8s.io/cluster-bootstrap => github.com/kcp-dev/kubernetes/staging/src/k8s.io/cluster-bootstrap v0.0.0-20240918143026-ab5c3a6448cb
	k8s.io/code-generator => github.com/kcp-dev/kubernetes/staging/src/k8s.io/code-generator v0.0.0-20240918143026-ab5c3a6448cb
	k8s.io/component-base => github.com/kcp-dev/kubernetes/staging/src/k8s.io/component-base v0.0.0-20240918143026-ab5c3a6448cb
	k8s.io/component-helpers => github.com/kcp-dev/kubernetes/staging/src/k8s.io/component-helpers v0.0.0-20240918143026-ab5c3a6448cb
	k8s.io/controller-manager => github.com/kcp-dev/kubernetes/staging/src/k8s.io/controller-manager v0.0.0-20240918143026-ab5c3a6448cb
	k8s.io/cri-api => github.com/kcp-dev/kubernetes/staging/src/k8s.io/cri-api v0.0.0-20240918143026-ab5c3a6448cb
	k8s.io/cri-client => github.com/kcp-dev/kubernetes/staging/src/k8s.io/cri-client v0.0.0-20240918143026-ab5c3a6448cb
	k8s.io/csi-translation-lib => github.com/kcp-dev/kubernetes/staging/src/k8s.io/csi-translation-lib v0.0.0-20240918143026-ab5c3a6448cb
	k8s.io/dynamic-resource-allocation => github.com/kcp-dev/kubernetes/staging/src/k8s.io/dynamic-resource-allocation v0.0.0-20240918143026-ab5c3a6448cb
	k8s.io/endpointslice => github.com/kcp-dev/kubernetes/staging/src/k8s.io/endpointslice v0.0.0-20240918143026-ab5c3a6448cb
	k8s.io/kms => github.com/kcp-dev/kubernetes/staging/src/k8s.io/kms v0.0.0-20240918143026-ab5c3a6448cb
	k8s.io/kube-aggregator => github.com/kcp-dev/kubernetes/staging/src/k8s.io/kube-aggregator v0.0.0-20240918143026-ab5c3a6448cb
	k8s.io/kube-controller-manager => github.com/kcp-dev/kubernetes/staging/src/k8s.io/kube-controller-manager v0.0.0-20240918143026-ab5c3a6448cb
	k8s.io/kube-proxy => github.com/kcp-dev/kubernetes/staging/src/k8s.io/kube-proxy v0.0.0-20240918143026-ab5c3a6448cb
	k8s.io/kube-scheduler => github.com/kcp-dev/kubernetes/staging/src/k8s.io/kube-scheduler v0.0.0-20240918143026-ab5c3a6448cb
	k8s.io/kubectl => github.com/kcp-dev/kubernetes/staging/src/k8s.io/kubectl v0.0.0-20240918143026-ab5c3a6448cb
	k8s.io/kubelet => github.com/kcp-dev/kubernetes/staging/src/k8s.io/kubelet v0.0.0-20240918143026-ab5c3a6448cb
	k8s.io/kubernetes => github.com/kcp-dev/kubernetes v0.0.0-20240918143026-ab5c3a6448cb
	k8s.io/legacy-cloud-providers => github.com/kcp-dev/kubernetes/staging/src/k8s.io/legacy-cloud-providers v0.0.0-20240918143026-ab5c3a6448cb
	k8s.io/metrics => github.com/kcp-dev/kubernetes/staging/src/k8s.io/metrics v0.0.0-20240918143026-ab5c3a6448cb
	k8s.io/mount-utils => github.com/kcp-dev/kubernetes/staging/src/k8s.io/mount-utils v0.0.0-20240918143026-ab5c3a6448cb
	k8s.io/pod-security-admission => github.com/kcp-dev/kubernetes/staging/src/k8s.io/pod-security-admission v0.0.0-20240918143026-ab5c3a6448cb
	k8s.io/sample-apiserver => github.com/kcp-dev/kubernetes/staging/src/k8s.io/sample-apiserver v0.0.0-20240918143026-ab5c3a6448cb
	k8s.io/sample-cli-plugin => github.com/kcp-dev/kubernetes/staging/src/k8s.io/sample-cli-plugin v0.0.0-20240918143026-ab5c3a6448cb
	k8s.io/sample-controller => github.com/kcp-dev/kubernetes/staging/src/k8s.io/sample-controller v0.0.0-20240918143026-ab5c3a6448cb
)

require (
	github.com/coreos/go-oidc v2.2.1+incompatible
	github.com/evanphx/json-patch v5.6.0+incompatible
	github.com/google/go-cmp v0.6.0
	github.com/gorilla/mux v1.8.1
	github.com/gorilla/securecookie v1.1.2
	github.com/kcp-dev/apimachinery/v2 v2.0.1-0.20240817110845-a9eb9752bfeb
	github.com/kcp-dev/client-go v0.0.0-20240912145314-f5949d81732a
	github.com/kcp-dev/kcp v0.26.0
	github.com/kcp-dev/kcp/sdk v0.26.0
	github.com/kcp-dev/logicalcluster/v3 v3.0.5
	github.com/kube-bind/kube-bind v0.4.6
	github.com/kube-bind/kube-bind/sdk/apis v0.4.6
	github.com/kube-bind/kube-bind/sdk/kcp v0.4.6
	github.com/pierrec/lz4 v2.6.1+incompatible
	github.com/spf13/pflag v1.0.6-0.20210604193023-d5e0c0615ace
	github.com/vmihailenco/msgpack/v4 v4.3.13
	golang.org/x/oauth2 v0.24.0
	k8s.io/api v0.32.0
	k8s.io/apiextensions-apiserver v0.32.0
	k8s.io/apimachinery v0.32.0
	k8s.io/apiserver v0.32.0
	k8s.io/client-go v0.32.0
	k8s.io/code-generator v0.32.0
	k8s.io/component-base v0.32.0
	k8s.io/klog/v2 v2.130.1
	k8s.io/utils v0.0.0-20241210054802-24370beab758
	sigs.k8s.io/controller-tools v0.16.1
)

require (
	cel.dev/expr v0.18.0 // indirect
	github.com/NYTimes/gziphandler v1.1.1 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.0 // indirect
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/bombsimon/logrusr/v3 v3.1.0 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/coreos/go-semver v0.3.1 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/emicklei/go-restful/v3 v3.11.0 // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/fxamacker/cbor/v2 v2.7.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/gobuffalo/flect v1.0.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/cel-go v0.22.0 // indirect
	github.com/google/gnostic-models v0.6.8 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.20.0 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/onsi/gomega v1.36.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pquerna/cachecontrol v0.1.0 // indirect
	github.com/prometheus/client_golang v1.19.1 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.55.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/spf13/cobra v1.8.1 // indirect
	github.com/stoewer/go-strcase v1.3.0 // indirect
	github.com/vmihailenco/tagparser v0.1.1 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.etcd.io/etcd/api/v3 v3.5.16 // indirect
	go.etcd.io/etcd/client/pkg/v3 v3.5.16 // indirect
	go.etcd.io/etcd/client/v3 v3.5.16 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.53.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.53.0 // indirect
	go.opentelemetry.io/otel v1.28.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.28.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.27.0 // indirect
	go.opentelemetry.io/otel/metric v1.28.0 // indirect
	go.opentelemetry.io/otel/sdk v1.28.0 // indirect
	go.opentelemetry.io/otel/trace v1.28.0 // indirect
	go.opentelemetry.io/proto/otlp v1.3.1 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/crypto v0.31.0 // indirect
	golang.org/x/exp v0.0.0-20240823005443-9b4947da3948 // indirect
	golang.org/x/mod v0.21.0 // indirect
	golang.org/x/net v0.30.0 // indirect
	golang.org/x/sync v0.10.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
	golang.org/x/term v0.27.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	golang.org/x/time v0.7.0 // indirect
	golang.org/x/tools v0.26.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240826202546-f6391c0de4c7 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240826202546-f6391c0de4c7 // indirect
	google.golang.org/grpc v1.65.0 // indirect
	google.golang.org/protobuf v1.35.1 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.12.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/square/go-jose.v2 v2.6.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/gengo/v2 v2.0.0-20240911193312-2b36238f13e9 // indirect
	k8s.io/kube-openapi v0.0.0-20241105132330-32ad38e42d3f // indirect
	sigs.k8s.io/apiserver-network-proxy/konnectivity-client v0.31.0 // indirect
	sigs.k8s.io/json v0.0.0-20241010143419-9aa6b5e7a4b3 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.5.0 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)