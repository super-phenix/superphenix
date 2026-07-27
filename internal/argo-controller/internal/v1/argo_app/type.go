package argoApp

import "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"

type AppSource struct {
	RepoURL        string                           `json:"repoUrl"`
	TargetRevision string                           `json:"targetRevision"`
	Chart          string                           `json:"chart,omitempty"`
	Path           string                           `json:"path,omitempty"`
	Helm           v1alpha1.ApplicationSourceHelm   `json:"helm"`
	Plugin         v1alpha1.ApplicationSourcePlugin `json:"plugin"`
}
