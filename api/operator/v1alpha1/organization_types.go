package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OrganizationSpec defines the desired state of Organization.
// Organizations are onboarded via GitOps and mirror organizations created
// in the Superphenix console.
type OrganizationSpec struct {
	// OrganizationID is the Superphenix organization UUID.
	// It must be a valid UUIDv4.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`
	OrganizationID string `json:"organizationID"`

	// NameOverride allows overriding the name used in labels.
	// It must be a valid Kubernetes label value (RFC 1123/6399).
	// +optional
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?$`
	NameOverride string `json:"nameOverride,omitempty"`
}

// OrganizationStatus defines the observed state of Organization.
type OrganizationStatus struct {
	// Conditions represent the current state of the Organization resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration reflects the generation of the most recently observed Organization.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="OrganizationID",type=string,JSONPath=`.spec.organizationID`
// +kubebuilder:printcolumn:name="Override",type=string,JSONPath=`.spec.nameOverride`
// +kubebuilder:printcolumn:name="Bound",type=string,JSONPath=`.status.conditions[?(@.type=="Bound")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Organization represents a Superphenix organization onboarded via GitOps.
type Organization struct {
	metav1.TypeMeta `json:",inline"`

	// Metadata is a standard object metadata.
	// +required
	metav1.ObjectMeta `json:"metadata"`

	// Spec defines the desired state of Organization.
	// +optional
	Spec OrganizationSpec `json:"spec,omitzero"`

	// Status defines the observed state of Organization.
	// +optional
	Status OrganizationStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// OrganizationList contains a list of Organization.
type OrganizationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Organization `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Organization{}, &OrganizationList{})
}
