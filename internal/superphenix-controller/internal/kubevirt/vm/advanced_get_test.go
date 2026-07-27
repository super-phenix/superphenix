package vm

import (
	"context"
	"errors"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	"go.uber.org/mock/gomock"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	v1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
)

// vmiNotFound builds a K8s NotFound error for a VMI (returned when the VM is stopped).
func vmiNotFound(name string) error {
	return k8serrors.NewNotFound(schema.GroupResource{Group: "kubevirt.io", Resource: "virtualmachineinstances"}, name)
}

// projectVM builds a VM that passes utils.CheckProjectLabel for the namespace.
func projectVM(namespace, name string, d v1.DomainSpec) *v1.VirtualMachine {
	return &v1.VirtualMachine{
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{spxId.SpxLabelProjectID: namespace},
		},
		Spec: v1.VirtualMachineSpec{
			Template: &v1.VirtualMachineInstanceTemplateSpec{
				Spec: v1.VirtualMachineInstanceSpec{Domain: d},
			},
		},
	}
}

// mockVirtClient installs a mock KubevirtClient into config.VirtClient for the
// duration of the test and returns the per-resource mocks.
func mockVirtClient(t *testing.T) (*kubecli.MockKubevirtClient, *kubecli.MockVirtualMachineInterface, *kubecli.MockExpandSpecInterface, *kubecli.MockVirtualMachineInstanceInterface) {
	t.Helper()
	ctrl := gomock.NewController(t)
	client := kubecli.NewMockKubevirtClient(ctrl)
	vmIface := kubecli.NewMockVirtualMachineInterface(ctrl)
	expandIface := kubecli.NewMockExpandSpecInterface(ctrl)
	vmiIface := kubecli.NewMockVirtualMachineInstanceInterface(ctrl)

	orig := config.VirtClient
	t.Cleanup(func() { config.VirtClient = orig })
	config.VirtClient = client

	return client, vmIface, expandIface, vmiIface
}

