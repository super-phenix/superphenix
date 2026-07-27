package datavolume

import (
	"context"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/resources/testhelper"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"go.uber.org/mock/gomock"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"kubevirt.io/client-go/kubecli"
	"kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
)

func newDataVolume(name, namespace string, labels map[string]string) *v1beta1.DataVolume {
	return &v1beta1.DataVolume{
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
	}
}

func setupFakeCdiClient(ctrl *gomock.Controller, objects ...runtime.Object) (*kubecli.MockKubevirtClient, *testhelper.FakeCdiClientset) {
	mockClient := kubecli.NewMockKubevirtClient(ctrl)
	fakeCdi := testhelper.NewFakeCdiClientset(objects...)

	mockClient.EXPECT().CdiClient().Return(fakeCdi).AnyTimes()

	return mockClient, fakeCdi
}

func TestClean(t *testing.T) {
	labelMarkKey := "superphenix.net/markedForDeletion"
	pastTimestamp := time.Now().Add(-1 * time.Hour).Format(utils.TimestampFormat)
	futureTimestamp := time.Now().Add(1 * time.Hour).Format(utils.TimestampFormat)

	tests := []struct {
		name          string
		dvs           []*v1beta1.DataVolume
		debug         bool
		wantErr       bool
		wantRemaining int
	}{
		{
			name:          "no marked data volumes",
			dvs:           []*v1beta1.DataVolume{},
			wantErr:       false,
			wantRemaining: 0,
		},
		{
			name: "delete expired data volume",
			dvs: []*v1beta1.DataVolume{
				newDataVolume("dv-expired", "ns1", map[string]string{labelMarkKey: pastTimestamp}),
			},
			wantErr:       false,
			wantRemaining: 0,
		},
		{
			name: "skip future data volume",
			dvs: []*v1beta1.DataVolume{
				newDataVolume("dv-future", "ns1", map[string]string{labelMarkKey: futureTimestamp}),
			},
			wantErr:       false,
			wantRemaining: 1,
		},
		{
			name: "debug mode skips deletion",
			dvs: []*v1beta1.DataVolume{
				newDataVolume("dv-debug", "ns1", map[string]string{labelMarkKey: pastTimestamp}),
			},
			debug:         true,
			wantErr:       false,
			wantRemaining: 1,
		},
		{
			name: "mixed expired and future",
			dvs: []*v1beta1.DataVolume{
				newDataVolume("dv-expired", "ns1", map[string]string{labelMarkKey: pastTimestamp}),
				newDataVolume("dv-future", "ns1", map[string]string{labelMarkKey: futureTimestamp}),
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

			objs := make([]runtime.Object, len(tt.dvs))
			for i, dv := range tt.dvs {
				objs[i] = dv
			}
			mockClient, fakeCdi := setupFakeCdiClient(ctrl, objs...)
			config.VirtClient = mockClient

			watcherObjs := make([]interface{}, len(tt.dvs))
			for i, dv := range tt.dvs {
				watcherObjs[i] = dv
			}
			testhelper.SetupFakeWatcher(informers.DataVolume, watcherObjs...)

			c := &Cleaner{
				ResourceType: "DataVolume",
				Logger:       zerolog.Nop(),
			}

			err := c.Clean(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("Clean() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			remaining, _ := fakeCdi.CdiV1beta1().DataVolumes("ns1").List(context.Background(), k8smetav1.ListOptions{})
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
		dvs       []*v1beta1.DataVolume
		wantErr   bool
		wantCount int
	}{
		{
			name: "mark matching data volumes",
			dvs: []*v1beta1.DataVolume{
				newDataVolume("dv-1", projectNs, map[string]string{spxId.SpxLabelProjectID: projectNs}),
			},
			wantErr:   false,
			wantCount: 1,
		},
		{
			name:      "no data volumes in namespace",
			dvs:       []*v1beta1.DataVolume{},
			wantErr:   false,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.Global.GarbageCollection.LabelMarkKey = labelMarkKey

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			objs := make([]runtime.Object, len(tt.dvs))
			for i, dv := range tt.dvs {
				objs[i] = dv
			}
			mockClient, fakeCdi := setupFakeCdiClient(ctrl, objs...)
			config.VirtClient = mockClient

			watcherObjs := make([]interface{}, len(tt.dvs))
			for i, dv := range tt.dvs {
				watcherObjs[i] = dv
			}
			testhelper.SetupFakeWatcher(informers.DataVolume, watcherObjs...)

			c := &Cleaner{
				ResourceType: "DataVolume",
				Logger:       zerolog.Nop(),
			}

			err := c.Mark(context.Background(), projectNs, timestamp)
			if (err != nil) != tt.wantErr {
				t.Errorf("Mark() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantCount > 0 {
				dv, _ := fakeCdi.CdiV1beta1().DataVolumes(projectNs).Get(context.Background(), tt.dvs[0].Name, k8smetav1.GetOptions{})
				if dv.Labels[labelMarkKey] != timestamp {
					t.Errorf("Mark() label = %v, want %v", dv.Labels[labelMarkKey], timestamp)
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
			resourceType: "DataVolume",
			want:         "Something went wrong during DataVolume cleaning.",
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
