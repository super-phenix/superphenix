package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ConditionTypeBound represents whether the resource is bound to a record in the Superphenix Database.
	ConditionTypeBound = "Bound"

	// ReasonBoundFound is used when the resource is found in the Superphenix Database.
	ReasonBoundFound = "Found"
	// ReasonBoundCreated is used when the resource is created in the Superphenix Database.
	ReasonBoundCreated = "Created"
	// ReasonBoundFailed is used when the resource binding fails.
	ReasonBoundFailed = "Failed"

	// ConditionTypeAvailabilityZonesReady represents whether the availability zones are valid and ready.
	ConditionTypeAvailabilityZonesReady = "AvailabilityZonesReady"

	// ReasonAvailabilityZonesValid is used when all availability zones are valid.
	ReasonAvailabilityZonesValid = "Valid"
	// ReasonAvailabilityZonesInvalid is used when one or more availability zones are invalid.
	ReasonAvailabilityZonesInvalid = "Invalid"
)

// ProjectSpec defines the desired state of Project.
type ProjectSpec struct {
	// ProjectID is the Superphenix project UUID.
	// It must be a valid UUIDv4.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`
	ProjectID string `json:"projectID"`

	// NameOverride allows overriding the name used in labels.
	// It must be a valid Kubernetes label value (RFC 1123/6399).
	// +optional
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?$`
	NameOverride string `json:"nameOverride,omitempty"`

	// OrganizationRef references the Organization resource this project belongs to.
	// +kubebuilder:validation:Required
	OrganizationRef OrganizationReference `json:"organizationRef"`

	// AvailabilityZones references the Cluster resources where this project can exist.
	// +optional
	AvailabilityZones []AvailabilityZoneReference `json:"availabilityZones,omitempty"`

	// GitOps defines the GitOps application configuration for this project.
	// +optional
	GitOps *ProjectGitOpsSpec `json:"gitops,omitempty"`
}

// ProjectGitOpsSpec defines the GitOps application overrides for a project.
type ProjectGitOpsSpec struct {
	// ManifestLocation defines where the GitOps manifests are located.
	// +optional
	ManifestLocation *ProjectManifestLocationSpec `json:"manifestLocation,omitempty"`
}

// ProjectManifestLocationSpec defines the location of GitOps manifests.
type ProjectManifestLocationSpec struct {
	// RepoURL overrides the default repository URL.
	// +optional
	RepoURL string `json:"repoURL,omitempty"`

	// Path overrides the default path.
	// +optional
	Path string `json:"path,omitempty"`

	// TargetRevision overrides the default target revision.
	// +optional
	TargetRevision string `json:"targetRevision,omitempty"`
}

// AvailabilityZoneReference defines a reference to a Cluster resource.
type AvailabilityZoneReference struct {
	// Name of the Cluster.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace of the Cluster. If empty, the namespace of the Project is used.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// OrganizationReference defines a reference to an Organization resource.
type OrganizationReference struct {
	// Name of the Organization.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace of the Organization. If empty, the namespace of the Project is used.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// ProjectStatus defines the observed state of Project.
type ProjectStatus struct {
	// Conditions represent the current state of the Project resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration reflects the generation of the most recently observed Project.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// AvailableZones lists the names of availability zones where the project is available.
	// Format: "region-availabilityZone"
	// +optional
	AvailableZones []string `json:"availableZones,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Organization",type=string,JSONPath=`.spec.organizationRef.name`
// +kubebuilder:printcolumn:name="ProjectID",type=string,JSONPath=`.spec.projectID`
// +kubebuilder:printcolumn:name="Override",type=string,JSONPath=`.spec.nameOverride`
// +kubebuilder:printcolumn:name="Bound",type=string,JSONPath=`.status.conditions[?(@.type=="Bound")].status`
// +kubebuilder:printcolumn:name="Available AZs",type=string,JSONPath=`.status.availableZones`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Project represents a Superphenix project onboarded via GitOps.
type Project struct {
	metav1.TypeMeta `json:",inline"`

	// Metadata is a standard object metadata.
	// +required
	metav1.ObjectMeta `json:"metadata"`

	// Spec defines the desired state of Project.
	// +required
	Spec ProjectSpec `json:"spec"`

	// Status defines the observed state of Project.
	// +optional
	Status ProjectStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ProjectList contains a list of Project.
type ProjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Project `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Project{}, &ProjectList{})
}
