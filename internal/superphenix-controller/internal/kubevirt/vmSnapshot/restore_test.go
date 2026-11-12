package vmSnapshot

import (
	"context"
	"errors"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"go.uber.org/mock/gomock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	k8stesting "k8s.io/client-go/testing"
	virtv1 "kubevirt.io/api/core/v1"
	"kubevirt.io/api/snapshot/v1beta1"
	"kubevirt.io/client-go/kubecli"
	snapshotfake "kubevirt.io/client-go/kubevirt/typed/snapshot/v1beta1/fake"
)

const (
	restoreTestOrgId     = "6f1c1a4e-2d1b-4c55-9a43-3f4f1e2b7a10"
	restoreTestProjectId = "0b8a7c55-8f1e-4d3a-9e57-2c6d4b1a9f21"
	restoreTestSnapshot  = "spx-snapshot"
	restoreTestSourceVm  = "spx-source-vm"
)

func TestRestoreVmSnapshot(t *testing.T) {
	namespace := spxId.ToSPXID(restoreTestProjectId)
	vmNotFound := apierrors.NewNotFound(schema.GroupResource{Resource: "virtualmachines"}, restoreTestSourceVm)
	errLookup := errors.New("api server unavailable")

	snapshot := &v1beta1.VirtualMachineSnapshot{
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:      restoreTestSnapshot,
			Namespace: namespace,
			Labels:    map[string]string{spxId.SpxLabelProjectID: namespace},
		},
	}
	snapshot.Spec.Source.Kind = virtv1.VirtualMachineGroupVersionKind.Kind
	snapshot.Spec.Source.Name = restoreTestSourceVm

	newVm := func(projectLabel string) *virtv1.VirtualMachine {
		return &virtv1.VirtualMachine{
			ObjectMeta: k8smetav1.ObjectMeta{
				Name:      restoreTestSourceVm,
				Namespace: namespace,
				Labels:    map[string]string{spxId.SpxLabelProjectID: projectLabel},
			},
		}
	}

	tests := []struct {
		name        string
		objects     []runtime.Object
		vm          *virtv1.VirtualMachine
		vmErr       error
		wantVmGet   bool
		wantErr     func(error) bool
		wantRestore bool
	}{
		{
			name:        "source VM exists",
			objects:     []runtime.Object{snapshot},
			vm:          newVm(namespace),
			wantVmGet:   true,
			wantRestore: true,
		},
		{
			name:      "source VM not found",
			objects:   []runtime.Object{snapshot},
			vmErr:     vmNotFound,
			wantVmGet: true,
			wantErr:   func(err error) bool { return errors.Is(err, ErrRestoreSourceMissing) },
		},
		{
			name:      "source VM belongs to another project",
			objects:   []runtime.Object{snapshot},
			vm:        newVm("spx-other-project"),
			wantVmGet: true,
			wantErr:   func(err error) bool { return errors.Is(err, ErrRestoreSourceMissing) },
		},
		{
			name:    "snapshot not found",
			wantErr: apierrors.IsNotFound,
		},
		{
			name:      "source VM lookup fails",
			objects:   []runtime.Object{snapshot},
			vmErr:     errLookup,
			wantVmGet: true,
			wantErr:   func(err error) bool { return errors.Is(err, errLookup) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			client := kubecli.NewMockKubevirtClient(ctrl)
			vmIface := kubecli.NewMockVirtualMachineInterface(ctrl)

			scheme := runtime.NewScheme()
			if err := v1beta1.AddToScheme(scheme); err != nil {
				t.Fatalf("failed to build scheme: %v", err)
			}
			tracker := k8stesting.NewObjectTracker(scheme, serializer.NewCodecFactory(scheme).UniversalDecoder())
			for _, obj := range tt.objects {
				if err := tracker.Add(obj); err != nil {
					t.Fatalf("failed to add object: %v", err)
				}
			}
			fake := &k8stesting.Fake{}
			fake.AddReactor("*", "*", k8stesting.ObjectReaction(tracker))
			fakeSnapshot := &snapshotfake.FakeSnapshotV1beta1{Fake: fake}

			orig := config.VirtClient
			t.Cleanup(func() { config.VirtClient = orig })
			config.VirtClient = client

			client.EXPECT().VirtualMachineSnapshot(namespace).Return(fakeSnapshot.VirtualMachineSnapshots(namespace)).AnyTimes()
			client.EXPECT().VirtualMachineRestore(namespace).Return(fakeSnapshot.VirtualMachineRestores(namespace)).AnyTimes()
			if tt.wantVmGet {
				client.EXPECT().VirtualMachine(namespace).Return(vmIface)
				vmIface.EXPECT().Get(gomock.Any(), restoreTestSourceVm, gomock.Any()).Return(tt.vm, tt.vmErr)
			}

			err := RestoreVmSnapshot(context.Background(), restoreTestOrgId, restoreTestProjectId, restoreTestSnapshot)

			switch {
			case tt.wantErr == nil && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case tt.wantErr != nil && (err == nil || !tt.wantErr(err)):
				t.Fatalf("unexpected error value: %v", err)
			}

			restores, err := fakeSnapshot.VirtualMachineRestores(namespace).List(context.Background(), k8smetav1.ListOptions{})
			if err != nil {
				t.Fatalf("failed to list restores: %v", err)
			}
			if got := len(restores.Items) > 0; got != tt.wantRestore {
				t.Fatalf("restore created = %t, want %t", got, tt.wantRestore)
			}
			if tt.wantRestore {
				if target := restores.Items[0].Spec.Target.Name; target != restoreTestSourceVm {
					t.Errorf("restore target = %q, want %q", target, restoreTestSourceVm)
				}
				if got := restores.Items[0].Spec.VirtualMachineSnapshotName; got != restoreTestSnapshot {
					t.Errorf("restore snapshot = %q, want %q", got, restoreTestSnapshot)
				}
			}
		})
	}
}
