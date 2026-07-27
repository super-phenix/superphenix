package subnet

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
	"go.uber.org/mock/gomock"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kubevirt.io/client-go/kubecli"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
)

func newSubnet(name string, labels map[string]string) *v1.Subnet {
	return &v1.Subnet{
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
		subnets       []*v1.Subnet
		debug         bool
		wantErr       bool
		wantRemaining int
	}{
		{
			name:          "no marked subnets",
			subnets:       []*v1.Subnet{},
			wantErr:       false,
			wantRemaining: 0,
		},
		{
			name: "delete expired subnet",
			subnets: []*v1.Subnet{
				newSubnet("subnet-expired", map[string]string{labelMarkKey: pastTimestamp}),
			},
			wantErr:       false,
			wantRemaining: 0,
		},
		{
			name: "skip future subnet",
			subnets: []*v1.Subnet{
				newSubnet("subnet-future", map[string]string{labelMarkKey: futureTimestamp}),
			},
			wantErr:       false,
			wantRemaining: 1,
		},
		{
			name: "debug mode skips deletion",
			subnets: []*v1.Subnet{
				newSubnet("subnet-debug", map[string]string{labelMarkKey: pastTimestamp}),
			},
			debug:         true,
			wantErr:       false,
			wantRemaining: 1,
		},
		{
			name: "mixed expired and future",
			subnets: []*v1.Subnet{
				newSubnet("subnet-expired", map[string]string{labelMarkKey: pastTimestamp}),
				newSubnet("subnet-future", map[string]string{labelMarkKey: futureTimestamp}),
			},
			wantErr:       false,
			wantRemaining: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.Global.GarbageCollection.LabelMarkKey = labelMarkKey
			config.Global.GarbageCollection.Debug = tt.debug

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockVirtClient := kubecli.NewMockKubevirtClient(ctrl)
			fakeNadClient := testhelper.NewFakeNadClientset()
			mockVirtClient.EXPECT().NetworkClient().Return(fakeNadClient).AnyTimes()
			config.VirtClient = mockVirtClient

			fakeClient := testhelper.NewFakeKubeOvnClientset()
			for _, s := range tt.subnets {
				_, err := fakeClient.KubeovnV1().Subnets().Create(context.Background(), s, k8smetav1.CreateOptions{})
				if err != nil {
					t.Fatalf("failed to create subnet: %v", err)
				}
			}
			config.KubeOvnClient = fakeClient

			watcherObjs := make([]interface{}, len(tt.subnets))
			for i, s := range tt.subnets {
				watcherObjs[i] = s
			}
			testhelper.SetupFakeWatcher(informers.Subnet, watcherObjs...)

			c := &Cleaner{
				ResourceType: "Subnet",
				Logger:       zerolog.Nop(),
			}

			err := c.Clean(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("Clean() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			remaining, _ := fakeClient.KubeovnV1().Subnets().List(context.Background(), k8smetav1.ListOptions{})
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
		subnets   []*v1.Subnet
		wantErr   bool
		wantCount int
	}{
		{
			name: "mark matching subnets",
			subnets: []*v1.Subnet{
				newSubnet("subnet-1", map[string]string{spxId.SpxLabelProjectID: projectNs}),
			},
			wantErr:   false,
			wantCount: 1,
		},
		{
			name:      "no subnets in namespace",
			subnets:   []*v1.Subnet{},
			wantErr:   false,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.Global.GarbageCollection.LabelMarkKey = labelMarkKey

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockVirtClient := kubecli.NewMockKubevirtClient(ctrl)
			fakeNadClient := testhelper.NewFakeNadClientset()
			mockVirtClient.EXPECT().NetworkClient().Return(fakeNadClient).AnyTimes()
			config.VirtClient = mockVirtClient

			fakeClient := testhelper.NewFakeKubeOvnClientset()
			for _, s := range tt.subnets {
				_, err := fakeClient.KubeovnV1().Subnets().Create(context.Background(), s, k8smetav1.CreateOptions{})
				if err != nil {
					t.Fatalf("failed to create subnet: %v", err)
				}
			}
			config.KubeOvnClient = fakeClient

			watcherObjs := make([]interface{}, len(tt.subnets))
			for i, s := range tt.subnets {
				watcherObjs[i] = s
			}
			testhelper.SetupFakeWatcher(informers.Subnet, watcherObjs...)

			// Setup empty NAD and NatGw watchers needed by Mark
			testhelper.SetupFakeWatcher(informers.NAD)
			testhelper.SetupFakeWatcher(informers.NatGw)

			c := &Cleaner{
				ResourceType: "Subnet",
				Logger:       zerolog.Nop(),
			}

			err := c.Mark(context.Background(), projectNs, timestamp)
			if (err != nil) != tt.wantErr {
				t.Errorf("Mark() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantCount > 0 {
				s, _ := fakeClient.KubeovnV1().Subnets().Get(context.Background(), tt.subnets[0].Name, k8smetav1.GetOptions{})
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
			resourceType: "Subnet",
			want:         "Something went wrong during Subnet cleaning.",
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
