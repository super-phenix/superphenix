package vm

import (
	"context"
	"fmt"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
	v1 "kubevirt.io/api/core/v1"
)

func setFakeSubnetWatcher(t *testing.T, subnets ...*unstructured.Unstructured) {
	t.Helper()
	idx := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	for _, s := range subnets {
		if err := idx.Add(s); err != nil {
			t.Fatalf("failed to add subnet to indexer: %v", err)
		}
	}
	old, had := informers.WatcherSet[informers.Subnet]
	informers.WatcherSet[informers.Subnet] = informers.Watcher{Indexer: idx}
	t.Cleanup(func() {
		if had {
			informers.WatcherSet[informers.Subnet] = old
		} else {
			delete(informers.WatcherSet, informers.Subnet)
		}
	})
}

func newSubnetUnstructured(name, namespace, cidr string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kubeovn.io/v1",
			"kind":       "Subnet",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels": map[string]interface{}{
					spxId.SpxLabelProjectID: namespace,
				},
			},
			"spec": map[string]interface{}{
				"cidrBlock": cidr,
			},
		},
	}
}

func TestWithNetworks_InterfaceState(t *testing.T) {
	const namespace = "prj-test"
	subnetObj := newSubnetUnstructured("sub-1", namespace, "10.0.0.0/24")
	setFakeSubnetWatcher(t, subnetObj)

	tests := []struct {
		name      string
		enabled   *bool
		wantState v1.InterfaceState
	}{
		{
			name:      "nil enabled defaults to link up",
			enabled:   nil,
			wantState: v1.InterfaceStateLinkUp,
		},
		{
			name:      "explicit true enabled sets link up",
			enabled:   boolPtr(true),
			wantState: v1.InterfaceStateLinkUp,
		},
		{
			name:      "explicit false enabled sets link down",
			enabled:   boolPtr(false),
			wantState: v1.InterfaceStateLinkDown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := &v1.VirtualMachine{
				Spec: v1.VirtualMachineSpec{
					Template: &v1.VirtualMachineInstanceTemplateSpec{
						ObjectMeta: k8smetav1.ObjectMeta{
							Annotations: make(map[string]string),
						},
					},
				},
			}

			networks := []Network{
				{
					Order:     0,
					SubnetEId: "sub-1",
					Model:     "virtio",
					Enabled:   tt.enabled,
				},
			}

			err := withNetworks(context.Background(), namespace, networks, vm)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(vm.Spec.Template.Spec.Domain.Devices.Interfaces) != 1 {
				t.Fatalf("expected 1 interface, got %d", len(vm.Spec.Template.Spec.Domain.Devices.Interfaces))
			}

			gotState := vm.Spec.Template.Spec.Domain.Devices.Interfaces[0].State
			if gotState != tt.wantState {
				t.Errorf("expected interface state %q, got %q", tt.wantState, gotState)
			}
		})
	}
}

