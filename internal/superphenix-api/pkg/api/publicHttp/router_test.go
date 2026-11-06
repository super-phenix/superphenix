package publicHttp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRejectPathTraversal(t *testing.T) {
	const instancePath = "/orga/api/spx-ctrl/az1/proj/instance/"

	tests := []struct {
		name       string
		target     string
		wantStatus int
	}{
		{name: "Plain path", target: instancePath + "spx-16c2e1e7", wantStatus: http.StatusOK},
		{name: "Encoded slash in query", target: instancePath + "spx-16c2e1e7?next=a%2Fb", wantStatus: http.StatusOK},
		{name: "Encoded slashes", target: instancePath + "..%2F..%2Fother%2Finstance%2Feid", wantStatus: http.StatusBadRequest},
		{name: "Lowercase encoded slash", target: instancePath + "x%2fy", wantStatus: http.StatusBadRequest},
		{name: "Encoded backslash", target: instancePath + "..%5Cother", wantStatus: http.StatusBadRequest},
		{name: "Double encoded slash", target: instancePath + "..%252Fother", wantStatus: http.StatusBadRequest},
		{name: "Encoded dot segment", target: instancePath + "%2E%2E/other", wantStatus: http.StatusBadRequest},
		{name: "Literal dot segment", target: instancePath + "../other", wantStatus: http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

			rejectPathTraversal(next).ServeHTTP(w, httptest.NewRequest(http.MethodGet, tt.target, nil))

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}
