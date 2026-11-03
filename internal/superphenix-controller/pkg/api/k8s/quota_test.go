package k8s

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
)

func TestSetQuotaRevision(t *testing.T) {
	const (
		orgId     = "11111111-1111-1111-1111-111111111111"
		projectId = "22222222-2222-2222-2222-222222222222"
	)
	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantBody   string
	}{
		{"older push: 412 naming both revisions", `{"hard":{"requests.cpu":"8"},"revision":5}`, http.StatusPreconditionFailed, "10, pushed 5"},
		{"newer push: applied", `{"hard":{"requests.cpu":"8"},"revision":11}`, http.StatusNoContent, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.K8sClient = fake.NewClientset(
				&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "spx-" + projectId}},
				&v1.ResourceQuota{
					ObjectMeta: metav1.ObjectMeta{
						Name: "quota-" + projectId, Namespace: "spx-" + projectId,
						Annotations: map[string]string{spxId.SpxAnnotationQuotaRevision: "10"},
					},
					Spec: v1.ResourceQuotaSpec{Hard: v1.ResourceList{"requests.cpu": resource.MustParse("4")}},
				},
			)
			config.Global.SpxPrefix = "spx"
			router := chi.NewRouter()
			router.Route("/{orgId}/{projectId}", QuotaEndpoint)

			req := httptest.NewRequest(http.MethodPut, "/"+orgId+"/"+projectId+"/quota", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("expected a body containing %q, got %q", tt.wantBody, rec.Body.String())
			}
		})
	}
}
