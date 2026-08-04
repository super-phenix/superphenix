package argoApp

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/argo"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/argo/view"
)

// Client is the subset of the Argo adapter the controller services depend on.
// Declared here, in the consumer, so both kaas and baas can be faked in tests.
// It is always backed by a live cluster connection: app.ProvideArgo is fatal
// when one cannot be established, so handlers never see a nil client.
type Client interface {
	CreateApp(ctx context.Context, info argo.CreateAppInfo) error
	UpdateApp(ctx context.Context, name, namespace string, info argo.UpdateAppInfo) error
	GetApp(ctx context.Context, name, namespace string) (view.AppView, error)
	DeleteApp(ctx context.Context, name, namespace string) error
	Namespace(projectId string) string
}

var _ Client = (*argo.Client)(nil)
