package netpol

import (
	"context"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/resources/testhelper"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	"testing"
	"time"

	"github.com/rs/zerolog"
	networkingv1 "k8s.io/api/networking/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
)

func newNetworkPolicy(name, namespace string, labels map[string]string) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
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
		netpols       []*networkingv1.NetworkPolicy
		debug         bool
		wantErr       bool
		wantRemaining int
	}{
		{
			name:          "no marked network policies",
			netpols:       []*networkingv1.NetworkPolicy{},
			wantErr:       false,
			wantRemaining: 0,
		},
		{
			name: "delete expired network policy",
			netpols: []*networkingv1.NetworkPolicy{
				newNetworkPolicy("np-expired", "ns1", map[string]string{labelMarkKey: pastTimestamp}),
			},
			wantErr:       false,
			wantRemaining: 0,
		},
		{
			name: "skip future network policy",
			netpols: []*networkingv1.NetworkPolicy{
				newNetworkPolicy("np-future", "ns1", map[string]string{labelMarkKey: futureTimestamp}),
			},
			wantErr:       false,
			wantRemaining: 1,
		},
		{
			name: "debug mode skips deletion",
			netpols: []*networkingv1.NetworkPolicy{
				newNetworkPolicy("np-debug", "ns1", map[string]string{labelMarkKey: pastTimestamp}),
			},
			debug:         true,
			wantErr:       false,
			wantRemaining: 1,
		},
		{
			name: "mixed expired and future",
			netpols: []*networkingv1.NetworkPolicy{
				newNetworkPolicy("np-expired", "ns1", map[string]string{labelMarkKey: pastTimestamp}),
				newNetworkPolicy("np-future", "ns1", map[string]string{labelMarkKey: futureTimestamp}),
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
			for _, np := range tt.netpols {
				_, _ = fakeClient.NetworkingV1().NetworkPolicies(np.Namespace).Create(context.Background(), np, k8smetav1.CreateOptions{})
			}
			config.K8sClient = fakeClient

			watcherObjs := make([]interface{}, len(tt.netpols))
			for i, np := range tt.netpols {
				watcherObjs[i] = np
			}
			testhelper.SetupFakeWatcher(informers.NetworkPolicy, watcherObjs...)

			c := &Cleaner{
				ResourceType: "NetworkPolicy",
				Logger:       zerolog.Nop(),
			}

			err := c.Clean(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("Clean() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			remaining, _ := fakeClient.NetworkingV1().NetworkPolicies("ns1").List(context.Background(), k8smetav1.ListOptions{})
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
		netpols   []*networkingv1.NetworkPolicy
		wantErr   bool
		wantCount int
	}{
		{
			name: "mark matching network policies",
			netpols: []*networkingv1.NetworkPolicy{
				newNetworkPolicy("np-1", "ns1", map[string]string{spxId.SpxLabelProjectID: projectNs}),
			},
			wantErr:   false,
			wantCount: 1,
		},
		{
			name:      "no network policies in namespace",
			netpols:   []*networkingv1.NetworkPolicy{},
			wantErr:   false,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.Global.GarbageCollection.LabelMarkKey = labelMarkKey

			fakeClient := fake.NewClientset()
			for _, np := range tt.netpols {
				_, _ = fakeClient.NetworkingV1().NetworkPolicies(np.Namespace).Create(context.Background(), np, k8smetav1.CreateOptions{})
			}
			config.K8sClient = fakeClient

			watcherObjs := make([]interface{}, len(tt.netpols))
			for i, np := range tt.netpols {
				watcherObjs[i] = np
			}
			testhelper.SetupFakeWatcher(informers.NetworkPolicy, watcherObjs...)

			c := &Cleaner{
				ResourceType: "NetworkPolicy",
				Logger:       zerolog.Nop(),
			}

			err := c.Mark(context.Background(), projectNs, timestamp)
			if (err != nil) != tt.wantErr {
				t.Errorf("Mark() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantCount > 0 {
				np, _ := fakeClient.NetworkingV1().NetworkPolicies(tt.netpols[0].Namespace).Get(context.Background(), tt.netpols[0].Name, k8smetav1.GetOptions{})
				if np.Labels[labelMarkKey] != timestamp {
					t.Errorf("Mark() label = %v, want %v", np.Labels[labelMarkKey], timestamp)
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
			resourceType: "NetworkPolicy",
			want:         "Something went wrong during NetworkPolicy cleaning.",
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
