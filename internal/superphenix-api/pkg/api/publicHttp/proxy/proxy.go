package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/consts"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	"github.com/go-chi/chi/v5/middleware"
)

func ApiProxy(httpProxy httputil.ReverseProxy) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		httpProxy.ServeHTTP(w, r)
	}
}

// AddUserIdToRequestHeader add the ID of the logged-in user in the header request
func AddUserIdToRequestHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := logger.GetLogger(r.Context())

		userId := r.Context().Value(consts.ContextUserId)
		if userId == nil {
			log.Error().Msg("No user id found in context")
			userId = ""
		}
		r.Header.Set(consts.HeaderUserId, userId.(string))

		next.ServeHTTP(w, r)
	})
}

// ReverseProxy Return a Reverse Proxy with a Rewrite function that change destination host and remove pattern from path
func ReverseProxy(rawUrl, pattern string, authSecret string) (httputil.ReverseProxy, error) {
	if rawUrl == "" || pattern == "" {
		return httputil.ReverseProxy{}, fmt.Errorf("parameters can't be empty: %s, %s", rawUrl, pattern)
	}

	endpoint, err := url.Parse(rawUrl)
	if err != nil {
		return httputil.ReverseProxy{}, fmt.Errorf("coudn't parse url: %s, %s", rawUrl, err)
	}

	if endpoint.Host == "" || endpoint.Scheme == "" {
		return httputil.ReverseProxy{}, fmt.Errorf("failed to get a valid endpoint with %s", rawUrl)
	}

	return httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			log := logger.GetLogger(r.In.Context())
			log.Info().Str("method", r.In.Method).Msgf("Proxy to %s on %s", rawUrl, pattern)
			r.SetURL(endpoint)
			if authSecret != "" {
				r.Out.Header.Set(consts.AuthorizationHeader, fmt.Sprintf("Bearer %s", authSecret)) // Override the AuthorizationHeader if needed
			}
			r.Out.RequestURI = strings.Replace(r.Out.RequestURI, pattern, "", 1) // Include Path and QueryParam
			r.Out.URL.Path = strings.Replace(r.Out.URL.Path, pattern, "", 1)     // Include only the path
		}}, nil
}

// SendBatchProxy proxies a request to multiple targets, removing the pattern from the path if given.
// targets: object AZ need at least code and url (others are optional)
func SendBatchProxy(r *http.Request, targets []config.AZConfig, pattern string) (map[string]*http.Response, error) {
	log := logger.GetLogger(r.Context())
	client := &http.Client{}
	responses := make(map[string]*http.Response)
	for _, target := range targets {
		r2, err := http.NewRequestWithContext(r.Context(), r.Method, r.URL.String(), r.Body)
		if err != nil {
			log.Error().Err(err).Msg("failed to create new request")
			return nil, err
		}
		// Re-add userId in header and Bearer
		r2.Header.Set(consts.HeaderUserId, r.Header.Get(consts.HeaderUserId))
		r2.Header.Set(middleware.RequestIDHeader, r.Header.Get(middleware.RequestIDHeader))
		r2.Header.Set(consts.AuthorizationHeader, fmt.Sprintf("Bearer %s", target.AuthSecret))

		if err := RewriteRequest(r2, target.ControllerUrl, pattern); err != nil {
			log.Error().Err(err).Msg("failed to proxy")
		}

		resp, err := client.Do(r2)
		if err != nil {
			log.Error().Err(err).Msg("Error doing request")
		}
		responses[target.Code] = resp
	}
	return responses, nil
}

