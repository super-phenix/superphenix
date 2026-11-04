package vm

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
	v1 "kubevirt.io/api/core/v1"
)

func TestWithSSHKeys(t *testing.T) {
	const namespace = "spx-project"

	idx := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{testNSIndex: cache.MetaNamespaceIndexFunc})
	for _, name := range []string{"live-1", "live-2"} {
		if err := idx.Add(&unstructured.Unstructured{Object: map[string]any{
			"metadata": map[string]any{"name": name, "namespace": namespace},
			"data":     map[string]any{"spxSshKey": "c3NoLXJzYSBBQUE="},
		}}); err != nil {
			t.Fatalf("add ssh key: %v", err)
		}
	}
	old, had := informers.WatcherSet[informers.SSH]
	informers.WatcherSet[informers.SSH] = informers.Watcher{Indexer: idx}
	t.Cleanup(func() {
		if had {
			informers.WatcherSet[informers.SSH] = old
		} else {
			delete(informers.WatcherSet, informers.SSH)
		}
	})

	tests := []struct {
		name        string
		current     []string
		requested   []string
		want        []string
		errContains string
	}{
		{name: "new vm with live keys", requested: []string{"live-1", "live-2"}, want: []string{"live-1", "live-2"}},
		{name: "new vm with a missing key", requested: []string{"dead"}, errContains: "ssh key dead not found"},
		{name: "update keeps live keys and their order", current: []string{"live-1", "live-2"}, requested: []string{"live-2", "live-1"}, want: []string{"live-2", "live-1"}},
		{name: "update drops a dead key of the vm", current: []string{"dead", "live-1"}, requested: []string{"dead", "live-1"}, want: []string{"live-1"}},
		{name: "update with only a dead key", current: []string{"dead"}, requested: []string{"dead"}, want: []string{}},
		{name: "update adding a missing key", current: []string{"live-1"}, requested: []string{"live-1", "dead"}, errContains: "ssh key dead not found"},
		{name: "update without keys", current: []string{"dead", "live-1"}, want: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := &v1.VirtualMachine{Spec: v1.VirtualMachineSpec{Template: &v1.VirtualMachineInstanceTemplateSpec{}}}
			for _, key := range tt.current {
				vm.Spec.Template.Spec.AccessCredentials = append(vm.Spec.Template.Spec.AccessCredentials, v1.AccessCredential{
					SSHPublicKey: &v1.SSHPublicKeyAccessCredential{
						Source: v1.SSHPublicKeyAccessCredentialSource{Secret: &v1.AccessCredentialSecretSource{SecretName: key}},
					},
				})
			}

			err := withSSHKeys(context.Background(), namespace, tt.requested, vm)
			if tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("expected error containing %q, got %v", tt.errContains, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("withSSHKeys: %v", err)
			}

			got := make([]string, 0)
			for _, credential := range vm.Spec.Template.Spec.AccessCredentials {
				got = append(got, credential.SSHPublicKey.Source.Secret.SecretName)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("keys = %v, want %v", got, tt.want)
			}
		})
	}
}
