package dbaas

import (
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/argo/view"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func testMetadata(t *testing.T) spxId.Metadata {
	t.Helper()
	m := spxId.Metadata{}
	if err := m.GenerateMetadata(testProjectID, "0a9b8c7d-6e5f-4a3b-8c2d-1e0f9a8b7c6d", "db-1"); err != nil {
		t.Fatalf("GenerateMetadata() error: %v", err)
	}
	return m
}

func pluginEnv(env v1alpha1.Env, name string) string {
	for _, e := range env {
		if e != nil && e.Name == name {
			return e.Value
		}
	}
	return ""
}

func appView(env v1alpha1.Env) view.AppView {
	var app view.AppView
	app.Spec.Source = &view.ApplicationSource{Plugin: &v1alpha1.ApplicationSourcePlugin{Env: env}}
	return app
}
