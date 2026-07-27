package authentication

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthenticate(t *testing.T) {
	// Dummy handler that just returns 200 OK
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	tests := []struct {
		name           string
		authTypes      []AuthType
		setupRequest   func() *http.Request
		expectedStatus int
	}{
		{
			name:      "No AuthTypes provided",
			authTypes: []AuthType{},
			setupRequest: func() *http.Request {
				return httptest.NewRequest("GET", "/", nil)
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "Single AuthType - Detection Fails",
			authTypes: []AuthType{
				{
					Name: "TestAuth",
					Detection: func(w http.ResponseWriter, r *http.Request) bool {
						return false
					},
					Validation: func(w http.ResponseWriter, r *http.Request) (*http.Request, error) {
						return r, nil
					},
				},
			},
			setupRequest: func() *http.Request {
				return httptest.NewRequest("GET", "/", nil)
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "Single AuthType - Detection Succeeds, Validation Succeeds",
			authTypes: []AuthType{
				{
					Name: "TestAuth",
					Detection: func(w http.ResponseWriter, r *http.Request) bool {
						return true
					},
					Validation: func(w http.ResponseWriter, r *http.Request) (*http.Request, error) {
						return r, nil
					},
				},
			},
			setupRequest: func() *http.Request {
				return httptest.NewRequest("GET", "/", nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "Single AuthType - Detection Succeeds, Validation Fails",
			authTypes: []AuthType{
				{
					Name: "TestAuth",
					Detection: func(w http.ResponseWriter, r *http.Request) bool {
						return true
					},
					Validation: func(w http.ResponseWriter, r *http.Request) (*http.Request, error) {
						return r, errors.New("validation failed")
					},
				},
			},
			setupRequest: func() *http.Request {
				return httptest.NewRequest("GET", "/", nil)
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "Multiple AuthTypes - Second Matches and Succeeds",
			authTypes: []AuthType{
				{
					Name: "Auth1",
					Detection: func(w http.ResponseWriter, r *http.Request) bool {
						return false
					},
					Validation: func(w http.ResponseWriter, r *http.Request) (*http.Request, error) {
						return r, nil
					},
				},
				{
					Name: "Auth2",
					Detection: func(w http.ResponseWriter, r *http.Request) bool {
						return true
					},
					Validation: func(w http.ResponseWriter, r *http.Request) (*http.Request, error) {
						return r, nil
					},
				},
			},
			setupRequest: func() *http.Request {
				return httptest.NewRequest("GET", "/", nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "Multiple AuthTypes - First Matches and Fails",
			authTypes: []AuthType{
				{
					Name: "Auth1",
					Detection: func(w http.ResponseWriter, r *http.Request) bool {
						return true
					},
					Validation: func(w http.ResponseWriter, r *http.Request) (*http.Request, error) {
						return r, errors.New("fail")
					},
				},
				{
					Name: "Auth2",
					Detection: func(w http.ResponseWriter, r *http.Request) bool {
						return true
					},
					Validation: func(w http.ResponseWriter, r *http.Request) (*http.Request, error) {
						return r, nil
					},
				},
			},
			setupRequest: func() *http.Request {
				return httptest.NewRequest("GET", "/", nil)
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "Multiple AuthTypes - None match",
			authTypes: []AuthType{
				{
					Name: "Auth1",
					Detection: func(w http.ResponseWriter, r *http.Request) bool {
						return false
					},
					Validation: func(w http.ResponseWriter, r *http.Request) (*http.Request, error) {
						return r, nil
					},
				},
				{
					Name: "Auth2",
					Detection: func(w http.ResponseWriter, r *http.Request) bool {
						return false
					},
					Validation: func(w http.ResponseWriter, r *http.Request) (*http.Request, error) {
						return r, nil
					},
				},
			},
			setupRequest: func() *http.Request {
				return httptest.NewRequest("GET", "/", nil)
			},
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := Authenticate(tt.authTypes...)
			handler := middleware(nextHandler)

			req := tt.setupRequest()
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}
		})
	}
}
