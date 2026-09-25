package audit

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClientIP(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		remoteAddr string
		want       string
	}{
		{
			name:       "cloudflare header wins",
			headers:    map[string]string{"CF-Connecting-IP": "1.1.1.1", "X-Forwarded-For": "2.2.2.2", "X-Real-IP": "3.3.3.3"},
			remoteAddr: "10.0.0.1:1234",
			want:       "1.1.1.1",
		},
		{
			name:       "forwarded for keeps the first hop",
			headers:    map[string]string{"X-Forwarded-For": " 2.2.2.2 , 10.0.0.9", "X-Real-IP": "3.3.3.3"},
			remoteAddr: "10.0.0.1:1234",
			want:       "2.2.2.2",
		},
		{
			name:       "real ip comes last",
			headers:    map[string]string{"X-Real-IP": "3.3.3.3"},
			remoteAddr: "10.0.0.1:1234",
			want:       "3.3.3.3",
		},
		{
			name:       "no header falls back to the peer",
			remoteAddr: "10.0.0.1:1234",
			want:       "10.0.0.1",
		},
		{
			name:       "ipv6 peer",
			remoteAddr: "[2001:db8::1]:1234",
			want:       "2001:db8::1",
		},
		{
			name:       "peer without port",
			remoteAddr: "10.0.0.1",
			want:       "10.0.0.1",
		},
		{
			name:       "oversized header is truncated",
			headers:    map[string]string{"X-Real-IP": strings.Repeat("a", 200)},
			remoteAddr: "10.0.0.1:1234",
			want:       strings.Repeat("a", maxAddrLength),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/", nil)
			r.RemoteAddr = tt.remoteAddr
			for key, value := range tt.headers {
				r.Header.Set(key, value)
			}

			assert.Equal(t, tt.want, ClientIP(r))
		})
	}
}

func TestCapturePeerAddrSurvivesRemoteAddrRewrite(t *testing.T) {
	tests := []struct {
		name       string
		capture    bool
		remoteAddr string
		rewritten  string
		want       string
	}{
		{name: "captured peer is kept", capture: true, remoteAddr: "10.0.0.1:1234", rewritten: "9.9.9.9", want: "10.0.0.1"},
		{name: "without capture the request address is used", remoteAddr: "10.0.0.1:1234", rewritten: "9.9.9.9", want: "9.9.9.9"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			var handler http.Handler = http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				// What chi's RealIP does.
				r.RemoteAddr = tt.rewritten
				got = PeerAddr(r)
			})
			if tt.capture {
				handler = CapturePeerAddr(handler)
			}

			r := httptest.NewRequest(http.MethodPost, "/", nil)
			r.RemoteAddr = tt.remoteAddr
			handler.ServeHTTP(httptest.NewRecorder(), r)

			assert.Equal(t, tt.want, got)
		})
	}
}