// SendProxy Proxify a request to the target removing the pattern from the path and adding body if given
func SendProxy(r *http.Request, target config.AZConfig, pattern string, body io.Reader) (resp *http.Response, err error) {
	log := logger.GetLogger(r.Context())
	client := &http.Client{}
	r2, err := http.NewRequestWithContext(r.Context(), r.Method, r.URL.String(), body)
	if err != nil {
		log.Error().Err(err).Msg("failed to create new request")
		return nil, err
	}
	// Re-add userId in header and Bearer
	r2.Header.Set(consts.HeaderUserId, r.Header.Get(consts.HeaderUserId))
	r2.Header.Set(middleware.RequestIDHeader, r.Header.Get(middleware.RequestIDHeader))
	r2.Header.Set(consts.AuthorizationHeader, fmt.Sprintf("Bearer %s", target.AuthSecret))
	// Rewrite Request to redirect to the right controller
	if err := RewriteRequest(r2, target.ControllerUrl, pattern+"/"+target.Code); err != nil {
		log.Error().Err(err).Msg("failed to proxy")
		return nil, err
	}

	resp, err = client.Do(r2)
	if err != nil {
		log.Error().Err(err).Msg("Error doing request")
		return nil, err
	}

	return resp, nil
}

// SendRequest Send a request to the target removing the pattern from the path and adding body if given
func SendRequest(ctx context.Context, dest, method string, body io.Reader, bearer string) (resp *http.Response, err error) {
	log := logger.GetLogger(ctx)
	client := &http.Client{}
	r2, err := http.NewRequestWithContext(ctx, method, dest, body)
	if err != nil {
		log.Error().Err(err).Msg("failed to create new request")
		return nil, err
	}
	// Add userId (if present) in header and Bearer
	if userId, ok := ctx.Value(consts.ContextUserId).(string); ok {
		r2.Header.Set(consts.HeaderUserId, userId)
	}
	r2.Header.Set(middleware.RequestIDHeader, ctx.Value(middleware.RequestIDKey).(string))
	r2.Header.Set(consts.AuthorizationHeader, fmt.Sprintf("Bearer %s", bearer))
	resp, err = client.Do(r2)
	if err != nil {
		log.Error().Err(err).Msg("Error doing request")
		return nil, err
	}

	return resp, nil
}

// RewriteRequest change destination host and remove pattern from path in request
func RewriteRequest(r *http.Request, rawUrl, pattern string) error {
	if rawUrl == "" || pattern == "" {
		return fmt.Errorf("parameters can't be empty: %s, %s", rawUrl, pattern)
	}

	target, err := url.Parse(rawUrl)
	if err != nil {
		return fmt.Errorf("coudn't parse url: %s, %s", rawUrl, err)
	}

	if target.Host == "" || target.Scheme == "" {
		return fmt.Errorf("failed to get a valid target with %s", rawUrl)
	}

	rewriteRequestURL(r, target)
	r.RequestURI = ""                                        // http.Client.Do doesn't allow request URI to be filled, so we need to empty it
	r.URL.Path = strings.Replace(r.URL.Path, pattern, "", 1) // Include only the path
	r.Host = target.Host                                     // Change the initial host to the new one
	return nil
}

// rewriteRequestURL override request url with target
func rewriteRequestURL(req *http.Request, target *url.URL) {
	targetQuery := target.RawQuery
	req.URL.Scheme = target.Scheme
	req.URL.Host = target.Host
	req.URL.Path, req.URL.RawPath = joinURLPath(target, req.URL)
	if targetQuery == "" || req.URL.RawQuery == "" {
		req.URL.RawQuery = targetQuery + req.URL.RawQuery
	} else {
		req.URL.RawQuery = targetQuery + "&" + req.URL.RawQuery
	}
}

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}

func joinURLPath(a, b *url.URL) (path, rawpath string) {
	if a.RawPath == "" && b.RawPath == "" {
		return singleJoiningSlash(a.Path, b.Path), ""
	}
	// Same as singleJoiningSlash, but uses EscapedPath to determine
	// whether a slash should be added
	apath := a.EscapedPath()
	bpath := b.EscapedPath()

	aslash := strings.HasSuffix(apath, "/")
	bslash := strings.HasPrefix(bpath, "/")

	switch {
	case aslash && bslash:
		return a.Path + b.Path[1:], apath + bpath[1:]
	case !aslash && !bslash:
		return a.Path + "/" + b.Path, apath + "/" + bpath
	}
	return a.Path + b.Path, apath + bpath
}
