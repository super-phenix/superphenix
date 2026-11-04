package ssh

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
)

const testNamespace = "spx-project"

func testVM(namespace, name, displayName string, keys ...string) *unstructured.Unstructured {
	credentials := make([]any, 0, len(keys))
	for _, key := range keys {
		credentials = append(credentials, map[string]any{
			"sshPublicKey": map[string]any{
				"propagationMethod": map[string]any{"noCloud": map[string]any{}},
				"source":            map[string]any{"secret": map[string]any{"secretName": key}},
			},
		})
	}
	metadata := map[string]any{"name": name, "namespace": namespace}
	if displayName != "" {
		metadata["labels"] = map[string]any{spxId.SpxLabelResourceName: displayName}
	}
	return &unstructured.Unstructured{Object: map[string]any{
		"metadata": metadata,
		"spec": map[string]any{
			"runStrategy": "Halted",
			"template":    map[string]any{"spec": map[string]any{"domain": map[string]any{}, "accessCredentials": credentials}},
		},
	}}
}

func setVMWatcher(t *testing.T, indexers cache.Indexers, vms ...*unstructured.Unstructured) {
	t.Helper()
	idx := cache.NewIndexer(cache.MetaNamespaceKeyFunc, indexers)
	for _, vm := range vms {
		if err := idx.Add(vm); err != nil {
			t.Fatalf("add vm: %v", err)
		}
	}
	old, had := informers.WatcherSet[informers.VirtualMachine]
	informers.WatcherSet[informers.VirtualMachine] = informers.Watcher{Indexer: idx}
	t.Cleanup(func() {
		if had {
			informers.WatcherSet[informers.VirtualMachine] = old
		} else {
			delete(informers.WatcherSet, informers.VirtualMachine)
		}
	})
}

func namespaceIndexers() cache.Indexers {
	return cache.Indexers{"namespace": cache.MetaNamespaceIndexFunc}
}

func TestUsage(t *testing.T) {
	tests := []struct {
		name string
		vms  []*unstructured.Unstructured
		want map[string][]view.SSHUser
	}{
		{name: "no vm", want: map[string][]view.SSHUser{}},
		{
			name: "vm without keys",
			vms:  []*unstructured.Unstructured{testVM(testNamespace, "vm-1", "")},
			want: map[string][]view.SSHUser{},
		},
		{
			name: "several vms and keys, sorted by eid",
			vms: []*unstructured.Unstructured{
				testVM(testNamespace, "vm-2", "gitops-vm", "key-1"),
				testVM(testNamespace, "vm-1", "", "key-1", "key-2"),
			},
			want: map[string][]view.SSHUser{
				"key-1": {{EId: "vm-1"}, {EId: "vm-2", ProductName: "gitops-vm"}},
				"key-2": {{EId: "vm-1"}},
			},
		},
		{
			name: "vm of another namespace",
			vms:  []*unstructured.Unstructured{testVM("spx-other", "vm-1", "", "key-1")},
			want: map[string][]view.SSHUser{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setVMWatcher(t, namespaceIndexers(), tt.vms...)

			got, err := Usage(testNamespace)
			if err != nil {
				t.Fatalf("Usage: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Usage = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeleteSSHKey(t *testing.T) {
	const key = "key-1"

	tests := []struct {
		name        string
		vms         []*unstructured.Unstructured
		indexers    cache.Indexers
		wantErr     func(error) bool
		wantDeleted bool
	}{
		{name: "unused key", wantDeleted: true},
		{
			name:        "used by a vm of another namespace",
			vms:         []*unstructured.Unstructured{testVM("spx-other", "vm-1", "", key)},
			wantDeleted: true,
		},
		{
			name:        "another key used",
			vms:         []*unstructured.Unstructured{testVM(testNamespace, "vm-1", "", "key-2")},
			wantDeleted: true,
		},
		{
			name:    "used by a stopped vm",
			vms:     []*unstructured.Unstructured{testVM(testNamespace, "vm-1", "", key)},
			wantErr: func(err error) bool { return errors.Is(err, ErrSSHKeyInUse) },
		},
		{
			name:    "one of several keys of a vm",
			vms:     []*unstructured.Unstructured{testVM(testNamespace, "vm-1", "", "key-2", key)},
			wantErr: func(err error) bool { return errors.Is(err, ErrSSHKeyInUse) },
		},
		{
			name:     "vm listing failure keeps the key",
			indexers: cache.Indexers{},
			wantErr:  func(err error) bool { return err != nil && !errors.Is(err, ErrSSHKeyInUse) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			indexers := tt.indexers
			if indexers == nil {
				indexers = namespaceIndexers()
			}
			setVMWatcher(t, indexers, tt.vms...)

			orig := config.K8sClient
			t.Cleanup(func() { config.K8sClient = orig })
			config.K8sClient = fake.NewSimpleClientset(&corev1.Secret{ObjectMeta: metav1.ObjectMeta{
				Name:      key,
				Namespace: testNamespace,
				Labels:    map[string]string{spxId.SpxLabelProjectID: testNamespace},
			}})

			err := DeleteSSHKey(context.Background(), testNamespace, key)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("DeleteSSHKey: %v", err)
			}
			if tt.wantErr != nil && !tt.wantErr(err) {
				t.Fatalf("unexpected error: %v", err)
			}

			_, getErr := config.K8sClient.CoreV1().Secrets(testNamespace).Get(context.Background(), key, metav1.GetOptions{})
			if deleted := apierrors.IsNotFound(getErr); deleted != tt.wantDeleted {
				t.Errorf("deleted = %v, want %v", deleted, tt.wantDeleted)
			}
		})
	}
}
