package vm

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
	v2 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
)

func newVirtualMachine(name, namespace string, labels map[string]string) *v2.VirtualMachine {
	return &v2.VirtualMachine{
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: v2.VirtualMachineSpec{
			Template: &v2.VirtualMachineInstanceTemplateSpec{
				ObjectMeta: k8smetav1.ObjectMeta{
					Labels: labels,
				},
			},
		},
	}
}

func TestClean(t *testing.T) {
	labelMarkKey := "superphenix.net/markedForDeletion"
	pastTimestamp := time.Now().Add(-1 * time.Hour).Format(utils.TimestampFormat)
	futureTimestamp := time.Now().Add(1 * time.Hour).Format(utils.TimestampFormat)

	tests := []struct {
		name        string
		vms         []*v2.VirtualMachine
		debug       bool
		wantErr     bool
		wantDeleted []string
	}{
		{
			name:        "no marked VMs",
			vms:         []*v2.VirtualMachine{},
			wantErr:     false,
			wantDeleted: nil,
		},
		{
			name: "delete expired VM",
			vms: []*v2.VirtualMachine{
				newVirtualMachine("vm-expired", "ns1", map[string]string{labelMarkKey: pastTimestamp}),
			},
			wantErr:     false,
			wantDeleted: []string{"vm-expired"},
		},
		{
			name: "skip future VM",
			vms: []*v2.VirtualMachine{
				newVirtualMachine("vm-future", "ns1", map[string]string{labelMarkKey: futureTimestamp}),
			},
			wantErr:     false,
			wantDeleted: nil,
		},
		{
			name: "debug mode skips deletion",
			vms: []*v2.VirtualMachine{
				newVirtualMachine("vm-debug", "ns1", map[string]string{labelMarkKey: pastTimestamp}),
			},
			debug:       true,
			wantErr:     false,
			wantDeleted: nil,
		},
		{
			name: "mixed expired and future",
			vms: []*v2.VirtualMachine{
				newVirtualMachine("vm-expired", "ns1", map[string]string{labelMarkKey: pastTimestamp}),
				newVirtualMachine("vm-future", "ns1", map[string]string{labelMarkKey: futureTimestamp}),
			},
			wantErr:     false,
			wantDeleted: []string{"vm-expired"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.Global.GarbageCollection.LabelMarkKey = labelMarkKey
			config.Global.GarbageCollection.Debug = tt.debug

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := kubecli.NewMockKubevirtClient(ctrl)
			config.VirtClient = mockClient

			watcherObjs := make([]interface{}, len(tt.vms))
			for i, vm := range tt.vms {
				watcherObjs[i] = vm
			}
			testhelper.SetupFakeWatcher(informers.VirtualMachine, watcherObjs...)

			if len(tt.wantDeleted) > 0 {
				for _, name := range tt.wantDeleted {
					mockVMInterface := kubecli.NewMockVirtualMachineInterface(ctrl)
					mockClient.EXPECT().VirtualMachine(gomock.Any()).Return(mockVMInterface)
					mockVMInterface.EXPECT().Delete(gomock.Any(), name, gomock.Any()).Return(nil)
				}
			}

			c := &Cleaner{
				ResourceType: "Virtual Machine",
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
		vms       []*v2.VirtualMachine
		wantErr   bool
		wantCount int
	}{
		{
			name: "mark matching VMs",
			vms: []*v2.VirtualMachine{
				newVirtualMachine("vm-1", projectNs, map[string]string{spxId.SpxLabelProjectID: projectNs}),
			},
			wantErr:   false,
			wantCount: 1,
		},
		{
			name:      "no VMs in namespace",
			vms:       []*v2.VirtualMachine{},
			wantErr:   false,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.Global.GarbageCollection.LabelMarkKey = labelMarkKey

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := kubecli.NewMockKubevirtClient(ctrl)
			config.VirtClient = mockClient

			watcherObjs := make([]interface{}, len(tt.vms))
			for i, vm := range tt.vms {
				watcherObjs[i] = vm
			}
			testhelper.SetupFakeWatcher(informers.VirtualMachine, watcherObjs...)

			if tt.wantCount > 0 {
				for range tt.vms {
					mockVMInterface := kubecli.NewMockVirtualMachineInterface(ctrl)
					mockClient.EXPECT().VirtualMachine(gomock.Any()).Return(mockVMInterface)
					mockVMInterface.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
						func(ctx context.Context, vm *v2.VirtualMachine, opts k8smetav1.UpdateOptions) (*v2.VirtualMachine, error) {
							if vm.Labels[labelMarkKey] != timestamp {
								t.Errorf("Mark() label = %v, want %v", vm.Labels[labelMarkKey], timestamp)
							}
							return vm, nil
						},
					)
				}
			}

			c := &Cleaner{
				ResourceType: "Virtual Machine",
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
			resourceType: "Virtual Machine",
			want:         "Something went wrong during Virtual Machine cleaning.",
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
