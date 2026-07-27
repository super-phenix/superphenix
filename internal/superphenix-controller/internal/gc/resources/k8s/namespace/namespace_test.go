package namespace

import (
	"context"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/resources/testhelper"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	"testing"
	"time"

	"github.com/rs/zerolog"
	corev1 "k8s.io/api/core/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
)

func newNamespace(name string, labels map[string]string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
}

func TestClean(t *testing.T) {
	labelMarkKey := "superphenix.net/markedForDeletion"
	pastTimestamp := time.Now().Add(-1 * time.Hour).Format(utils.TimestampFormat)
	futureTimestamp := time.Now().Add(1 * time.Hour).Format(utils.TimestampFormat)

	tests := []struct {
		name          string
		namespaces    []*corev1.Namespace
		debug         bool
		wantErr       bool
		wantRemaining int
	}{
		{
			name:          "no marked namespaces",
			namespaces:    []*corev1.Namespace{},
			wantErr:       false,
			wantRemaining: 0,
		},
		{
			name: "delete expired namespace",
			namespaces: []*corev1.Namespace{
				newNamespace("ns-expired", map[string]string{labelMarkKey: pastTimestamp}),
			},
			wantErr:       false,
			wantRemaining: 0,
		},
		{
			name: "skip future namespace",
			namespaces: []*corev1.Namespace{
				newNamespace("ns-future", map[string]string{labelMarkKey: futureTimestamp}),
			},
			wantErr:       false,
			wantRemaining: 1,
		},
		{
			name: "debug mode skips deletion",
			namespaces: []*corev1.Namespace{
				newNamespace("ns-debug", map[string]string{labelMarkKey: pastTimestamp}),
			},
			debug:         true,
			wantErr:       false,
			wantRemaining: 1,
		},
		{
			name: "mixed expired and future",
			namespaces: []*corev1.Namespace{
				newNamespace("ns-expired", map[string]string{labelMarkKey: pastTimestamp}),
				newNamespace("ns-future", map[string]string{labelMarkKey: futureTimestamp}),
			},
			wantErr:       false,
			wantRemaining: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.Global.GarbageCollection.LabelMarkKey = labelMarkKey
			config.Global.GarbageCollection.Debug = tt.debug

			// Setup fake k8s client with existing namespaces
			objs := make([]k8smetav1.Object, len(tt.namespaces))
			for i, ns := range tt.namespaces {
				objs[i] = ns
			}
			fakeClient := fake.NewClientset()
			for _, ns := range tt.namespaces {
				_, _ = fakeClient.CoreV1().Namespaces().Create(context.Background(), ns, k8smetav1.CreateOptions{})
			}
			config.K8sClient = fakeClient

			// Setup fake watcher with marked namespaces
			watcherObjs := make([]interface{}, len(tt.namespaces))
			for i, ns := range tt.namespaces {
				watcherObjs[i] = ns
			}
			testhelper.SetupFakeWatcher(informers.Namespace, watcherObjs...)

			c := &Cleaner{
				ResourceType: "Namespace",
				Logger:       zerolog.Nop(),
			}

			err := c.Clean(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("Clean() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			remaining, _ := fakeClient.CoreV1().Namespaces().List(context.Background(), k8smetav1.ListOptions{})
			if len(remaining.Items) != tt.wantRemaining {
				t.Errorf("Clean() remaining = %d, want %d", len(remaining.Items), tt.wantRemaining)
			}
		})
	}
}

func TestMark(t *testing.T) {
	labelMarkKey := "superphenix.net/markedForDeletion"
	timestamp := time.Now().Add(48 * time.Hour).Format(utils.TimestampFormat)

	tests := []struct {
		name      string
		namespace string
		existing  *corev1.Namespace
		wantErr   bool
		wantLabel bool
	}{
		{
			name:      "mark existing namespace",
			namespace: "test-ns",
			existing: newNamespace("test-ns", map[string]string{
				spxId.SpxLabelProjectID: "test-ns",
			}),
			wantErr:   false,
			wantLabel: true,
		},
		{
			name:      "namespace not found returns nil",
			namespace: "nonexistent",
			existing:  nil,
			wantErr:   false,
			wantLabel: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.Global.GarbageCollection.LabelMarkKey = labelMarkKey

			fakeClient := fake.NewClientset()
			if tt.existing != nil {
				_, _ = fakeClient.CoreV1().Namespaces().Create(context.Background(), tt.existing, k8smetav1.CreateOptions{})
			}
			config.K8sClient = fakeClient

			c := &Cleaner{
				ResourceType: "Namespace",
				Logger:       zerolog.Nop(),
			}

			err := c.Mark(context.Background(), tt.namespace, timestamp)
			if (err != nil) != tt.wantErr {
				t.Errorf("Mark() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantLabel {
				ns, _ := fakeClient.CoreV1().Namespaces().Get(context.Background(), tt.namespace, k8smetav1.GetOptions{})
				if ns.Labels[labelMarkKey] != timestamp {
					t.Errorf("Mark() label = %v, want %v", ns.Labels[labelMarkKey], timestamp)
				}
			}
		})
	}
}

func TestErrorMessage(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
		want         string
	}{
		{
			name:         "returns correct error message",
			resourceType: "Namespace",
			want:         "Something went wrong during Namespace cleaning.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Cleaner{ResourceType: tt.resourceType}
			if got := c.ErrorMessage(); got != tt.want {
				t.Errorf("ErrorMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}
