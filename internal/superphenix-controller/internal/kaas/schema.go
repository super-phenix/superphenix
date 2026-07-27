package kaas

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/cluster-api-provider-kubevirt/api/v1alpha1"
	"sigs.k8s.io/cluster-api/api/core/v1beta2"
)

var (
	clustersGVR = schema.GroupVersionResource{
		Group:    v1beta2.GroupVersion.Group,
		Version:  v1beta2.GroupVersion.Version,
		Resource: "clusters",
	}

	machineDeploymentsGVR = schema.GroupVersionResource{
		Group:    v1beta2.GroupVersion.Group,
		Version:  v1beta2.GroupVersion.Version,
		Resource: "machinedeployments",
	}

	kubevirtMachineTemplatesGVR = schema.GroupVersionResource{
		Group:    v1alpha1.GroupVersion.Group,
		Version:  v1alpha1.GroupVersion.Version,
		Resource: "kubevirtmachinetemplates",
	}
)
