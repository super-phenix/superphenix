package utils

import (
	"slices"
	"strings"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
)

const (
	CustomLabelPrefix = "user.superphenix.net/"
)

var (
	allowedLabelKeyList = []string{
		"velero.io/backup-name",
		"velero.io/restore-name",
		"app.kubernetes.io/name",
	}

	excludedLabels = []string{
		"superphenix.net/ignoreNetworkPolicies",
		"superphenix.net/workloadClass",
	}

	allowedAnnotationKeyList = []string{
		"replication.storage.openshift.io/volume-replication-name",
	}

	excludedAnnotationKeyList = []string{
		"superphenix.net/allowedProjects",
	}
)

func FilterLabels(labels map[string]string) map[string]string {
	filteredLabels := make(map[string]string)
	for k, v := range labels {
		// Keep only Spx Labels
		if strings.HasPrefix(k, spxId.SpxLabelPrefix) ||
			strings.HasPrefix(k, CustomLabelPrefix) ||
			slices.Contains(allowedLabelKeyList, k) {
			// Remove excluded labels
			if !slices.Contains(excludedLabels, k) {
				filteredLabels[k] = v
			}

		}
	}

	return filteredLabels
}

func FilterAnnotations(annotations map[string]string) map[string]string {
	filteredAnnotations := make(map[string]string)
	for k, v := range annotations {
		if strings.HasPrefix(k, spxId.SpxLabelPrefix) ||
			slices.Contains(allowedAnnotationKeyList, k) {
			if !slices.Contains(excludedAnnotationKeyList, k) {
				filteredAnnotations[k] = v
			}
		}
	}

	return filteredAnnotations
}
