module github.com/super-phenix/superphenix/internal/superphenix-controller

go 1.25.7

require (
	github.com/backube/snapscheduler v1.2.0
	github.com/go-chi/chi/v5 v5.3.1
	github.com/google/go-cmp v0.7.0
	github.com/gorilla/websocket v1.5.4-0.20250319132907-e064f32e3674
	github.com/kubeovn/kube-ovn v1.15.1
	github.com/prometheus/client_golang v1.23.2
	github.com/rs/zerolog v1.35.1
	github.com/spf13/pflag v1.0.10
	github.com/spf13/viper v1.21.0
	github.com/super-phenix/superphenix/pkg/chi-helper v0.0.0-00010101000000-000000000000
	github.com/super-phenix/superphenix/pkg/superphenix-id v0.0.0
	github.com/super-phenix/superphenix/pkg/utils v0.0.0
	github.com/swaggo/http-swagger v1.3.4
	github.com/swaggo/swag v1.16.6
	github.com/vmware-tanzu/velero v1.18.0
	go.opentelemetry.io/otel v1.40.0
	go.opentelemetry.io/otel/exporters/jaeger v1.17.0
	go.opentelemetry.io/otel/sdk v1.40.0
	go.uber.org/mock v0.6.0
	golang.org/x/sync v0.22.0
	k8s.io/api v0.35.0
	k8s.io/apimachinery v0.35.0
	k8s.io/client-go v12.0.0+incompatible
	kubevirt.io/api v1.8.0-alpha.0
	kubevirt.io/client-go v1.8.0-alpha.0
	sigs.k8s.io/cluster-api v1.12.2
	sigs.k8s.io/cluster-api-provider-kubevirt v0.11.1
)

require (
	github.com/KyleBanks/depth v1.2.1 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.13 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/spec v0.21.0 // indirect
	github.com/go-openapi/swag/cmdutils v0.25.4 // indirect
	github.com/go-openapi/swag/conv v0.25.4 // indirect
	github.com/go-openapi/swag/fileutils v0.25.4 // indirect
	github.com/go-openapi/swag/jsonname v0.25.4 // indirect
	github.com/go-openapi/swag/jsonutils v0.25.4 // indirect
	github.com/go-openapi/swag/loading v0.25.4 // indirect
	github.com/go-openapi/swag/mangling v0.25.4 // indirect
	github.com/go-openapi/swag/netutils v0.25.4 // indirect
	github.com/go-openapi/swag/stringutils v0.25.4 // indirect
	github.com/go-openapi/swag/typeutils v0.25.4 // indirect
	github.com/go-openapi/swag/yamlutils v0.25.4 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.30.3 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/google/gnostic-models v0.7.1 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/onsi/gomega v1.38.3 // indirect
	github.com/ovn-kubernetes/libovsdb v0.8.1 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.67.4 // indirect
	github.com/prometheus/procfs v0.19.2 // indirect
	github.com/sagikazarmark/locafero v0.12.0 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/swaggo/files v0.0.0-20220610200504-28940afbdbfe // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel/metric v1.40.0 // indirect
	go.opentelemetry.io/otel/trace v1.40.0 // indirect
	go.yaml.in/yaml/v2 v2.4.3 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.54.0 // indirect
	golang.org/x/mod v0.38.0 // indirect
	golang.org/x/tools v0.48.0 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.13.0 // indirect
	k8s.io/apiextensions-apiserver v0.35.0 // indirect
	sigs.k8s.io/controller-runtime v0.23.1 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.3.2 // indirect
)

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/emicklei/go-restful/v3 v3.13.0 // indirect
	github.com/go-kit/log v0.2.1 // indirect
	github.com/go-logfmt/logfmt v0.6.1 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-openapi/jsonpointer v0.22.4 // indirect
	github.com/go-openapi/jsonreference v0.21.4 // indirect
	github.com/go-openapi/swag v0.25.4 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/uuid v1.6.1-0.20241114170450-2d3c2a9cc518
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/k8snetworkplumbingwg/network-attachment-definition-client v1.7.7
	github.com/kubernetes-csi/external-snapshotter/client/v4 v4.2.0
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/openshift/api v0.0.0 // indirect
	github.com/openshift/client-go v3.9.0+incompatible // indirect
	github.com/openshift/custom-resource-status v1.1.2 // indirect
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.83.0 // indirect
	golang.org/x/net v0.57.0 // indirect
	golang.org/x/oauth2 v0.34.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/term v0.45.0 // indirect
	golang.org/x/text v0.40.0 // indirect
	golang.org/x/time v0.14.0 // indirect
	google.golang.org/protobuf v1.36.12-0.20260120151049-f2248ac996af // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	k8s.io/klog/v2 v2.140.0 // indirect
	k8s.io/kube-openapi v0.34.3 // indirect
	k8s.io/utils v0.0.0-20260210185600-b8788abfbbc2
	kubevirt.io/containerized-data-importer-api v1.64.0
	kubevirt.io/controller-lifecycle-operator-sdk/api v0.2.4 // indirect
	sigs.k8s.io/json v0.0.0-20250730193827-2d320260d730 // indirect
	sigs.k8s.io/yaml v1.6.0 // indirect
)

replace (
	github.com/go-kit/kit => github.com/go-kit/kit v0.13.0
	github.com/nxadm/tail => github.com/nxadm/tail v0.0.0-20211216163028-4472660a31a6
	github.com/openshift/api => github.com/openshift/api v0.0.0-20210105115604-44119421ec6b
	github.com/openshift/client-go => github.com/openshift/client-go v0.0.0-20210112165513-ebc401615f47
	github.com/operator-framework/operator-lifecycle-manager => github.com/operator-framework/operator-lifecycle-manager v0.0.0-20190128024246-5eb7ae5bdb7a
)

replace (
	k8s.io/api => k8s.io/api v0.34.2
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.34.2
	k8s.io/apimachinery => k8s.io/apimachinery v0.34.2
	k8s.io/apiserver => k8s.io/apiserver v0.34.2

	k8s.io/cli-runtime => k8s.io/cli-runtime v0.34.2
	k8s.io/client-go => k8s.io/client-go v0.34.2
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.34.2
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.34.2
	k8s.io/code-generator => k8s.io/code-generator v0.34.2
	k8s.io/component-base => k8s.io/component-base v0.34.2
	k8s.io/cri-api => k8s.io/cri-api v0.34.2
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.34.2
	k8s.io/dynamic-resource-allocation => k8s.io/dynamic-resource-allocation v0.34.2
	k8s.io/klog => k8s.io/klog v0.4.0
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.34.2
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.34.2
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20250710124328-f3f2b991d03b
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.34.2
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.34.2
	k8s.io/kubectl => k8s.io/kubectl v0.34.2
	k8s.io/kubelet => k8s.io/kubelet v0.34.2
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.34.2
	k8s.io/metrics => k8s.io/metrics v0.34.2
	k8s.io/mount-utils => k8s.io/mount-utils v0.34.2
	k8s.io/node-api => k8s.io/node-api v0.34.2
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.34.2
	k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.34.2
	k8s.io/sample-controller => k8s.io/sample-controller v0.34.2
)

replace github.com/super-phenix/superphenix/pkg/utils => ../../pkg/utils

replace github.com/super-phenix/superphenix/pkg/superphenix-id => ../../pkg/superphenix-id

replace github.com/super-phenix/superphenix/pkg/chi-helper => ../../pkg/chi-helper
