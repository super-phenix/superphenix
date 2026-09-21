package view

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	v1 "kubevirt.io/api/core/v1"
)

func TestVMIToView_LinkState(t *testing.T) {
	tests := []struct {
		name               string
		vmi                v1.VirtualMachineInstance
		expectedInterfaces []VirtualMachineInstanceNetworkInterface
	}{
		{
			name: "single interface with link up",
			vmi: v1.VirtualMachineInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vmi-1",
					Namespace: "default",
				},
				Status: v1.VirtualMachineInstanceStatus{
					Interfaces: []v1.VirtualMachineInstanceNetworkInterface{
						{
							Name:      "net0",
							MAC:       "52:54:00:12:34:56",
							IPs:       []string{"192.168.1.10"},
							LinkState: "up",
						},
					},
				},
			},
			expectedInterfaces: []VirtualMachineInstanceNetworkInterface{
				{
					Name:      "net0",
					MAC:       "52:54:00:12:34:56",
					IPs:       []string{"192.168.1.10"},
					LinkState: "up",
				},
			},
		},
		{
			name: "single interface with link down",
			vmi: v1.VirtualMachineInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vmi-2",
					Namespace: "default",
				},
				Status: v1.VirtualMachineInstanceStatus{
					Interfaces: []v1.VirtualMachineInstanceNetworkInterface{
						{
							Name:      "net0",
							MAC:       "52:54:00:12:34:56",
							IPs:       []string{"192.168.1.10"},
							LinkState: "down",
						},
					},
				},
			},
			expectedInterfaces: []VirtualMachineInstanceNetworkInterface{
				{
					Name:      "net0",
					MAC:       "52:54:00:12:34:56",
					IPs:       []string{"192.168.1.10"},
					LinkState: "down",
				},
			},
		},
		{
			name: "multiple interfaces with mixed link states",
			vmi: v1.VirtualMachineInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vmi-3",
					Namespace: "default",
				},
				Status: v1.VirtualMachineInstanceStatus{
					Interfaces: []v1.VirtualMachineInstanceNetworkInterface{
						{
							Name:      "net0",
							MAC:       "52:54:00:12:34:56",
							IPs:       []string{"192.168.1.10"},
							LinkState: "up",
						},
						{
							Name:      "net1",
							MAC:       "52:54:00:78:9a:bc",
							IPs:       []string{"10.0.0.5"},
							LinkState: "down",
						},
						{
							Name: "net2",
							MAC:  "52:54:00:de:f0:12",
							IPs:  []string{"172.16.0.2"},
						},
					},
				},
			},
			expectedInterfaces: []VirtualMachineInstanceNetworkInterface{
				{
					Name:      "net0",
					MAC:       "52:54:00:12:34:56",
					IPs:       []string{"192.168.1.10"},
					LinkState: "up",
				},
				{
					Name:      "net1",
					MAC:       "52:54:00:78:9a:bc",
					IPs:       []string{"10.0.0.5"},
					LinkState: "down",
				},
				{
					Name: "net2",
					MAC:  "52:54:00:de:f0:12",
					IPs:  []string{"172.16.0.2"},
				},
			},
		},
		{
			name: "empty interfaces",
			vmi: v1.VirtualMachineInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vmi-empty",
					Namespace: "default",
				},
				Status: v1.VirtualMachineInstanceStatus{
					Interfaces: nil,
				},
			},
			expectedInterfaces: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := VMIToView(tt.vmi)
			if !reflect.DeepEqual(tt.expectedInterfaces, result.Status.Interfaces) {
				t.Errorf("expected interfaces %+v, got %+v", tt.expectedInterfaces, result.Status.Interfaces)
			}
		})
	}
}

func TestUnstructuredVMIToView_LinkState(t *testing.T) {
	tests := []struct {
		name               string
		unstructured       *unstructured.Unstructured
		expectedInterfaces []VirtualMachineInstanceNetworkInterface
	}{
		{
			name: "unstructured with link down",
			unstructured: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "kubevirt.io/v1",
					"kind":       "VirtualMachineInstance",
					"metadata": map[string]interface{}{
						"name":      "test-vmi-unstructured-1",
						"namespace": "default",
					},
					"status": map[string]interface{}{
						"interfaces": []interface{}{
							map[string]interface{}{
								"name":        "net0",
								"mac":         "52:54:00:12:34:56",
								"ipAddresses": []interface{}{"192.168.1.10"},
								"linkState":   "down",
							},
						},
					},
				},
			},
			expectedInterfaces: []VirtualMachineInstanceNetworkInterface{
				{
					Name:      "net0",
					MAC:       "52:54:00:12:34:56",
					IPs:       []string{"192.168.1.10"},
					LinkState: "down",
				},
			},
		},
		{
			name: "unstructured with multiple mixed link states",
			unstructured: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "kubevirt.io/v1",
					"kind":       "VirtualMachineInstance",
					"metadata": map[string]interface{}{
						"name":      "test-vmi-unstructured-2",
						"namespace": "default",
					},
					"status": map[string]interface{}{
						"interfaces": []interface{}{
							map[string]interface{}{
								"name":        "net0",
								"mac":         "52:54:00:12:34:56",
								"ipAddresses": []interface{}{"192.168.1.10"},
								"linkState":   "up",
							},
							map[string]interface{}{
								"name":        "net1",
								"mac":         "52:54:00:ab:cd:ef",
								"ipAddresses": []interface{}{"10.0.0.8"},
								"linkState":   "down",
							},
						},
					},
				},
			},
			expectedInterfaces: []VirtualMachineInstanceNetworkInterface{
				{
					Name:      "net0",
					MAC:       "52:54:00:12:34:56",
					IPs:       []string{"192.168.1.10"},
					LinkState: "up",
				},
				{
					Name:      "net1",
					MAC:       "52:54:00:ab:cd:ef",
					IPs:       []string{"10.0.0.8"},
					LinkState: "down",
				},
			},
		},
		{
			name: "unstructured with interface omitting linkState",
			unstructured: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "kubevirt.io/v1",
					"kind":       "VirtualMachineInstance",
					"metadata": map[string]interface{}{
						"name":      "test-vmi-unstructured-3",
						"namespace": "default",
					},
					"status": map[string]interface{}{
						"interfaces": []interface{}{
							map[string]interface{}{
								"name":        "net0",
								"mac":         "52:54:00:12:34:56",
								"ipAddresses": []interface{}{"192.168.1.10"},
							},
						},
					},
				},
			},
			expectedInterfaces: []VirtualMachineInstanceNetworkInterface{
				{
					Name: "net0",
					MAC:  "52:54:00:12:34:56",
					IPs:  []string{"192.168.1.10"},
				},
			},
		},
		{
			name: "unstructured with nil interfaces",
			unstructured: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "kubevirt.io/v1",
					"kind":       "VirtualMachineInstance",
					"metadata": map[string]interface{}{
						"name":      "test-vmi-unstructured-empty",
						"namespace": "default",
					},
					"status": map[string]interface{}{},
				},
			},
			expectedInterfaces: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := UnstructuredVMIToView(tt.unstructured)
			if !reflect.DeepEqual(tt.expectedInterfaces, result.Status.Interfaces) {
				t.Errorf("expected interfaces %+v, got %+v", tt.expectedInterfaces, result.Status.Interfaces)
			}
		})
	}
}
