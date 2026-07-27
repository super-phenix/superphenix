package v1alpha1

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ConditionTypeReachable represents the connectivity status to the remote cluster.
	ConditionTypeReachable = "Reachable"

	// ReasonConnectionSuccess is used when the connection to the remote cluster is successful.
	ReasonConnectionSuccess = "ConnectionSuccess"
	// ReasonConnectionFailed is used when the connection to the remote cluster fails.
	ReasonConnectionFailed = "ConnectionFailed"
	// ReasonSecretNotFound is used when the secret containing the connection credentials is not found.
	ReasonSecretNotFound = "SecretNotFound"
	// ReasonInvalidSecret is used when the secret containing the connection credentials is invalid.
	ReasonInvalidSecret = "InvalidSecret"
	// ReasonConnectionConfigError is used when the connection configuration is invalid or missing.
	ReasonConnectionConfigError = "ConnectionConfigError"

	// ReasonInvalidVersion is used when the requested version change is invalid.
	ReasonInvalidVersion = "InvalidVersion"

	// ConditionTypeCompatibleVersion represents the compatibility status of the cluster version with the management version.
	ConditionTypeCompatibleVersion = "CompatibleVersion"
	// ReasonCompatibleVersion is used when the cluster version is compatible with the management version.
	ReasonCompatibleVersion = "CompatibleVersion"
	// ReasonIncompatibleVersion is used when the cluster version is not supported by the management version.
	ReasonIncompatibleVersion = "IncompatibleVersion"

	// ConditionTypeArgoCDSynced represents the synchronization status of the cluster's ArgoCD Application.
	ConditionTypeArgoCDSynced = "ArgoCDSynced"
	// ReasonArgoCDSynced is used when the ArgoCD Application is synced.
	ReasonArgoCDSynced = "ArgoCDSynced"
	// ReasonArgoCDOutOfSync is used when the ArgoCD Application is out of sync.
	ReasonArgoCDOutOfSync = "ArgoCDOutOfSync"
	// ReasonArgoCDSyncFailed is used when the ArgoCD Application sync failed.
	ReasonArgoCDSyncFailed = "ArgoCDSyncFailed"
	// ReasonArgoCDUnknown is used when the ArgoCD Application status is unknown.
	ReasonArgoCDUnknown = "ArgoCDUnknown"
	// ReasonArgoCDSyncing is used when the ArgoCD Application is syncing.
	ReasonArgoCDSyncing = "ArgoCDSyncing"

	// ConditionTypeReady represents the status when the cluster is fully operational.
	ConditionTypeReady = "Ready"
	// ReasonReconcileSuccess is used when the cluster is fully reconciled.
	ReasonReconcileSuccess = "ReconcileSuccess"
	// ReasonHealthCheckFailed is used when the health check fails.
	ReasonHealthCheckFailed = "HealthCheckFailed"
	// ReasonApplicationReconcileFailed is used when the ArgoCD application reconciliation fails.
	ReasonApplicationReconcileFailed = "ApplicationReconcileFailed"
	// ReasonAppProjectReconcileFailed is used when the ArgoCD project reconciliation fails.
	ReasonAppProjectReconcileFailed = "AppProjectReconcileFailed"
	// ReasonConfigMapReconcileFailed is used when the clusters ConfigMap reconciliation fails.
	ReasonConfigMapReconcileFailed = "ConfigMapReconcileFailed"
	// ReasonConfigMapCleanupFailed is used when removing the cluster from the clusters ConfigMap fails.
	ReasonConfigMapCleanupFailed = "ConfigMapCleanupFailed"

	// ConditionTypePaused represents the status when the cluster synchronization is paused.
	ConditionTypePaused = "Paused"
	// ReasonPaused is used when the cluster synchronization is paused.
	ReasonPaused = "Paused"
	// ReasonResumed is used when the cluster synchronization is resumed.
	ReasonResumed = "Resumed"

	// ReasonDeleting is used when the cluster is being deleted.
	ReasonDeleting = "Deleting"
)

// DeploymentTopology defines whether the cluster is hyperconverged or decoupled.
// +kubebuilder:validation:Enum=Hyperconverged;Decoupled
type DeploymentTopology string

const (
	// DeploymentTopologyHyperconverged - Storage and virtualization run on the same cluster.
	DeploymentTopologyHyperconverged DeploymentTopology = "Hyperconverged"

	// DeploymentTopologyDecoupled - Storage and virtualization run on separate clusters.
	DeploymentTopologyDecoupled DeploymentTopology = "Decoupled"
)

