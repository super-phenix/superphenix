package view

import (
	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PostgresClusterView is a trimmed CloudNative-PG Cluster.
type PostgresClusterView struct {
	ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec PostgresClusterSpec `json:"spec,omitempty"`

	// +optional
	Status PostgresClusterStatus `json:"status,omitempty"`
}

type PostgresClusterSpec struct {
	// Instances is the number of instances, one primary and the others replicas.
	Instances int `json:"instances,omitempty"`

	// ImageName is the PostgreSQL container image, its tag is the version.
	// +optional
	ImageName string `json:"imageName,omitempty"`

	// +optional
	Storage PostgresStorage `json:"storage,omitempty"`

	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// +optional
	PostgreSQL PostgresConfiguration `json:"postgresql,omitempty"`
}

type PostgresStorage struct {
	// +optional
	Size string `json:"size,omitempty"`

	// +optional
	StorageClass string `json:"storageClass,omitempty"`
}

type PostgresConfiguration struct {
	// Parameters are the PostgreSQL configuration parameters.
	// +optional
	Parameters map[string]string `json:"parameters,omitempty"`
}

type PostgresClusterStatus struct {
	// Phase is the current phase of the cluster, e.g. "Cluster in healthy state".
	// +optional
	Phase string `json:"phase,omitempty"`

	// PhaseReason is the reason of the current phase.
	// +optional
	PhaseReason string `json:"phaseReason,omitempty"`

	// Instances is the number of instances created.
	// +optional
	Instances int `json:"instances,omitempty"`

	// ReadyInstances is the number of ready instances.
	// +optional
	ReadyInstances int `json:"readyInstances,omitempty"`

	// CurrentPrimary is the name of the primary instance.
	// +optional
	CurrentPrimary string `json:"currentPrimary,omitempty"`

	// Image is the PostgreSQL image the instances run.
	// +optional
	Image string `json:"image,omitempty"`

	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// DBaaSEndpoint is a load balancer address serving one CloudNative-PG service.
type DBaaSEndpoint struct {
	// Role is the CloudNative-PG service: rw, ro or r.
	Role string `json:"role,omitempty"`
	Host string `json:"host"`
	Port int64  `json:"port"`
	// External is true when Host is the EIP of the database instead of the load balancer VIP.
	External bool `json:"external"`
}

func DBaaSToResource(cluster PostgresClusterView, endpoints []DBaaSEndpoint) DBaaS {
	return DBaaS{
		Resource: Resource{
			ID:          cluster.Labels[spxId.SpxLabelResourceLocalID],
			EId:         cluster.Name,
			ProductName: cluster.Labels[spxId.SpxLabelResourceName],
			Gitops:      cluster.Labels[spxId.SpxLabelGitops],
		},
		Cluster:   cluster,
		Endpoints: endpoints,
	}
}
