package vmSnapshot

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
	"k8s.io/apimachinery/pkg/runtime/serializer"
	k8stesting "k8s.io/client-go/testing"
	"kubevirt.io/api/snapshot/v1beta1"
	"kubevirt.io/client-go/kubecli"
	snapshotfake "kubevirt.io/client-go/kubevirt/typed/snapshot/v1beta1/fake"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
)

func newVMSnapshot(name, namespace string, labels map[string]string) *v1beta1.VirtualMachineSnapshot {
	return &v1beta1.VirtualMachineSnapshot{
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
	}
}

func setupFakeSnapshotClient(ctrl *gomock.Controller, objects ...*v1beta1.VirtualMachineSnapshot) *kubecli.MockKubevirtClient {
	mockClient := kubecli.NewMockKubevirtClient(ctrl)

	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)

	fake := &k8stesting.Fake{}
	codecs := serializer.NewCodecFactory(scheme)
	tracker := k8stesting.NewObjectTracker(scheme, codecs.UniversalDecoder())

	for _, obj := range objects {
		_ = tracker.Add(obj)
	}
	fake.AddReactor("*", "*", k8stesting.ObjectReaction(tracker))

	fakeSnapshot := &snapshotfake.FakeSnapshotV1beta1{Fake: fake}

	mockClient.EXPECT().VirtualMachineSnapshot(gomock.Any()).DoAndReturn(
		func(namespace string) interface{} {
			return fakeSnapshot.VirtualMachineSnapshots(namespace)
		},
	).AnyTimes()

	return mockClient
}

func TestClean(t *testing.T) {
	labelMarkKey := "superphenix.net/markedForDeletion"
	pastTimestamp := time.Now().Add(-1 * time.Hour).Format(utils.TimestampFormat)
	futureTimestamp := time.Now().Add(1 * time.Hour).Format(utils.TimestampFormat)

	tests := []struct {
		name          string
		snapshots     []*v1beta1.VirtualMachineSnapshot
		debug         bool
		wantErr       bool
		wantRemaining int
	}{
		{
			name:          "no marked snapshots",
			snapshots:     []*v1beta1.VirtualMachineSnapshot{},
			wantErr:       false,
			wantRemaining: 0,
		},
		{
			name: "delete expired snapshot",
			snapshots: []*v1beta1.VirtualMachineSnapshot{
				newVMSnapshot("snap-expired", "ns1", map[string]string{labelMarkKey: pastTimestamp}),
			},
			wantErr:       false,
			wantRemaining: 0,
		},
		{
			name: "skip future snapshot",
			snapshots: []*v1beta1.VirtualMachineSnapshot{
				newVMSnapshot("snap-future", "ns1", map[string]string{labelMarkKey: futureTimestamp}),
			},
			wantErr:       false,
			wantRemaining: 1,
		},
		{
			name: "debug mode skips deletion",
			snapshots: []*v1beta1.VirtualMachineSnapshot{
				newVMSnapshot("snap-debug", "ns1", map[string]string{labelMarkKey: pastTimestamp}),
			},
			debug:         true,
			wantErr:       false,
			wantRemaining: 1,
		},
		{
			name: "mixed expired and future",
			snapshots: []*v1beta1.VirtualMachineSnapshot{
				newVMSnapshot("snap-expired", "ns1", map[string]string{labelMarkKey: pastTimestamp}),
				newVMSnapshot("snap-future", "ns1", map[string]string{labelMarkKey: futureTimestamp}),
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

			config.VirtClient = setupFakeSnapshotClient(ctrl, tt.snapshots...)

			watcherObjs := make([]interface{}, len(tt.snapshots))
			for i, s := range tt.snapshots {
				watcherObjs[i] = s
			}
			testhelper.SetupFakeWatcher(informers.VirtualMachineSnapshot, watcherObjs...)

			c := &Cleaner{
				ResourceType: "VM Snapshot",
				Logger:       zerolog.Nop(),
			}

			err := c.Clean(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("Clean() error = %v, wantErr %v", err, tt.wantErr)
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
		snapshots []*v1beta1.VirtualMachineSnapshot
		wantErr   bool
		wantCount int
	}{
		{
			name: "mark matching snapshots",
			snapshots: []*v1beta1.VirtualMachineSnapshot{
				newVMSnapshot("snap-1", projectNs, map[string]string{spxId.SpxLabelProjectID: projectNs}),
			},
			wantErr:   false,
			wantCount: 1,
		},
		{
			name:      "no snapshots in namespace",
			snapshots: []*v1beta1.VirtualMachineSnapshot{},
			wantErr:   false,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.Global.GarbageCollection.LabelMarkKey = labelMarkKey

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			config.VirtClient = setupFakeSnapshotClient(ctrl, tt.snapshots...)

			watcherObjs := make([]interface{}, len(tt.snapshots))
			for i, s := range tt.snapshots {
				watcherObjs[i] = s
			}
			testhelper.SetupFakeWatcher(informers.VirtualMachineSnapshot, watcherObjs...)

			c := &Cleaner{
				ResourceType: "VM Snapshot",
				Logger:       zerolog.Nop(),
			}

			err := c.Mark(context.Background(), projectNs, timestamp)
			if (err != nil) != tt.wantErr {
				t.Errorf("Mark() error = %v, wantErr %v", err, tt.wantErr)
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
			resourceType: "VM Snapshot",
			want:         "Something went wrong during VM Snapshot cleaning.",
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