// ClusterType defines the type of cluster when in Decoupled mode.
// +kubebuilder:validation:Enum=Storage;Virtualization
type ClusterType string

const (
	// ClusterTypeStorage - Dedicated storage cluster.
	ClusterTypeStorage ClusterType = "Storage"

	// ClusterTypeVirtualization - Dedicated virtualization/hypervisor cluster.
	ClusterTypeVirtualization ClusterType = "Virtualization"
)

// TalosManagementMode defines the management mode for the Talos cluster.
// +kubebuilder:validation:Enum=Unmanaged;Import;Full
type TalosManagementMode string

const (
	// TalosManagementUnmanaged - The Talos cluster is not managed by the operator. This requires an externally managed installation and configuration of Talos.
	TalosManagementUnmanaged TalosManagementMode = "Unmanaged"

	// TalosManagementImport - The operator imports an already installed Talos cluster and manages its configuration.
	TalosManagementImport TalosManagementMode = "Import"

	// TalosManagementFull - The Talos cluster is fully installed and configured by the operator.
	TalosManagementFull TalosManagementMode = "Full"
)

// ClusterSpec defines the desired state of Cluster.
type ClusterSpec struct {
	// DeploymentTopology defines whether the cluster is hyperconverged or decoupled.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Hyperconverged;Decoupled
	DeploymentTopology DeploymentTopology `json:"deploymentTopology,omitempty"`

	// Type specifies the cluster type (Storage or Virtualization) when DeploymentTopology is Decoupled.
	// This field can only be set when DeploymentTopology is Decoupled and is ignored otherwise.
	// +optional
	Type *ClusterType `json:"type,omitempty"`

	// TalosManagementMode specifies how Talos configuration should be managed.
	// +optional
	// +kubebuilder:default=Unmanaged
	// +kubebuilder:validation:Enum=Unmanaged;Import;Full
	TalosManagementMode TalosManagementMode `json:"talosManagementMode,omitempty"`

	// TalosManagerConfiguration is a YAML dict of unknown values that will be passed to the talos-manager chart.
	// +optional
	TalosManagerConfiguration *apiextensionsv1.JSON `json:"talosManagerConfiguration,omitempty"`

	// Region is the geographic region where this cluster is located.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Region string `json:"region"`

	// AvailabilityZone is the availability zone identifier within the region.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	AvailabilityZone string `json:"availabilityZone"`

	// SystemConfiguration is a YAML dict of unknown values that will be passed to the system configuration chart.
	// +optional
	SystemConfiguration *apiextensionsv1.JSON `json:"systemConfiguration,omitempty"`

	// RepoURL is the URL of the repository where the Superphenix system chart is located.
	// If not specified, the default value from the controller configuration is used.
	// +optional
	RepoURL string `json:"repoURL,omitempty"`

	// ChartName is the name of the Superphenix system chart.
	// If not specified, the default value from the controller configuration is used.
	// +optional
	ChartName string `json:"chartName,omitempty"`

	// Version is the Superphenix version for this cluster.
	// It must follow semantic versioning.
	// If not specified, the default value from the controller configuration is used.
	// +optional
	// +kubebuilder:validation:Pattern=`^v?([0-9]+)(\.[0-9]+)?(\.[0-9]+)?(-([0-9A-Za-z\-.]+))?(\+([0-9A-Za-z\-.]+))?$`
	Version string `json:"version,omitempty"`

	// Connection defines the parameters to connect to the remote cluster.
	// +kubebuilder:validation:Required
	Connection *ClusterConnectionSpec `json:"connection"`

	// PauseSync allows to temporarily pause the synchronization of the Superphenix stack on this cluster.
	// When set to true, an ArgoCD sync window is added to the cluster's project to prevent any automated or manual sync.
	// +optional
	// +kubebuilder:default=false
	PauseSync bool `json:"pauseSync,omitempty"`

	// Manual allows to disable the autosync of all applications on this cluster.
	// When set to true, "forceManual" is passed to the Superphenix system chart.
	// +optional
	// +kubebuilder:default=false
	Manual bool `json:"manual,omitempty"`

	// CleanupOnDeletion allows the cleanup of the cluster when it gets deleted.
	// When set to true, deleting the cluster resource will automatically propagate the deletion
	// to the target, with every application getting cascade deleted. This option is dangerous
	// and should usually be set to true only before a deletion is planned to avoid any accident.
	// +optional
	// +kubebuilder:default=false
	CleanupOnDeletion bool `json:"cleanupOnDeletion,omitempty"`
}

