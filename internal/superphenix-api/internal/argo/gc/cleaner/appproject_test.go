package cleaner_test

import (
	"context"
	"testing"
	"time"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/argo/gc/cleaner"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/argo/testhelper"

	argocdv1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/rs/zerolog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
)

func newAppProject(name, namespace string, labels map[string]string) argocdv1alpha1.AppProject {
	return argocdv1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
	}
}

func TestAppProjectCleanerClean(t *testing.T) {
	t.Parallel()

	labelMarkKey := "superphenix.net/markedForDeletion"
	appProjectNamespace := "superphenix-system"
	pastTimestamp := time.Now().Add(-1 * time.Hour).Format(cleaner.TimestampFormat)
	futureTimestamp := time.Now().Add(1 * time.Hour).Format(cleaner.TimestampFormat)

	tests := []struct {
		name          string
		projects      []argocdv1alpha1.AppProject
		debug         bool
		wantErr       bool
		wantRemaining int
	}{
		{
			name:          "no marked app projects",
			projects:      []argocdv1alpha1.AppProject{},
			debug:         false,
			wantErr:       false,
			wantRemaining: 0,
		},
		{
			name: "deletes app project with past timestamp",
			projects: []argocdv1alpha1.AppProject{
				newAppProject("proj1", appProjectNamespace, map[string]string{
					labelMarkKey: pastTimestamp,
				}),
			},
			debug:         false,
			wantErr:       false,
			wantRemaining: 0,
		},
		{
			name: "does not delete app project with future timestamp",
			projects: []argocdv1alpha1.AppProject{
				newAppProject("proj1", appProjectNamespace, map[string]string{
					labelMarkKey: futureTimestamp,
				}),
			},
			debug:         false,
			wantErr:       false,
			wantRemaining: 1,
		},
		{
			name: "debug mode does not delete app projects",
			projects: []argocdv1alpha1.AppProject{
				newAppProject("proj1", appProjectNamespace, map[string]string{
					labelMarkKey: pastTimestamp,
				}),
			},
			debug:         true,
			wantErr:       false,
			wantRemaining: 1,
		},
		{
			name: "mixed timestamps deletes only expired",
			projects: []argocdv1alpha1.AppProject{
				newAppProject("proj-expired", appProjectNamespace, map[string]string{
					labelMarkKey: pastTimestamp,
				}),
				newAppProject("proj-not-expired", appProjectNamespace, map[string]string{
					labelMarkKey: futureTimestamp,
				}),
			},
			debug:         false,
			wantErr:       false,
			wantRemaining: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			clientset := testhelper.NewFakeClientset()
			for i := range tt.projects {
				if err := clientset.Tracker().Add(&tt.projects[i]); err != nil {
					t.Fatalf("failed to add object to tracker: %v", err)
				}
			}
			apps := clientset.ArgoprojV1alpha1()

			c := cleaner.NewAppProject(apps, cleaner.Options{
				LabelMarkKey:        labelMarkKey,
				Debug:               tt.debug,
				AppProjectNamespace: appProjectNamespace,
			}, zerolog.Nop())

			err := c.Clean(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("Clean() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			remaining, err := apps.AppProjects(appProjectNamespace).List(context.Background(), metav1.ListOptions{})
			if err != nil {
				t.Fatalf("failed to list remaining app projects: %v", err)
			}
			if len(remaining.Items) != tt.wantRemaining {
				t.Errorf("Clean() remaining = %d, want %d", len(remaining.Items), tt.wantRemaining)
			}
		})
	}
}

func TestAppProjectCleanerMark(t *testing.T) {
	t.Parallel()

	labelMarkKey := "superphenix.net/markedForDeletion"
	appProjectNamespace := "superphenix-system"
	namespace := "test-project"
	timestamp := time.Now().Add(48 * time.Hour).Format(cleaner.TimestampFormat)

	tests := []struct {
		name      string
		projects  []argocdv1alpha1.AppProject
		namespace string
		wantErr   bool
		wantCount int
	}{
		{
			name:      "no app projects to mark",
			projects:  []argocdv1alpha1.AppProject{},
			namespace: namespace,
			wantErr:   false,
			wantCount: 0,
		},
		{
			name: "marks app project with matching project label",
			projects: []argocdv1alpha1.AppProject{
				newAppProject("proj1", appProjectNamespace, map[string]string{
					spxId.SpxLabelProjectID: namespace,
				}),
			},
			namespace: namespace,
			wantErr:   false,
			wantCount: 1,
		},
		{
			name: "does not mark app project with different project label",
			projects: []argocdv1alpha1.AppProject{
				newAppProject("proj1", appProjectNamespace, map[string]string{
					spxId.SpxLabelProjectID: "other-project",
				}),
			},
			namespace: namespace,
			wantErr:   false,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			clientset := testhelper.NewFakeClientset()
			for i := range tt.projects {
				if err := clientset.Tracker().Add(&tt.projects[i]); err != nil {
					t.Fatalf("failed to add object to tracker: %v", err)
				}
			}
			apps := clientset.ArgoprojV1alpha1()

			c := cleaner.NewAppProject(apps, cleaner.Options{
				LabelMarkKey:        labelMarkKey,
				AppProjectNamespace: appProjectNamespace,
			}, zerolog.Nop())

			err := c.Mark(context.Background(), tt.namespace, timestamp)
			if (err != nil) != tt.wantErr {
				t.Errorf("Mark() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			projects, err := apps.AppProjects(appProjectNamespace).List(context.Background(), metav1.ListOptions{})
			if err != nil {
				t.Fatalf("failed to list app projects: %v", err)
			}

			markedCount := 0
			for _, proj := range projects.Items {
				if proj.GetLabels()[labelMarkKey] == timestamp {
					markedCount++
				}
			}
			if markedCount != tt.wantCount {
				t.Errorf("Mark() marked = %d, want %d", markedCount, tt.wantCount)
			}
		})
	}
}

func TestAppProjectCleanerErrorMessage(t *testing.T) {
	t.Parallel()

	c := cleaner.NewAppProject(nil, cleaner.Options{}, zerolog.Nop())
	want := "Something went wrong during ArgoApp Project cleaning."
	if got := c.ErrorMessage(); got != want {
		t.Errorf("ErrorMessage() = %q, want %q", got, want)
	}
}
