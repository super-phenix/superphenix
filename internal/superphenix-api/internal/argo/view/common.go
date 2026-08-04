package view

import (
	"slices"
	"strings"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var excludedLabels = []string{
	"superphenix.net/ignoreNetworkPolicies",
	"superphenix.net/workloadClass",
}

type AbstractObject struct {
	k8smetav1.ObjectMeta `json:"metadata"`
}

type ObjectMeta struct {
	Name              string                     `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	GenerateName      string                     `json:"generateName,omitempty" protobuf:"bytes,2,opt,name=generateName"`
	Namespace         string                     `json:"namespace,omitempty" protobuf:"bytes,3,opt,name=namespace"`
	CreationTimestamp k8smetav1.Time             `json:"creationTimestamp,omitempty" protobuf:"bytes,8,opt,name=creationTimestamp"`
	Labels            map[string]string          `json:"labels,omitempty" protobuf:"bytes,11,rep,name=labels"`
	OwnerReferences   []k8smetav1.OwnerReference `json:"ownerReferences,omitempty" patchStrategy:"merge" patchMergeKey:"uid" protobuf:"bytes,13,rep,name=ownerReferences"`
}

type OwnerReference struct {
	APIVersion         string    `json:"apiVersion" protobuf:"bytes,5,opt,name=apiVersion"`
	Kind               string    `json:"kind" protobuf:"bytes,1,opt,name=kind"`
	Name               string    `json:"name" protobuf:"bytes,3,opt,name=name"`
	UID                types.UID `json:"uid" protobuf:"bytes,4,opt,name=uid,casttype=k8s.io/apimachinery/pkg/types.UID"`
	Controller         *bool     `json:"controller,omitempty" protobuf:"varint,6,opt,name=controller"`
	BlockOwnerDeletion *bool     `json:"blockOwnerDeletion,omitempty" protobuf:"varint,7,opt,name=blockOwnerDeletion"`
}

func filterLabels(labels map[string]string) map[string]string {
	filteredLabels := make(map[string]string)
	for k, v := range labels {
		// Keep only Spx Labels
		if strings.HasPrefix(k, spxId.SpxLabelPrefix) {
			// Remove excluded labels
			if !slices.Contains(excludedLabels, k) {
				filteredLabels[k] = v
			}

		}
	}

	return filteredLabels
}
