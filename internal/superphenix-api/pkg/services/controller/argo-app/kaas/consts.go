package kaas

import argoApp "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/argo-app"

var (
	CpuValueList    = []int{1, 2, 4, 8, 16, 32}
	MemoryValueList = []int{1, 2, 4, 8, 16, 32, 64}

	CpNetworkPolicies      = []string{"default", "none"}
	WorkersNetworkPolicies = []string{"default", "strict", "none"}
)

const (
	KaasPrefix     = "kaas"
	quantityFormat = "%dGi"
	maxnodeGroup   = 5

	// Disaster recovery (dedicated control plane datastore) requires Kubernetes >= 1.35.
	dataStoreMinMajor = 1
	dataStoreMinMinor = 35

	// Default dedicated-datastore size (GiB) when DR is enabled without an explicit size.
	defaultDataStoreStorageGi = 8
)

var ignoreDifferences = argoApp.IgnoreDifferences{
	argoApp.ResourceIgnoreDifferences{
		Group: "batch",
		Kind:  "Job",
		JSONPointers: []string{
			"/metadata/labels/app.kubernetes.io~1version",
			"/metadata/labels/helm.sh~1chart",
			"/metadata/labels/superphenix.net~1gitops",
			"/spec/template/metadata/labels/app.kubernetes.io~1version",
			"/spec/template/metadata/labels/helm.sh~1chart",
			"/spec/template/metadata/labels/superphenix.net~1gitops",
		},
	},
	argoApp.ResourceIgnoreDifferences{
		Group: "infrastructure.cluster.x-k8s.io",
		Kind:  "KubevirtMachineTemplate",
		JSONPointers: []string{
			"/metadata/labels/app.kubernetes.io~1version",
			"/metadata/labels/helm.sh~1chart",
			"/metadata/labels/superphenix.net~1gitops",
			"/spec/template/spec/virtualMachineTemplate/metadata/labels/app.kubernetes.io~1version",
			"/spec/template/spec/virtualMachineTemplate/metadata/labels/helm.sh~1chart",
			"/spec/template/spec/virtualMachineTemplate/metadata/labels/superphenix.net~1gitops",
			"/spec/template/spec/virtualMachineTemplate/spec/template/metadata/labels/app.kubernetes.io~1version",
			"/spec/template/spec/virtualMachineTemplate/spec/template/metadata/labels/helm.sh~1chart",
			"/spec/template/spec/virtualMachineTemplate/spec/template/metadata/labels/superphenix.net~1gitops",
		},
		JQPathExpressions: []string{
			".spec.template.spec.virtualMachineTemplate.spec.dataVolumeTemplates[].metadata.labels",
			".spec.template.spec.virtualMachineTemplate.spec.dataVolumeTemplates[0].spec.source.registry.url",
		},
	},
}
