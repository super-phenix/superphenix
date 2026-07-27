package ssh

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

func newSecret(name, namespace string, labels map[string]string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: k8smetav1.ObjectMeta{
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
		secrets       []*corev1.Secret
		debug         bool
		wantErr       bool
		wantRemaining int
	}{
		{
			name:          "no marked secrets",
			secrets:       []*corev1.Secret{},
			wantErr:       false,
			wantRemaining: 0,
		},
		{
			name: "delete expired secret",
			secrets: []*corev1.Secret{
				newSecret("ssh-expired", "ns1", map[string]string{labelMarkKey: pastTimestamp}),
			},
			wantErr:       false,
			wantRemaining: 0,
		},
		{
			name: "skip future secret",
			secrets: []*corev1.Secret{
				newSecret("ssh-future", "ns1", map[string]string{labelMarkKey: futureTimestamp}),
			},
			wantErr:       false,
			wantRemaining: 1,
		},
		{
			name: "debug mode skips deletion",
			secrets: []*corev1.Secret{
				newSecret("ssh-debug", "ns1", map[string]string{labelMarkKey: pastTimestamp}),
			},
			debug:         true,
			wantErr:       false,
			wantRemaining: 1,
		},
		{
			name: "mixed expired and future",
			secrets: []*corev1.Secret{
				newSecret("ssh-expired", "ns1", map[string]string{labelMarkKey: pastTimestamp}),
				newSecret("ssh-future", "ns1", map[string]string{labelMarkKey: futureTimestamp}),
			},
			wantErr:       false,
			wantRemaining: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.Global.GarbageCollection.LabelMarkKey = labelMarkKey
			config.Global.GarbageCollection.Debug = tt.debug

			fakeClient := fake.NewClientset()
			for _, s := range tt.secrets {
				_, _ = fakeClient.CoreV1().Secrets(s.Namespace).Create(context.Background(), s, k8smetav1.CreateOptions{})
			}
			config.K8sClient = fakeClient

			watcherObjs := make([]interface{}, len(tt.secrets))
			for i, s := range tt.secrets {
				watcherObjs[i] = s
			}
			testhelper.SetupFakeWatcher(informers.SSH, watcherObjs...)

			c := &Cleaner{
				ResourceType: "SSH",
				Logger:       zerolog.Nop(),
			}

			err := c.Clean(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("Clean() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			remaining, _ := fakeClient.CoreV1().Secrets("ns1").List(context.Background(), k8smetav1.ListOptions{})
			if len(remaining.Items) != tt.wantRemaining {
				t.Errorf("Clean() remaining = %d, want %d", len(remaining.Items), tt.wantRemaining)
			}
		})
	}
}

func TestMark(t *testing.T) {
	labelMarkKey := "superphenix.net/markedForDeletion"
	timestamp := time.Now().Add(48 * time.Hour).Format(utils.TimestampFormat)
	projectNs := "project-1"

	tests := []struct {
		name      string
		secrets   []*corev1.Secret
		wantErr   bool
		wantCount int
	}{
		{
			name: "mark matching secrets",
			secrets: []*corev1.Secret{
				newSecret("ssh-1", "ns1", map[string]string{spxId.SpxLabelProjectID: projectNs}),
			},
			wantErr:   false,
			wantCount: 1,
		},
		{
			name:      "no secrets in namespace",
			secrets:   []*corev1.Secret{},
			wantErr:   false,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.Global.GarbageCollection.LabelMarkKey = labelMarkKey

			fakeClient := fake.NewClientset()
			for _, s := range tt.secrets {
				_, _ = fakeClient.CoreV1().Secrets(s.Namespace).Create(context.Background(), s, k8smetav1.CreateOptions{})
			}
			config.K8sClient = fakeClient

			watcherObjs := make([]interface{}, len(tt.secrets))
			for i, s := range tt.secrets {
				watcherObjs[i] = s
			}
			testhelper.SetupFakeWatcher(informers.SSH, watcherObjs...)

			c := &Cleaner{
				ResourceType: "SSH",
				Logger:       zerolog.Nop(),
			}

			err := c.Mark(context.Background(), projectNs, timestamp)
			if (err != nil) != tt.wantErr {
				t.Errorf("Mark() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantCount > 0 {
				s, _ := fakeClient.CoreV1().Secrets(tt.secrets[0].Namespace).Get(context.Background(), tt.secrets[0].Name, k8smetav1.GetOptions{})
				if s.Labels[labelMarkKey] != timestamp {
					t.Errorf("Mark() label = %v, want %v", s.Labels[labelMarkKey], timestamp)
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
			resourceType: "SSH",
			want:         "Something went wrong during SSH cleaning.",
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