func TestWithNetworks_SpecPreservedWhenDisabled(t *testing.T) {
	const namespace = "prj-test"
	subnet1 := newSubnetUnstructured("sub-1", namespace, "10.0.0.0/24")
	subnet2 := newSubnetUnstructured("sub-2", namespace, "10.0.1.0/24")
	setFakeSubnetWatcher(t, subnet1, subnet2)

	tests := []struct {
		name        string
		networks    []Network
		wantStates  map[string]v1.InterfaceState
		wantDefault string
	}{
		{
			name: "primary disabled and secondary enabled",
			networks: []Network{
				{
					Order:     0,
					SubnetEId: "sub-1",
					Model:     "virtio",
					Enabled:   boolPtr(false),
					IPv4:      "10.0.0.10",
				},
				{
					Order:     1,
					SubnetEId: "sub-2",
					Model:     "e1000",
					Enabled:   boolPtr(true),
					IPv4:      "10.0.1.20",
				},
			},
			wantStates: map[string]v1.InterfaceState{
				"interface-0": v1.InterfaceStateLinkDown,
				"interface-1": v1.InterfaceStateLinkUp,
			},
			wantDefault: "interface-0",
		},
		{
			name: "primary enabled and secondary disabled",
			networks: []Network{
				{
					Order:     0,
					SubnetEId: "sub-1",
					Model:     "virtio",
					Enabled:   boolPtr(true),
				},
				{
					Order:     1,
					SubnetEId: "sub-2",
					Model:     "virtio",
					Enabled:   boolPtr(false),
				},
			},
			wantStates: map[string]v1.InterfaceState{
				"interface-0": v1.InterfaceStateLinkUp,
				"interface-1": v1.InterfaceStateLinkDown,
			},
			wantDefault: "interface-0",
		},
		{
			name: "primary omitted (nil) and secondary disabled",
			networks: []Network{
				{
					Order:     0,
					SubnetEId: "sub-1",
					Model:     "virtio",
					Enabled:   nil,
				},
				{
					Order:     1,
					SubnetEId: "sub-2",
					Model:     "virtio",
					Enabled:   boolPtr(false),
				},
			},
			wantStates: map[string]v1.InterfaceState{
				"interface-0": v1.InterfaceStateLinkUp,
				"interface-1": v1.InterfaceStateLinkDown,
			},
			wantDefault: "interface-0",
		},
		{
			name: "all interfaces disabled",
			networks: []Network{
				{
					Order:     0,
					SubnetEId: "sub-1",
					Model:     "virtio",
					Enabled:   boolPtr(false),
				},
				{
					Order:     1,
					SubnetEId: "sub-2",
					Model:     "virtio",
					Enabled:   boolPtr(false),
				},
			},
			wantStates: map[string]v1.InterfaceState{
				"interface-0": v1.InterfaceStateLinkDown,
				"interface-1": v1.InterfaceStateLinkDown,
			},
			wantDefault: "interface-0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := &v1.VirtualMachine{
				Spec: v1.VirtualMachineSpec{
					Template: &v1.VirtualMachineInstanceTemplateSpec{
						ObjectMeta: k8smetav1.ObjectMeta{
							Annotations: make(map[string]string),
						},
					},
				},
			}

			err := withNetworks(context.Background(), namespace, tt.networks, vm)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify interface count matches networks count
			if len(vm.Spec.Template.Spec.Domain.Devices.Interfaces) != len(tt.networks) {
				t.Errorf("expected %d interfaces, got %d", len(tt.networks), len(vm.Spec.Template.Spec.Domain.Devices.Interfaces))
			}

			// Verify network count matches networks count
			if len(vm.Spec.Template.Spec.Networks) != len(tt.networks) {
				t.Errorf("expected %d networks, got %d", len(tt.networks), len(vm.Spec.Template.Spec.Networks))
			}

			// Verify interface states
			for _, iface := range vm.Spec.Template.Spec.Domain.Devices.Interfaces {
				expectedState, ok := tt.wantStates[iface.Name]
				if !ok {
					t.Errorf("unexpected interface name %s", iface.Name)
					continue
				}
				if iface.State != expectedState {
					t.Errorf("interface %s: expected state %q, got %q", iface.Name, expectedState, iface.State)
				}
			}

			// Verify Multus network configurations and default network
			for _, net := range vm.Spec.Template.Spec.Networks {
				if net.NetworkSource.Multus == nil {
					t.Errorf("network %s is missing Multus configuration", net.Name)
					continue
				}
				isDefault := net.NetworkSource.Multus.Default
				if net.Name == tt.wantDefault && !isDefault {
					t.Errorf("network %s expected Default=true, got false", net.Name)
				} else if net.Name != tt.wantDefault && isDefault {
					t.Errorf("network %s expected Default=false, got true", net.Name)
				}
			}

			// Verify migration annotations and static IP annotations are preserved regardless of enabled
			for _, net := range tt.networks {
				liveMigKey := fmt.Sprintf("%s.%s.ovn.kubernetes.io/allow_live_migration", net.SubnetEId, namespace)
				if vm.Spec.Template.ObjectMeta.Annotations[liveMigKey] != "true" {
					t.Errorf("expected live migration annotation for subnet %s", net.SubnetEId)
				}
				if net.IPv4 != "" {
					ipKey := fmt.Sprintf("%s.%s.ovn.kubernetes.io/ip_address", net.SubnetEId, namespace)
					if vm.Spec.Template.ObjectMeta.Annotations[ipKey] != net.IPv4 {
						t.Errorf("expected ip annotation %q, got %q", net.IPv4, vm.Spec.Template.ObjectMeta.Annotations[ipKey])
					}
				}
			}
		})
	}
}