// ConnectionMode defines how the operator connects to the cluster.
// +kubebuilder:validation:Enum=Remote;Local
type ConnectionMode string

const (
	// ConnectionModeRemote - Connect to a remote cluster via URL and Secret.
	ConnectionModeRemote ConnectionMode = "Remote"

	// ConnectionModeLocal - Connect to the local cluster (the one where the operator is running).
	ConnectionModeLocal ConnectionMode = "Local"
)

// ClusterConnectionSpec defines the parameters to connect to the remote cluster.
type ClusterConnectionSpec struct {
	// Mode specifies the connection mode (Remote or Local).
	// +kubebuilder:validation:Required
	// +kubebuilder:default=Local
	Mode ConnectionMode `json:"mode"`

	// URL is the address of the remote cluster API server.
	// This is required when Mode is Remote and ignored when Mode is Local.
	// +optional
	URL string `json:"url,omitempty"`

	// SecretRef is a reference to a secret containing the connection credentials.
	// This is required when Mode is Remote and ignored when Mode is Local.
	// +optional
	SecretRef *SecretReference `json:"secretRef,omitempty"`
}

// SecretReference defines a reference to a Secret.
type SecretReference struct {
	// Name of the secret
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace of the secret. If empty, the namespace of the Cluster is used.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// ClusterStatus defines the observed state of Cluster.
type ClusterStatus struct {
	// Phase represents the current phase of the cluster lifecycle.
	// +kubebuilder:validation:Enum=Deployed;Deploying;OutOfSync;Error;Paused;Unknown
	// +optional
	Phase string `json:"phase,omitempty"`

	// SuperphenixVersion is the actual Superphenix version currently running on the cluster.
	// +optional
	SuperphenixVersion string `json:"superphenixVersion,omitempty"`

	// KubernetesVersion is the version of the Kubernetes cluster.
	// +optional
	KubernetesVersion string `json:"kubernetesVersion,omitempty"`

	// NodeCount is the number of nodes in the cluster.
	// +optional
	NodeCount int `json:"nodeCount,omitempty"`

	// Conditions represent the current state of the Cluster resource.
	// Standard condition types include:
	// - "Ready": the cluster is fully operational
	// - "Progressing": the cluster is being provisioned or updated
	// - "Degraded": the cluster has encountered issues
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration reflects the generation of the most recently observed Cluster.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// LastSync is the last time a sync was performed on the cluster.
	// +optional
	LastSync *metav1.Time `json:"lastSync,omitempty"`

	// Apps reports the state of each application deployed by the cluster's root
	// app-of-apps, keyed by application name.
	// +optional
	Apps map[string]ClusterApp `json:"apps,omitempty"`
}

// ClusterApp reports the observed state of a single application belonging to the
// cluster's app-of-apps tree.
type ClusterApp struct {
	// Name is the application name.
	Name string `json:"name"`

	// Status is the health status of the Application (e.g. Healthy, Degraded, Progressing).
	// +optional
	Status string `json:"status,omitempty"`

	// LastRefresh is the last time ArgoCD reconciled the Application against its source.
	// +optional
	LastRefresh *metav1.Time `json:"lastRefresh,omitempty"`

	// LastSync is the last time a sync operation on the Application completed.
	// +optional
	LastSync *metav1.Time `json:"lastSync,omitempty"`

	// Version is the target revision of the Application source.
	// +optional
	Version string `json:"version,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Topology",type=string,JSONPath=`.spec.deploymentTopology`
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="Region",type=string,JSONPath=`.spec.region`
// +kubebuilder:printcolumn:name="AZ",type=string,JSONPath=`.spec.availabilityZone`
// +kubebuilder:printcolumn:name="SPX version",type=string,JSONPath=`.status.superphenixVersion`
// +kubebuilder:printcolumn:name="K8S version",type=string,JSONPath=`.status.kubernetesVersion`
// +kubebuilder:printcolumn:name="Nodes",type=integer,JSONPath=`.status.nodeCount`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Cluster represents a Superphenix cluster deployment.
// A cluster is an availability zone (AZ) where the full Superphenix stack is deployed.
type Cluster struct {
	metav1.TypeMeta `json:",inline"`

	// Metadata is a standard object metadata.
	// +required
	metav1.ObjectMeta `json:"metadata"`

	// Spec defines the desired state of Cluster.
	// +required
	Spec ClusterSpec `json:"spec"`

	// Status defines the observed state of Cluster.
	// +optional
	Status ClusterStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ClusterList contains a list of Cluster
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Cluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Cluster{}, &ClusterList{})
}
