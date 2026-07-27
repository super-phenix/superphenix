package vpc

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
	"k8s.io/apimachinery/pkg/runtime"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
)

func newVpc(name string, labels map[string]string) *v1.Vpc {
	return &v1.Vpc{
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
		vpcs          []*v1.Vpc
		debug         bool
		wantErr       bool
		wantRemaining int
	}{
		{
			name:          "no marked VPCs",
			vpcs:          []*v1.Vpc{},
			wantErr:       false,
			wantRemaining: 0,
		},
		{
			name: "delete expired VPC",
			vpcs: []*v1.Vpc{
				newVpc("vpc-expired", map[string]string{labelMarkKey: pastTimestamp}),
			},
			wantErr:       false,
			wantRemaining: 0,
		},
		{
			name: "skip future VPC",
			vpcs: []*v1.Vpc{
				newVpc("vpc-future", map[string]string{labelMarkKey: futureTimestamp}),
			},
			wantErr:       false,
			wantRemaining: 1,
		},
		{
			name: "debug mode skips deletion",
			vpcs: []*v1.Vpc{
				newVpc("vpc-debug", map[string]string{labelMarkKey: pastTimestamp}),
			},
			debug:         true,
			wantErr:       false,
			wantRemaining: 1,
		},
		{
			name: "mixed expired and future",
			vpcs: []*v1.Vpc{
				newVpc("vpc-expired", map[string]string{labelMarkKey: pastTimestamp}),
				newVpc("vpc-future", map[string]string{labelMarkKey: futureTimestamp}),
			},
			wantErr:       false,
			wantRemaining: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.Global.GarbageCollection.LabelMarkKey = labelMarkKey
			config.Global.GarbageCollection.Debug = tt.debug

			objs := make([]runtime.Object, len(tt.vpcs))
			for i, vpc := range tt.vpcs {
				objs[i] = vpc
			}
			fakeClient := testhelper.NewFakeKubeOvnClientset(objs...)
			config.KubeOvnClient = fakeClient

			watcherObjs := make([]interface{}, len(tt.vpcs))
			for i, vpc := range tt.vpcs {
				watcherObjs[i] = vpc
			}
			testhelper.SetupFakeWatcher(informers.VPC, watcherObjs...)

			c := &Cleaner{
				ResourceType: "VPC",
				Logger:       zerolog.Nop(),
			}

			err := c.Clean(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("Clean() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			remaining, _ := fakeClient.KubeovnV1().Vpcs().List(context.Background(), k8smetav1.ListOptions{})
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
		vpcs      []*v1.Vpc
		wantErr   bool
		wantCount int
	}{
		{
			name: "mark matching VPCs",
			vpcs: []*v1.Vpc{
				newVpc("vpc-1", map[string]string{spxId.SpxLabelProjectID: projectNs}),
			},
			wantErr:   false,
			wantCount: 1,
		},
		{
			name:      "no VPCs in namespace",
			vpcs:      []*v1.Vpc{},
			wantErr:   false,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.Global.GarbageCollection.LabelMarkKey = labelMarkKey

			objs := make([]runtime.Object, len(tt.vpcs))
			for i, vpc := range tt.vpcs {
				objs[i] = vpc
			}
			fakeClient := testhelper.NewFakeKubeOvnClientset(objs...)
			config.KubeOvnClient = fakeClient

			watcherObjs := make([]interface{}, len(tt.vpcs))
			for i, vpc := range tt.vpcs {
				watcherObjs[i] = vpc
			}
			testhelper.SetupFakeWatcher(informers.VPC, watcherObjs...)

			c := &Cleaner{
				ResourceType: "VPC",
				Logger:       zerolog.Nop(),
			}

			err := c.Mark(context.Background(), projectNs, timestamp)
			if (err != nil) != tt.wantErr {
				t.Errorf("Mark() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantCount > 0 {
				vpc, _ := fakeClient.KubeovnV1().Vpcs().Get(context.Background(), tt.vpcs[0].Name, k8smetav1.GetOptions{})
				if vpc.Labels[labelMarkKey] != timestamp {
					t.Errorf("Mark() label = %v, want %v", vpc.Labels[labelMarkKey], timestamp)
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
			resourceType: "VPC",
			want:         "Something went wrong during VPC cleaning.",
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