func TestGetAdvancedOptions(t *testing.T) {
	ctx := context.Background()
	const ns, name = "proj-ns", "vm-1"

	t.Run("running vm resolves live values", func(t *testing.T) {
		client, vmIface, _, vmiIface := mockVirtClient(t)

		// Saved spec forces persistent off; the running VMI still has it on.
		raw := projectVM(ns, name, v1.DomainSpec{
			Devices: v1.Devices{TPM: &v1.TPMDevice{Enabled: boolPtr(true), Persistent: boolPtr(false)}},
		})
		expanded := projectVM(ns, name, v1.DomainSpec{
			Devices: v1.Devices{TPM: &v1.TPMDevice{Enabled: boolPtr(true), Persistent: boolPtr(false)}},
		})
		vmi := advDomainVMI(v1.DomainSpec{
			Devices: v1.Devices{TPM: &v1.TPMDevice{Enabled: boolPtr(true), Persistent: boolPtr(true)}},
		})

		client.EXPECT().VirtualMachine(ns).Return(vmIface).Times(2)
		vmIface.EXPECT().Get(gomock.Any(), name, gomock.Any()).Return(raw, nil)
		vmIface.EXPECT().GetWithExpandedSpec(gomock.Any(), name).Return(expanded, nil)
		client.EXPECT().VirtualMachineInstance(ns).Return(vmiIface)
		vmiIface.EXPECT().Get(gomock.Any(), name, gomock.Any()).Return(vmi, nil)

		got, err := GetAdvancedOptions(ctx, ns, name)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		p := advLeaves(got)["tpm.persistent"]
		if p.Value == nil || *p.Value || p.Live == nil || !*p.Live || !p.Stale {
			t.Fatalf("expected tpm.persistent {value:false, live:true, stale:true}, got %+v", p)
		}
	})

	t.Run("stopped vm has no live values", func(t *testing.T) {
		client, vmIface, _, vmiIface := mockVirtClient(t)

		raw := projectVM(ns, name, v1.DomainSpec{
			Devices: v1.Devices{TPM: &v1.TPMDevice{Enabled: boolPtr(false)}},
		})
		expanded := projectVM(ns, name, v1.DomainSpec{
			Devices: v1.Devices{TPM: &v1.TPMDevice{Enabled: boolPtr(false)}},
		})

		client.EXPECT().VirtualMachine(ns).Return(vmIface).Times(2)
		vmIface.EXPECT().Get(gomock.Any(), name, gomock.Any()).Return(raw, nil)
		vmIface.EXPECT().GetWithExpandedSpec(gomock.Any(), name).Return(expanded, nil)
		client.EXPECT().VirtualMachineInstance(ns).Return(vmiIface)
		vmiIface.EXPECT().Get(gomock.Any(), name, gomock.Any()).Return(nil, vmiNotFound(name))

		got, err := GetAdvancedOptions(ctx, ns, name)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if te := advLeaves(got)["tpm.enabled"]; te.Live != nil || te.Stale {
			t.Fatalf("expected no live values for stopped vm, got %+v", te)
		}
	})

	t.Run("with fake preference resolves blocks from the instance type", func(t *testing.T) {
		client, vmIface, _, vmiIface := mockVirtClient(t)

		raw := projectVM(ns, name, v1.DomainSpec{})
		// Instance type adds EFI (secure boot on) and a persistent vTPM.
		expanded := projectVM(ns, name, v1.DomainSpec{
			Devices:  v1.Devices{TPM: &v1.TPMDevice{Persistent: boolPtr(true)}},
			Firmware: &v1.Firmware{Bootloader: &v1.Bootloader{EFI: &v1.EFI{SecureBoot: boolPtr(true)}}},
		})

		client.EXPECT().VirtualMachine(ns).Return(vmIface).Times(2)
		vmIface.EXPECT().Get(gomock.Any(), name, gomock.Any()).Return(raw, nil)
		vmIface.EXPECT().GetWithExpandedSpec(gomock.Any(), name).Return(expanded, nil)
		client.EXPECT().VirtualMachineInstance(ns).Return(vmiIface)
		vmiIface.EXPECT().Get(gomock.Any(), name, gomock.Any()).Return(nil, vmiNotFound(name))

		got, err := GetAdvancedOptions(ctx, ns, name)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		leaves := advLeaves(got)
		if e := leaves["efi.enabled"]; e.Source != view.SourcePreference || e.Value == nil || !*e.Value {
			t.Fatalf("expected efi.enabled true/preference, got %+v", e)
		}
		if sb := leaves["efi.secureBoot"]; sb.Source != view.SourcePreference || sb.Value == nil || !*sb.Value {
			t.Fatalf("expected secureBoot true/preference, got %+v", sb)
		}
		if tp := leaves["tpm.enabled"]; tp.Source != view.SourcePreference || tp.Value == nil || !*tp.Value {
			t.Fatalf("expected tpm.enabled true/preference, got %+v", tp)
		}
	})

	t.Run("without preference resolves kubevirt defaults", func(t *testing.T) {
		client, vmIface, _, vmiIface := mockVirtClient(t)

		raw := projectVM(ns, name, v1.DomainSpec{})
		expanded := projectVM(ns, name, v1.DomainSpec{})

		client.EXPECT().VirtualMachine(ns).Return(vmIface).Times(2)
		vmIface.EXPECT().Get(gomock.Any(), name, gomock.Any()).Return(raw, nil)
		vmIface.EXPECT().GetWithExpandedSpec(gomock.Any(), name).Return(expanded, nil)
		client.EXPECT().VirtualMachineInstance(ns).Return(vmiIface)
		vmiIface.EXPECT().Get(gomock.Any(), name, gomock.Any()).Return(nil, vmiNotFound(name))

		got, err := GetAdvancedOptions(ctx, ns, name)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		leaves := advLeaves(got)
		for _, key := range []string{"efi.enabled", "efi.secureBoot", "tpm.enabled"} {
			rb := leaves[key]
			if rb.Source != view.SourceDefault || rb.Value == nil || *rb.Value {
				t.Fatalf("expected %s false/default, got %+v", key, rb)
			}
		}
	})

	t.Run("get error is propagated", func(t *testing.T) {
		client, vmIface, _, _ := mockVirtClient(t)

		client.EXPECT().VirtualMachine(ns).Return(vmIface)
		vmIface.EXPECT().Get(gomock.Any(), name, gomock.Any()).Return(nil, errors.New("boom"))

		if _, err := GetAdvancedOptions(ctx, ns, name); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("expand error is propagated", func(t *testing.T) {
		client, vmIface, _, _ := mockVirtClient(t)

		raw := projectVM(ns, name, v1.DomainSpec{})
		client.EXPECT().VirtualMachine(ns).Return(vmIface).Times(2)
		vmIface.EXPECT().Get(gomock.Any(), name, gomock.Any()).Return(raw, nil)
		vmIface.EXPECT().GetWithExpandedSpec(gomock.Any(), name).Return(nil, errors.New("expand failed"))

		if _, err := GetAdvancedOptions(ctx, ns, name); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("vmi error other than not-found is propagated", func(t *testing.T) {
		client, vmIface, _, vmiIface := mockVirtClient(t)

		raw := projectVM(ns, name, v1.DomainSpec{})
		expanded := projectVM(ns, name, v1.DomainSpec{})
		client.EXPECT().VirtualMachine(ns).Return(vmIface).Times(2)
		vmIface.EXPECT().Get(gomock.Any(), name, gomock.Any()).Return(raw, nil)
		vmIface.EXPECT().GetWithExpandedSpec(gomock.Any(), name).Return(expanded, nil)
		client.EXPECT().VirtualMachineInstance(ns).Return(vmiIface)
		vmiIface.EXPECT().Get(gomock.Any(), name, gomock.Any()).Return(nil, errors.New("api down"))

		if _, err := GetAdvancedOptions(ctx, ns, name); err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestGetPreferenceAdvancedOptions(t *testing.T) {
	ctx := context.Background()
	const ns, pref = "proj-ns", "windows.2k25.virtio"

	client, _, expandIface, _ := mockVirtClient(t)

	client.EXPECT().ExpandSpec(ns).Return(expandIface)
	expandIface.EXPECT().ForVirtualMachine(gomock.Any()).DoAndReturn(
		func(vm *v1.VirtualMachine) (*v1.VirtualMachine, error) {
			if vm.Spec.Preference == nil || vm.Spec.Preference.Name != pref {
				t.Fatalf("expected stub VM carrying preference %q, got %+v", pref, vm.Spec.Preference)
			}
			return &v1.VirtualMachine{
				Spec: v1.VirtualMachineSpec{
					Template: &v1.VirtualMachineInstanceTemplateSpec{
						Spec: v1.VirtualMachineInstanceSpec{
							Domain: v1.DomainSpec{
								Devices: v1.Devices{TPM: &v1.TPMDevice{Persistent: boolPtr(true)}},
							},
						},
					},
				},
			}, nil
		},
	)

	got, err := GetPreferenceAdvancedOptions(ctx, ns, pref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p := advLeaves(got)["tpm.persistent"]; p.Source != view.SourcePreference ||
		p.Value == nil || !*p.Value {
		t.Fatalf("expected tpm.persistent {true, preference}, got %+v", advLeaves(got)["tpm.persistent"])
	}
}
