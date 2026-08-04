package argo_test

import (
	"context"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/argo"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/argo/testhelper"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

const (
	testOrgId     = "3f2b9a10-1b3c-4c5d-8e7f-0a1b2c3d4e5f"
	testProjectId = "8c7d6e5f-4a3b-2c1d-9e8f-7a6b5c4d3e2f"
	testLocalId   = "1a2b3c4d-5e6f-4a8b-9c0d-1e2f3a4b5c6d"
)

// newClient wires a client over fake Argo CD and Kubernetes clientsets.
func newClient(t *testing.T, objects ...*v1alpha1.Application) *argo.Client {
	t.Helper()

	fake := testhelper.NewFakeClientset()
	for _, obj := range objects {
		if err := fake.Tracker().Add(obj); err != nil {
			t.Fatalf("failed to add object to tracker: %v", err)
		}
	}

	return argo.NewClient(fake.ArgoprojV1alpha1(), k8sfake.NewClientset(), argo.Options{
		AppProjectNamespace: "self-service-argocd",
	})
}

func createInfo(appName string, source argo.AppSource) argo.CreateAppInfo {
	return argo.CreateAppInfo{
		Metadata: spxId.Metadata{OrgId: testOrgId, ProjectId: testProjectId, ResourceLocalId: testLocalId},
		General:  argo.AppGeneral{AppName: appName, Destination: "spx-az1"},
		Spec:     argo.AppSpec{Source: source},
	}
}

// TestCreateAppSideEffects covers what the deleted createArgo HTTP handler used
// to do around the Application create: the namespace and AppProject must exist.
func TestCreateAppSideEffects(t *testing.T) {
	t.Parallel()

	c := newClient(t)
	namespace := c.Namespace(testProjectId)

	if err := c.CreateApp(context.Background(), createInfo("kaas-1", argo.AppSource{RepoURL: "https://example.test"})); err != nil {
		t.Fatalf("CreateApp() error = %v", err)
	}

	if _, err := c.Apps().Applications(namespace).Get(context.Background(), "kaas-1", metav1.GetOptions{}); err != nil {
		t.Errorf("Application was not created: %v", err)
	}
	// The AppProject name uses a literal "spx-" prefix, independent of spxPrefix.
	if _, err := c.Apps().AppProjects("self-service-argocd").Get(context.Background(), "spx-"+testProjectId, metav1.GetOptions{}); err != nil {
		t.Errorf("AppProject was not created: %v", err)
	}
}

// TestCreateAppSourceExclusivity pins the plugin-vs-helm rule: a named plugin
// wins, and helm is only set in its absence. Setting both would make Argo CD
// reject the Application.
func TestCreateAppSourceExclusivity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		source     argo.AppSource
		wantPlugin bool
		wantHelm   bool
	}{
		{
			name:       "named plugin wins over helm",
			source:     argo.AppSource{Plugin: v1alpha1.ApplicationSourcePlugin{Name: "uuidv5"}},
			wantPlugin: true,
		},
		{
			name:     "unnamed plugin falls back to helm",
			source:   argo.AppSource{Helm: v1alpha1.ApplicationSourceHelm{Values: "foo: bar"}},
			wantHelm: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := newClient(t)
			if err := c.CreateApp(context.Background(), createInfo("app-1", tt.source)); err != nil {
				t.Fatalf("CreateApp() error = %v", err)
			}

			got, err := c.Apps().Applications(c.Namespace(testProjectId)).Get(context.Background(), "app-1", metav1.GetOptions{})
			if err != nil {
				t.Fatalf("failed to read back application: %v", err)
			}

			if (got.Spec.Source.Plugin != nil) != tt.wantPlugin {
				t.Errorf("Plugin set = %v, want %v", got.Spec.Source.Plugin != nil, tt.wantPlugin)
			}
			if (got.Spec.Source.Helm != nil) != tt.wantHelm {
				t.Errorf("Helm set = %v, want %v", got.Spec.Source.Helm != nil, tt.wantHelm)
			}
		})
	}
}

// TestUpdateAppGitopsGuard checks that applications owned by a GitOps
// repository are refused, and that the sentinel is the one callers match on to
// return a meaningful status.
func TestUpdateAppGitopsGuard(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		labels  map[string]string
		wantErr error
	}{
		{
			name:    "gitops-managed app is refused",
			labels:  map[string]string{spxId.SpxLabelGitops: "true"},
			wantErr: argo.ErrGitopsManaged,
		},
		{
			name:   "regular app is updated",
			labels: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			namespace := spxId.ToSPXID(testProjectId)
			c := newClient(t, &v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{Name: "app-1", Namespace: namespace, Labels: tt.labels},
			})

			err := c.UpdateApp(context.Background(), "app-1", namespace, argo.UpdateAppInfo{
				Source: argo.AppSource{RepoURL: "https://example.test"},
			})
			if err != tt.wantErr {
				t.Fatalf("UpdateApp() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}

			got, err := c.Apps().Applications(namespace).Get(context.Background(), "app-1", metav1.GetOptions{})
			if err != nil {
				t.Fatalf("failed to read back application: %v", err)
			}
			if got.Spec.Source.RepoURL != "https://example.test" {
				t.Errorf("RepoURL = %q, want %q", got.Spec.Source.RepoURL, "https://example.test")
			}
		})
	}
}

// TestGetAppMissing is a regression test: the pre-merge implementation
// dereferenced the nil result on the error path, which now panics the API
// process rather than a single-purpose controller.
func TestGetAppMissing(t *testing.T) {
	t.Parallel()

	c := newClient(t)

	got, err := c.GetApp(context.Background(), "absent", c.Namespace(testProjectId))
	if err == nil {
		t.Fatal("GetApp() error = nil, want NotFound")
	}
	if got.Name != "" {
		t.Errorf("GetApp() = %+v, want zero value", got)
	}
}
