package dbaas

import (
	"reflect"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const testEIP = "203.0.113.10"

func TestListEndpoints(t *testing.T) {
	tests := []struct {
		name string
		lbs  []*unstructured.Unstructured
		eips []*unstructured.Unstructured
		want []view.DBaaSEndpoint
	}{
		{
			name: "load balancer only",
			lbs:  []*unstructured.Unstructured{newLBRule("ns", testLocalId, 5432, 5434)},
			want: []view.DBaaSEndpoint{
				{Role: "rw", Host: testVIP, Port: 5432},
				{Role: "r", Host: testVIP, Port: 5434},
			},
		},
		{
			name: "load balancer and eip",
			lbs:  []*unstructured.Unstructured{newLBRule("ns", testLocalId, 5432, 5434)},
			eips: []*unstructured.Unstructured{newEIP("ns", testLocalId, testEIP)},
			want: []view.DBaaSEndpoint{
				{Role: "rw", Host: testVIP, Port: 5432},
				{Role: "r", Host: testVIP, Port: 5434},
				{Role: "rw", Host: testEIP, Port: 5432, External: true},
				{Role: "r", Host: testEIP, Port: 5434, External: true},
			},
		},
		{
			name: "eip without address",
			lbs:  []*unstructured.Unstructured{newLBRule("ns", testLocalId, 5432)},
			eips: []*unstructured.Unstructured{newEIP("ns", testLocalId, "")},
			want: []view.DBaaSEndpoint{{Role: "rw", Host: testVIP, Port: 5432}},
		},
		{
			name: "eip of another database",
			lbs:  []*unstructured.Unstructured{newLBRule("ns", testLocalId, 5432)},
			eips: []*unstructured.Unstructured{newEIP("ns", testOtherLocalId, testEIP)},
			want: []view.DBaaSEndpoint{{Role: "rw", Host: testVIP, Port: 5432}},
		},
		{
			name: "eip of another project",
			lbs:  []*unstructured.Unstructured{newLBRule("ns", testLocalId, 5432)},
			eips: []*unstructured.Unstructured{newEIP("other", testLocalId, testEIP)},
			want: []view.DBaaSEndpoint{{Role: "rw", Host: testVIP, Port: 5432}},
		},
		{
			name: "no load balancer",
			eips: []*unstructured.Unstructured{newEIP("ns", testLocalId, testEIP)},
			want: []view.DBaaSEndpoint{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setFakeWatcher(t, informers.SwitchLBRules, tt.lbs...)
			setFakeWatcher(t, informers.EIP, tt.eips...)

			got := listEndpoints(t.Context(), "ns", testLocalId)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("listEndpoints() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
