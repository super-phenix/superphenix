package volumeSnapshot

import (
	"context"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/resources/testhelper"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	"testing"
	"time"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	"github.com/rs/zerolog"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	k8stesting "k8s.io/client-go/testing"
	snapshotfake "kubevirt.io/client-go/externalsnapshotter/typed/volumesnapshot/v1/fake"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
)

func newVolumeSnapshot(name, namespace string, labels map[string]string) *volumesnapshotv1.VolumeSnapshot {
	return &volumesnapshotv1.VolumeSnapshot{
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
	}
}

func setupFakeSnapshotClient(objects ...*volumesnapshotv1.VolumeSnapshot) *snapshotfake.FakeSnapshotV1 {
	scheme := runtime.NewScheme()
	_ = volumesnapshotv1.AddToScheme(scheme)

	codecs := serializer.NewCodecFactory(scheme)
	tracker := k8stesting.NewObjectTracker(scheme, codecs.UniversalDecoder())

	for _, obj := range objects {
		_ = tracker.Add(obj)
	}

	fake := &k8stesting.Fake{}
	fake.AddReactor("*", "*", k8stesting.ObjectReaction(tracker))

	return &snapshotfake.FakeSnapshotV1{Fake: fake}
}

func TestClean(t *testing.T) {
	labelMarkKey := "superphenix.net/markedForDeletion"
	pastTimestamp := time.Now().Add(-1 * time.Hour).Format(utils.TimestampFormat)
	futureTimestamp := time.Now().Add(1 * time.Hour).Format(utils.TimestampFormat)

	tests := []struct {
		name          string
		snapshots     []*volumesnapshotv1.VolumeSnapshot
		debug         bool
		wantErr       bool
		wantRemaining int
	}{
		{
			name:          "no marked volume snapshots",
			snapshots:     []*volumesnapshotv1.VolumeSnapshot{},
			wantErr:       false,
			wantRemaining: 0,
		},
		{
			name: "delete expired volume snapshot",
			snapshots: []*volumesnapshotv1.VolumeSnapshot{
				newVolumeSnapshot("vs-expired", "ns1", map[string]string{labelMarkKey: pastTimestamp}),
			},
			wantErr:       false,
			wantRemaining: 0,
		},
		{
			name: "skip future volume snapshot",
			snapshots: []*volumesnapshotv1.VolumeSnapshot{
				newVolumeSnapshot("vs-future", "ns1", map[string]string{labelMarkKey: futureTimestamp}),
			},
			wantErr:       false,
			wantRemaining: 1,
		},
		{
			name: "debug mode skips deletion",
			snapshots: []*volumesnapshotv1.VolumeSnapshot{
				newVolumeSnapshot("vs-debug", "ns1", map[string]string{labelMarkKey: pastTimestamp}),
			},
			debug:         true,
			wantErr:       false,
			wantRemaining: 1,
		},
		{
			name: "mixed expired and future",
			snapshots: []*volumesnapshotv1.VolumeSnapshot{
				newVolumeSnapshot("vs-expired", "ns1", map[string]string{labelMarkKey: pastTimestamp}),
				newVolumeSnapshot("vs-future", "ns1", map[string]string{labelMarkKey: futureTimestamp}),
			},
			wantErr:       false,
			wantRemaining: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.Global.GarbageCollection.LabelMarkKey = labelMarkKey
			config.Global.GarbageCollection.Debug = tt.debug

			fakeClient := setupFakeSnapshotClient(tt.snapshots...)
			config.SnapshotClient = fakeClient

			watcherObjs := make([]interface{}, len(tt.snapshots))
			for i, s := range tt.snapshots {
				watcherObjs[i] = s
			}
			testhelper.SetupFakeWatcher(informers.VolumeSnapshot, watcherObjs...)

			c := &Cleaner{
				ResourceType: "Volume Snapshot",
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
		snapshots []*volumesnapshotv1.VolumeSnapshot
		wantErr   bool
		wantCount int
	}{
		{
			name: "mark matching volume snapshots",
			snapshots: []*volumesnapshotv1.VolumeSnapshot{
				newVolumeSnapshot("vs-1", projectNs, map[string]string{spxId.SpxLabelProjectID: projectNs}),
			},
			wantErr:   false,
			wantCount: 1,
		},
		{
			name:      "no volume snapshots in namespace",
			snapshots: []*volumesnapshotv1.VolumeSnapshot{},
			wantErr:   false,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.Global.GarbageCollection.LabelMarkKey = labelMarkKey

			fakeClient := setupFakeSnapshotClient(tt.snapshots...)
			config.SnapshotClient = fakeClient

			watcherObjs := make([]interface{}, len(tt.snapshots))
			for i, s := range tt.snapshots {
				watcherObjs[i] = s
			}
			testhelper.SetupFakeWatcher(informers.VolumeSnapshot, watcherObjs...)

			c := &Cleaner{
				ResourceType: "Volume Snapshot",
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
			resourceType: "Volume Snapshot",
			want:         "Something went wrong during Volume Snapshot cleaning.",
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
