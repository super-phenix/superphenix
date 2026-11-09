package kratos

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	kratos "github.com/ory/kratos-client-go"
)

const aal2Body = `{
	"error": {"id": "session_aal2_required", "code": 403, "status": "Forbidden", "message": "aal2 required"},
	"redirect_browser_to": "http://localhost:4433/self-service/login/browser?aal=aal2"
}`

func TestAuthenticatorKratosErrors(t *testing.T) {
	tests := []struct {
		name           string
		kratosStatus   int
		kratosBody     string
		query          string
		noCookie       bool
		expectStatus   int
		expectLocation string
	}{
		{
			name:           "second factor required keeps a valid return_to",
			kratosStatus:   http.StatusForbidden,
			kratosBody:     aal2Body,
			query:          "?return_to=http%3A%2F%2Flocalhost%3A4200%2Fdashboard",
			expectStatus:   http.StatusFound,
			expectLocation: "http://localhost:4433/self-service/login/browser?aal=aal2&return_to=http%3A%2F%2Flocalhost%3A4200%2Fdashboard",
		},
		{
			name:           "second factor required without return_to",
			kratosStatus:   http.StatusForbidden,
			kratosBody:     aal2Body,
			expectStatus:   http.StatusFound,
			expectLocation: "http://localhost:4433/self-service/login/browser?aal=aal2",
		},
		{
			name:           "literal null return_to from the console is dropped",
			kratosStatus:   http.StatusForbidden,
			kratosBody:     aal2Body,
			query:          "?return_to=null",
			expectStatus:   http.StatusFound,
			expectLocation: "http://localhost:4433/self-service/login/browser?aal=aal2",
		},
		{
			name:         "another 403 does not redirect",
			kratosStatus: http.StatusForbidden,
			kratosBody:   `{"error": {"id": "security_csrf_violation", "code": 403, "message": "csrf"}}`,
			expectStatus: http.StatusInternalServerError,
		},
		{
			name:         "aal2 error without a redirect target does not redirect",
			kratosStatus: http.StatusForbidden,
			kratosBody:   `{"error": {"id": "session_aal2_required", "code": 403, "message": "aal2 required"}}`,
			expectStatus: http.StatusInternalServerError,
		},
		{
			name:         "invalid session is unauthorized",
			kratosStatus: http.StatusUnauthorized,
			kratosBody:   `{"error": {"id": "session_inactive", "code": 401, "message": "no session"}}`,
			expectStatus: http.StatusUnauthorized,
		},
		{
			name:         "kratos failure stays a server error",
			kratosStatus: http.StatusInternalServerError,
			kratosBody:   `{"error": {"code": 500, "message": "boom"}}`,
			expectStatus: http.StatusInternalServerError,
		},
		{
			name:         "missing cookie is a bad request",
			noCookie:     true,
			expectStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.kratosStatus)
				_, _ = w.Write([]byte(tt.kratosBody))
			}))
			defer upstream.Close()

			previous := kratosApiClient
			defer func() { kratosApiClient = previous }()
			cfg := kratos.NewConfiguration()
			cfg.Servers = kratos.ServerConfigurations{{URL: upstream.URL}}
			kratosApiClient = kratos.NewAPIClient(cfg)

			request := httptest.NewRequest(http.MethodGet, "/v1/session/token"+tt.query, nil)
			if !tt.noCookie {
				request.AddCookie(&http.Cookie{Name: kratosCookie, Value: "session"})
			}
			recorder := httptest.NewRecorder()

			Authenticator(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Fatal("the next handler must not run")
			})).ServeHTTP(recorder, request)

			assert.Equal(t, tt.expectStatus, recorder.Code)
			assert.Equal(t, tt.expectLocation, recorder.Header().Get("Location"))
		})
	}
}

func TestSecondFactorRedirectIgnoresOtherErrors(t *testing.T) {
	assert.Empty(t, secondFactorRedirect(assert.AnError, "http://localhost:4200/"))
}
