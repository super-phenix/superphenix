package kubevirt

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	kovm "github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubevirt/vm"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	"github.com/go-chi/chi/v5"
	"k8s.io/client-go/kubernetes/fake"
	v1 "kubevirt.io/api/core/v1"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
)

// jsonReqWithParams builds a request with a JSON body and chi URL params.
func jsonReqWithParams(t *testing.T, target string, body any, params map[string]string) *http.Request {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	r := httptest.NewRequest(http.MethodPost, target, bytes.NewReader(raw))
	r.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestInstanceHandlers_ComputeSizeRejected(t *testing.T) {
	origPrefix := config.Global.SpxPrefix
	origK8s := config.K8sClient
	config.Global.SpxPrefix = "spx"
	t.Cleanup(func() {
		config.Global.SpxPrefix = origPrefix
		config.K8sClient = origK8s
	})

	const (
		orgId     = "0b9f6a52-3c1e-4d7a-9a51-2f4c8e6d1a01"
		projectId = "6c2d8e41-7a3b-4f90-b812-5e9d0c3a7f02"
		eid       = "vm-1"
	)
	params := map[string]string{"orgId": orgId, "projectId": projectId, "effectiveId": eid}

	tests := []struct {
		name   string
		update bool
		cpu    int
		memory string
		want   string
	}{
		{name: "create with cpu above maximum", cpu: 1000000, memory: "4Gi", want: "cpu value"},
		{name: "create with memory above maximum", cpu: 2, memory: "1000000Gi", want: "memory ("},
		{name: "update with cpu above maximum", update: true, cpu: 1000000, memory: "4Gi", want: "cpu value"},
		{name: "update with memory zero", update: true, cpu: 2, memory: "0Gi", want: "memory ("},
		{name: "create with unparseable memory", cpu: 2, memory: "abc", want: "memory ("},
		{name: "update with decimal unit memory", update: true, cpu: 2, memory: "2G", want: "memory ("},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.K8sClient = fake.NewSimpleClientset()
			// No expectations: invalid sizes are rejected before any KubeVirt call.
			installMockVirtClient(t)
			w := httptest.NewRecorder()

			if tt.update {
				var body kovm.UpdateVMInfo
				body.General.RunStrategy = string(v1.RunStrategyAlways)
				body.Compute.Cpu = tt.cpu
				body.Compute.Memory = tt.memory
				updateInstance(w, jsonReqWithParams(t, "/instance/"+eid, body, params))
			} else {
				body := kovm.CreateVMInfo{
					Metadata: spxId.Metadata{OrgId: orgId, ProjectId: projectId, ResourceLocalId: eid},
				}
				body.General.RunStrategy = string(v1.RunStrategyAlways)
				body.Compute.Cpu = tt.cpu
				body.Compute.Memory = tt.memory
				createInstance(w, jsonReqWithParams(t, "/instance", body, params))
			}

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d (%s)", w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tt.want) {
				t.Errorf("expected body containing %q, got %s", tt.want, w.Body.String())
			}
		})
	}
}
