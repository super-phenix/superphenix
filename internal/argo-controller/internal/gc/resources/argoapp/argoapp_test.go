package argoapp

import (
	"github.com/super-phenix/superphenix/internal/argo-controller/internal/gc/utils"
	"github.com/super-phenix/superphenix/internal/argo-controller/pkg/config"
	"context"
	"testing"
	"time"

	"github.com/super-phenix/superphenix/internal/argo-controller/internal/gc/resources/testhelper"

	argocdv1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/rs/zerolog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
)

func newApplication(name, namespace string, labels map[string]string) argocdv1alpha1.Application {
	return argocdv1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
	}
}

func TestClean(t *testing.T) {
	labelMarkKey := "superphenix.net/markedForDeletion"
	pastTimestamp := time.Now().Add(-1 * time.Hour).Format(utils.TimestampFormat)
	futureTimestamp := time.Now().Add(1 * time.Hour).Format(utils.TimestampFormat)

	tests := []struct {
		name          string
		apps          []argocdv1alpha1.Application
		debug         bool
		wantErr       bool
		wantRemaining int
	}{
		{
			name:          "no marked applications",
			apps:          []argocdv1alpha1.Application{},
			debug:         false,
			wantErr:       false,
			wantRemaining: 0,
		},
		{
			name: "deletes application with past timestamp",
			apps: []argocdv1alpha1.Application{
				newApplication("app1", "ns1", map[string]string{
					labelMarkKey: pastTimestamp,
				}),
			},
			debug:         false,
			wantErr:       false,
			wantRemaining: 0,
		},
		{
			name: "does not delete application with future timestamp",
			apps: []argocdv1alpha1.Application{
				newApplication("app1", "ns1", map[string]string{
					labelMarkKey: futureTimestamp,
				}),
			},
			debug:         false,
			wantErr:       false,
			wantRemaining: 1,
		},
		{
			name: "debug mode does not delete applications",
			apps: []argocdv1alpha1.Application{
				newApplication("app1", "ns1", map[string]string{
					labelMarkKey: pastTimestamp,
				}),
			},
			debug:         true,
			wantErr:       false,
			wantRemaining: 1,
		},
		{
			name: "mixed timestamps deletes only expired",
			apps: []argocdv1alpha1.Application{
				newApplication("app-expired", "ns1", map[string]string{
					labelMarkKey: pastTimestamp,
				}),
				newApplication("app-not-expired", "ns1", map[string]string{
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
			config.Global.GarbageCollection.LabelMarkKey = labelMarkKey
			config.Global.GarbageCollection.Debug = tt.debug

			clientset := testhelper.NewFakeClientset()
			for i := range tt.apps {
				if err := clientset.Tracker().Add(&tt.apps[i]); err != nil {
					t.Fatalf("failed to add object to tracker: %v", err)
				}
			}
			config.ArgoClient = clientset.ArgoprojV1alpha1()

			c := &Cleaner{
				ResourceType: "Argo Application",
				Logger:       zerolog.Nop(),
			}

			err := c.Clean(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("Clean() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			remaining, err := config.ArgoClient.Applications("").List(context.Background(), metav1.ListOptions{})
			if err != nil {
				t.Fatalf("failed to list remaining applications: %v", err)
			}
			if len(remaining.Items) != tt.wantRemaining {
				t.Errorf("Clean() remaining = %d, want %d", len(remaining.Items), tt.wantRemaining)
			}
		})
	}
}

func TestMark(t *testing.T) {
	labelMarkKey := "superphenix.net/markedForDeletion"
	namespace := "test-project"
	timestamp := time.Now().Add(48 * time.Hour).Format(utils.TimestampFormat)

	tests := []struct {
		name      string
		apps      []argocdv1alpha1.Application
		namespace string
		wantErr   bool
		wantCount int
	}{
		{
			name:      "no applications to mark",
			apps:      []argocdv1alpha1.Application{},
			namespace: namespace,
			wantErr:   false,
			wantCount: 0,
		},
		{
			name: "marks application with matching project label",
			apps: []argocdv1alpha1.Application{
				newApplication("app1", namespace, map[string]string{
					spxId.SpxLabelProjectID: namespace,
				}),
			},
			namespace: namespace,
			wantErr:   false,
			wantCount: 1,
		},
		{
			name: "does not mark application in different namespace",
			apps: []argocdv1alpha1.Application{
				newApplication("app1", "other-ns", map[string]string{
					spxId.SpxLabelProjectID: "other-ns",
				}),
			},
			namespace: namespace,
			wantErr:   false,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.Global.GarbageCollection.LabelMarkKey = labelMarkKey

			clientset := testhelper.NewFakeClientset()
			for i := range tt.apps {
				if err := clientset.Tracker().Add(&tt.apps[i]); err != nil {
					t.Fatalf("failed to add object to tracker: %v", err)
				}
			}
			config.ArgoClient = clientset.ArgoprojV1alpha1()

			c := &Cleaner{
				ResourceType: "Argo Application",
				Logger:       zerolog.Nop(),
			}

			err := c.Mark(context.Background(), tt.namespace, timestamp)
			if (err != nil) != tt.wantErr {
				t.Errorf("Mark() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			apps, err := config.ArgoClient.Applications(tt.namespace).List(context.Background(), metav1.ListOptions{})
			if err != nil {
				t.Fatalf("failed to list applications: %v", err)
			}

			markedCount := 0
			for _, app := range apps.Items {
				if app.GetLabels()[labelMarkKey] == timestamp {
					markedCount++
				}
			}
			if markedCount != tt.wantCount {
				t.Errorf("Mark() marked = %d, want %d", markedCount, tt.wantCount)
			}
		})
	}
}

func TestErrorMessage(t *testing.T) {
	c := &Cleaner{ResourceType: "Argo Application"}
	want := "Something went wrong during Argo Application cleaning."
	if got := c.ErrorMessage(); got != want {
		t.Errorf("ErrorMessage() = %q, want %q", got, want)
	}
}
