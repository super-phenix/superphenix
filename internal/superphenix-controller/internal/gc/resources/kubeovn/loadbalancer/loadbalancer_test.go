package loadbalancer

import (
	"context"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/resources/testhelper"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	"testing"
	"time"

	v1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	"github.com/rs/zerolog"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
)

func newSwitchLBRule(name string, labels map[string]string) *v1.SwitchLBRule {
	return &v1.SwitchLBRule{
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
		lbs           []*v1.SwitchLBRule
		debug         bool
		wantErr       bool
		wantRemaining int
	}{
		{
			name:          "no marked load balancers",
			lbs:           []*v1.SwitchLBRule{},
			wantErr:       false,
			wantRemaining: 0,
		},
		{
			name: "delete expired load balancer",
			lbs: []*v1.SwitchLBRule{
				newSwitchLBRule("lb-expired", map[string]string{labelMarkKey: pastTimestamp}),
			},
			wantErr:       false,
			wantRemaining: 0,
		},
		{
			name: "skip future load balancer",
			lbs: []*v1.SwitchLBRule{
				newSwitchLBRule("lb-future", map[string]string{labelMarkKey: futureTimestamp}),
			},
			wantErr:       false,
			wantRemaining: 1,
		},
		{
			name: "debug mode skips deletion",
			lbs: []*v1.SwitchLBRule{
				newSwitchLBRule("lb-debug", map[string]string{labelMarkKey: pastTimestamp}),
			},
			debug:         true,
			wantErr:       false,
			wantRemaining: 1,
		},
		{
			name: "mixed expired and future",
			lbs: []*v1.SwitchLBRule{
				newSwitchLBRule("lb-expired", map[string]string{labelMarkKey: pastTimestamp}),
				newSwitchLBRule("lb-future", map[string]string{labelMarkKey: futureTimestamp}),
			},
			wantErr:       false,
			wantRemaining: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.Global.GarbageCollection.LabelMarkKey = labelMarkKey
			config.Global.GarbageCollection.Debug = tt.debug

			fakeClient := testhelper.NewFakeKubeOvnClientset()
			for _, lb := range tt.lbs {
				_, err := fakeClient.KubeovnV1().SwitchLBRules().Create(context.Background(), lb, k8smetav1.CreateOptions{})
				if err != nil {
					t.Fatalf("failed to create object: %v", err)
				}
			}
			config.KubeOvnClient = fakeClient

			watcherObjs := make([]interface{}, len(tt.lbs))
			for i, lb := range tt.lbs {
				watcherObjs[i] = lb
			}
			testhelper.SetupFakeWatcher(informers.SwitchLBRules, watcherObjs...)

			c := &Cleaner{
				ResourceType: "LoadBalancer",
				Logger:       zerolog.Nop(),
			}

			err := c.Clean(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("Clean() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			remaining, _ := fakeClient.KubeovnV1().SwitchLBRules().List(context.Background(), k8smetav1.ListOptions{})
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
		lbs       []*v1.SwitchLBRule
		wantErr   bool
		wantCount int
	}{
		{
			name: "mark matching load balancers",
			lbs: []*v1.SwitchLBRule{
				newSwitchLBRule("lb-1", map[string]string{spxId.SpxLabelProjectID: projectNs}),
			},
			wantErr:   false,
			wantCount: 1,
		},
		{
			name:      "no load balancers in namespace",
			lbs:       []*v1.SwitchLBRule{},
			wantErr:   false,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.Global.GarbageCollection.LabelMarkKey = labelMarkKey

			fakeClient := testhelper.NewFakeKubeOvnClientset()
			for _, lb := range tt.lbs {
				_, err := fakeClient.KubeovnV1().SwitchLBRules().Create(context.Background(), lb, k8smetav1.CreateOptions{})
				if err != nil {
					t.Fatalf("failed to create object: %v", err)
				}
			}
			config.KubeOvnClient = fakeClient

			watcherObjs := make([]interface{}, len(tt.lbs))
			for i, lb := range tt.lbs {
				watcherObjs[i] = lb
			}
			testhelper.SetupFakeWatcher(informers.SwitchLBRules, watcherObjs...)

			c := &Cleaner{
				ResourceType: "LoadBalancer",
				Logger:       zerolog.Nop(),
			}

			err := c.Mark(context.Background(), projectNs, timestamp)
			if (err != nil) != tt.wantErr {
				t.Errorf("Mark() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantCount > 0 {
				lb, _ := fakeClient.KubeovnV1().SwitchLBRules().Get(context.Background(), tt.lbs[0].Name, k8smetav1.GetOptions{})
				if lb.Labels[labelMarkKey] != timestamp {
					t.Errorf("Mark() label = %v, want %v", lb.Labels[labelMarkKey], timestamp)
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
			resourceType: "LoadBalancer",
			want:         "Something went wrong during LoadBalancer cleaning.",
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
