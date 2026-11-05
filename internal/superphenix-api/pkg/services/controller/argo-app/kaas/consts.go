package kaas

import (
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/super-phenix/superphenix/pkg/utils/validation"
)

var (
	CpNetworkPolicies      = []string{"default", "none"}
	WorkersNetworkPolicies = []string{"default", "strict", "none"}
)

const (
	KaasPrefix     = "kaas"
	quantityFormat = "%dGi"
	maxnodeGroup   = 5

	nodeRoleLabelPrefix = "node-role.kubernetes.io/"

	// Longest node group name: the chart renders "<cluster uuid>-<name>-<kube version>-<template version>"
	// as a label value, limited to 63 characters.
	maxNodeGroupNameLength = 14

	minReplicas = 1
	maxReplicas = validation.MaxComputeValue

	// Largest boot disk or datastore size (GiB) accepted for a KaaS cluster.
	maxStorageGiB = validation.MaxComputeValue

	// Disaster recovery (dedicated control plane datastore) requires Kubernetes >= 1.35.
	dataStoreMinMajor = 1
	dataStoreMinMinor = 35

	// Default dedicated-datastore size (GiB) when DR is enabled without an explicit size.
	defaultDataStoreStorageGi = 8
)

var ignoreDifferences = v1alpha1.IgnoreDifferences{
	v1alpha1.ResourceIgnoreDifferences{
		Group: "batch",
		Kind:  "Job",
		JSONPointers: []string{
			"/metadata/labels/app.kubernetes.io~1version",
			"/metadata/labels/helm.sh~1chart",
			"/metadata/labels/superphenix.net~1gitops",
			"/spec",
		},
	},
	v1alpha1.ResourceIgnoreDifferences{
		Group: "infrastructure.cluster.x-k8s.io",
		Kind:  "KubevirtMachineTemplate",
		JSONPointers: []string{
			"/metadata/labels/app.kubernetes.io~1version",
			"/metadata/labels/helm.sh~1chart",
			"/metadata/labels/superphenix.net~1gitops",
			"/spec",
		},
	},
}
