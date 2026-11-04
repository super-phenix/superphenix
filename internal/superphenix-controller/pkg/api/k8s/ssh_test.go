package k8s

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/api/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"github.com/go-chi/chi/v5"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
)

const (
	testProjectId = "project"
	testKey       = "key-1"
)

func sshRequest(method string, params map[string]string) *http.Request {
	r := httptest.NewRequest(method, "/ssh", http.NoBody)
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func setWatcher(t *testing.T, resource string, objects ...*unstructured.Unstructured) {
	t.Helper()
	idx := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{"namespace": cache.MetaNamespaceIndexFunc})
	for _, object := range objects {
		if err := idx.Add(object); err != nil {
			t.Fatalf("add object: %v", err)
		}
	}
	old, had := informers.WatcherSet[resource]
	informers.WatcherSet[resource] = informers.Watcher{Indexer: idx}
	t.Cleanup(func() {
		if had {
			informers.WatcherSet[resource] = old
		} else {
			delete(informers.WatcherSet, resource)
		}
	})
}

func setSecret(t *testing.T, namespace string) {
	t.Helper()
	orig := config.K8sClient
	t.Cleanup(func() { config.K8sClient = orig })
	config.K8sClient = fake.NewSimpleClientset(&corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Name:      testKey,
		Namespace: namespace,
		Labels:    map[string]string{spxId.SpxLabelProjectID: namespace},
	}})
}

func vmUsingKey(namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "vm-1", "namespace": namespace},
		"spec": map[string]any{"template": map[string]any{"spec": map[string]any{
			"domain": map[string]any{},
			"accessCredentials": []any{map[string]any{
				"sshPublicKey": map[string]any{"source": map[string]any{"secret": map[string]any{"secretName": testKey}}},
			}},
		}}},
	}}
}

func TestDeleteSSH(t *testing.T) {
	namespace := utils.GetNamespace(testProjectId)

	tests := []struct {
		name     string
		vms      []*unstructured.Unstructured
		key      string
		wantCode int
		wantBody string
		// The instances of a key are only exposed by the usage endpoint
		absentBody string
	}{
		{name: "unused key", key: testKey, wantCode: http.StatusOK},
		{name: "key in use", key: testKey, vms: []*unstructured.Unstructured{vmUsingKey(namespace)}, wantCode: http.StatusConflict, wantBody: "SSH key is in use and cannot be deleted", absentBody: "vm-1"},
		{name: "unknown key", key: "unknown", wantCode: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setWatcher(t, informers.VirtualMachine, tt.vms...)
			setSecret(t, namespace)

			w := httptest.NewRecorder()
			deleteSSH(w, sshRequest(http.MethodDelete, map[string]string{"projectId": testProjectId, "effectiveId": tt.key}))

			if w.Code != tt.wantCode {
				t.Fatalf("code = %d, want %d (%s)", w.Code, tt.wantCode, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tt.wantBody) {
				t.Errorf("body = %s, want %q", w.Body.String(), tt.wantBody)
			}
			if tt.absentBody != "" && strings.Contains(w.Body.String(), tt.absentBody) {
				t.Errorf("body = %s leaks %q", w.Body.String(), tt.absentBody)
			}
		})
	}
}

func TestSSHIsUsed(t *testing.T) {
	namespace := utils.GetNamespace(testProjectId)

	tests := []struct {
		name string
		vms  []*unstructured.Unstructured
		want bool
	}{
		{name: "unused key"},
		{name: "key in use", vms: []*unstructured.Unstructured{vmUsingKey(namespace)}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setWatcher(t, informers.VirtualMachine, tt.vms...)
			setWatcher(t, informers.SSH, &unstructured.Unstructured{Object: map[string]any{
				"metadata": map[string]any{"name": testKey, "namespace": namespace},
				"data":     map[string]any{"spxSshKey": "c3NoLXJzYSBBQUE="},
			}})
			setSecret(t, namespace)

			w := httptest.NewRecorder()
			listSSHs(w, sshRequest(http.MethodGet, map[string]string{"projectId": testProjectId}))
			if w.Code != http.StatusOK {
				t.Fatalf("list code = %d (%s)", w.Code, w.Body.String())
			}
			var list []view.SSH
			if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
				t.Fatalf("unmarshal list: %v", err)
			}
			if len(list) != 1 || list[0].IsUsed != tt.want {
				t.Errorf("list = %+v, want isUsed %v", list, tt.want)
			}

			w = httptest.NewRecorder()
			getSSH(w, sshRequest(http.MethodGet, map[string]string{"projectId": testProjectId}), testKey)
			if w.Code != http.StatusOK {
				t.Fatalf("get code = %d (%s)", w.Code, w.Body.String())
			}
			var got view.SSH
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatalf("unmarshal get: %v", err)
			}
			if got.IsUsed != tt.want {
				t.Errorf("get isUsed = %v, want %v", got.IsUsed, tt.want)
			}
		})
	}
}

func TestGetSSHUsage(t *testing.T) {
	namespace := utils.GetNamespace(testProjectId)

	tests := []struct {
		name     string
		vms      []*unstructured.Unstructured
		key      string
		wantCode int
		wantBody string
	}{
		{name: "unused key", key: testKey, wantCode: http.StatusOK, wantBody: "[]"},
		{
			name:     "key in use",
			key:      testKey,
			vms:      []*unstructured.Unstructured{vmUsingKey(namespace)},
			wantCode: http.StatusOK,
			wantBody: `[{"eid":"vm-1","productName":""}]`,
		},
		{name: "unknown key", key: "unknown", wantCode: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setWatcher(t, informers.VirtualMachine, tt.vms...)
			setSecret(t, namespace)

			w := httptest.NewRecorder()
			getSSHUsage(w, sshRequest(http.MethodGet, map[string]string{"projectId": testProjectId, "effectiveId": tt.key}))

			if w.Code != tt.wantCode {
				t.Fatalf("code = %d, want %d (%s)", w.Code, tt.wantCode, w.Body.String())
			}
			if tt.wantBody != "" && w.Body.String() != tt.wantBody {
				t.Errorf("body = %s, want %s", w.Body.String(), tt.wantBody)
			}
		})
	}
}
