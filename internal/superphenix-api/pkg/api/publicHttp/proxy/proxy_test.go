package proxy

import (
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"reflect"
	"testing"
)

func TestReverseProxy(t *testing.T) {
	const rawUrlValid = "http://100.81.3.17"
	const rawUrlInvalid = "URLINVALID"
	const pattern = "/api/kubevirt"
	const urlScheme = "http"
	const urlHost = "100.81.3.17"

	emptyProxy := httputil.ReverseProxy{}

	type args struct {
		rawUrl       string
		pattern      string
		authSecret   string
		proxyRequest httputil.ProxyRequest
	}

	type want struct {
		url        string
		urlScheme  string
		urlHost    string
		path       string
		requestURI string
	}

	tests := []struct {
		name    string
		args    args
		want    want
		wantErr bool
	}{
		{
			name:    "Empty RawURL and Pattern",
			args:    args{rawUrl: "", pattern: ""},
			wantErr: true,
		},
		{
			name:    "Empty RawURL",
			args:    args{rawUrl: "", pattern: pattern},
			wantErr: true,
		},
		{
			name:    "Empty Pattern",
			args:    args{rawUrl: rawUrlValid, pattern: ""},
			wantErr: true,
		},
		{
			name:    "Invalid RawURL",
			args:    args{rawUrl: rawUrlInvalid, pattern: pattern},
			wantErr: true,
		},
		{
			name: "Valid RawURL and Pattern",
			args: args{
				rawUrl:  "http://100.81.3.17",
				pattern: "/api/kubevirt",
				proxyRequest: httputil.ProxyRequest{
					In:  httptest.NewRequest(http.MethodGet, "/api/kubevirt/vm", nil),
					Out: httptest.NewRequest(http.MethodGet, "/api/kubevirt/vm", nil),
				}},
			want: want{
				url:        "http://100.81.3.17/vm",
				urlScheme:  "http",
				urlHost:    "100.81.3.17",
				path:       "/vm",
				requestURI: "/vm",
			},
			wantErr: false,
		},
		{
			name: "Valid RawURL and Pattern with QueryParam",
			args: args{
				rawUrl:  "http://100.81.3.17",
				pattern: "/api/kubevirt",
				proxyRequest: httputil.ProxyRequest{
					In:  httptest.NewRequest(http.MethodGet, "/api/kubevirt/vm?bearer=token", nil),
					Out: httptest.NewRequest(http.MethodGet, "/api/kubevirt/vm?bearer=token", nil),
				}},
			want: want{
				url:        "http://100.81.3.17/vm?bearer=token",
				urlScheme:  "http",
				urlHost:    "100.81.3.17",
				path:       "/vm",
				requestURI: "/vm?bearer=token",
			},
			wantErr: false,
		},
		{
			name: "Encoded slashes stay encoded",
			args: args{
				rawUrl:  rawUrlValid,
				pattern: pattern,
				proxyRequest: httputil.ProxyRequest{
					In:  httptest.NewRequest(http.MethodGet, "/api/kubevirt/vm/..%2F..%2Fother%2Fvm", nil),
					Out: httptest.NewRequest(http.MethodGet, "/api/kubevirt/vm/..%2F..%2Fother%2Fvm", nil),
				}},
			want: want{
				url:        "http://100.81.3.17/vm/..%2F..%2Fother%2Fvm",
				urlScheme:  urlScheme,
				urlHost:    urlHost,
				path:       "/vm/../../other/vm",
				requestURI: "/vm/..%2F..%2Fother%2Fvm",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReverseProxy(tt.args.rawUrl, tt.args.pattern, tt.args.authSecret)

			// Check if we go expected err ouput
			if (err != nil) != tt.wantErr {
				t.Errorf("ReverseProxy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// If we expect error check if we have an Empty Proxy struct
			if tt.wantErr {
				if !reflect.DeepEqual(got, emptyProxy) {
					t.Errorf("ReverseProxy() got = %v, want %v", got, tt.want)
				}
				return
			}

			// If we didn't expect error check if the proxy is correctly setup
			got.Rewrite(&tt.args.proxyRequest)

			if tt.args.proxyRequest.Out.URL.String() != tt.want.url {
				t.Errorf("ReverseProxy() url got = %v, want %v", tt.args.proxyRequest.Out.URL.String(), tt.want.url)
			}

			if tt.args.proxyRequest.Out.URL.Scheme != tt.want.urlScheme {
				t.Errorf("ReverseProxy() urlScheme got = %v, want %v", tt.args.proxyRequest.Out.URL.Scheme, tt.want.urlScheme)
			}

			if tt.args.proxyRequest.Out.URL.Host != tt.want.urlHost {
				t.Errorf("ReverseProxy() urlHost got = %v, want %v", tt.args.proxyRequest.Out.URL.Host, tt.want.urlHost)
			}

			if tt.args.proxyRequest.Out.URL.Path != tt.want.path {
				t.Errorf("ReverseProxy() path got = %v, want %v", tt.args.proxyRequest.Out.URL.Path, tt.want.path)
			}

			if tt.args.proxyRequest.Out.RequestURI != tt.want.requestURI {
				t.Errorf("ReverseProxy() requestURI got = %v, want %v", tt.args.proxyRequest.Out.RequestURI, tt.want.requestURI)
			}

		})
	}
}

func TestRewriteRequest(t *testing.T) {
	const controllerUrl = "http://ctrl"
	const pattern = "/api/spx-ctrl/az1"

	tests := []struct {
		name     string
		rawUrl   string
		pattern  string
		target   string
		wantUrl  string
		wantPath string
		wantErr  bool
	}{
		{
			name:    "Empty pattern",
			rawUrl:  controllerUrl,
			target:  "/api/spx-ctrl/az1/proj/instance",
			wantErr: true,
		},
		{
			name:     "Plain path",
			rawUrl:   controllerUrl,
			pattern:  pattern,
			target:   "/orga/api/spx-ctrl/az1/proj/instance/eid?all=true",
			wantUrl:  "http://ctrl/orga/proj/instance/eid?all=true",
			wantPath: "/orga/proj/instance/eid",
		},
		{
			name:     "Encoded slashes stay encoded",
			rawUrl:   controllerUrl,
			pattern:  pattern,
			target:   "/orga/api/spx-ctrl/az1/proj/instance/..%2F..%2Fother%2Finstance%2Feid",
			wantUrl:  "http://ctrl/orga/proj/instance/..%2F..%2Fother%2Finstance%2Feid",
			wantPath: "/orga/proj/instance/../../other/instance/eid",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, tt.target, nil)
			err := RewriteRequest(r, tt.rawUrl, tt.pattern)
			if (err != nil) != tt.wantErr {
				t.Fatalf("RewriteRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if r.URL.String() != tt.wantUrl {
				t.Errorf("RewriteRequest() url got = %v, want %v", r.URL.String(), tt.wantUrl)
			}
			if r.URL.Path != tt.wantPath {
				t.Errorf("RewriteRequest() path got = %v, want %v", r.URL.Path, tt.wantPath)
			}
		})
	}
}
