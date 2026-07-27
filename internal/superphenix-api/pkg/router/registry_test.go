package router_test

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"

	"github.com/go-chi/chi/v5"
)

// tag returns a middleware that records its name, so tests can assert chain order.
func tag(rec *[]string, name string) router.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			*rec = append(*rec, name)
			next.ServeHTTP(w, r)
		})
	}
}

// handler records its name and returns 200.
func handler(rec *[]string, name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		*rec = append(*rec, name)
		w.WriteHeader(http.StatusOK)
	}
}

func do(root chi.Router, method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	rr := httptest.NewRecorder()
	root.ServeHTTP(rr, req)
	return rr
}

func build(reg *router.Registry) chi.Router {
	root := chi.NewRouter()
	reg.Build(root)
	return root
}

func TestBuildAppliesChainInOrder(t *testing.T) {
	var rec []string
	reg := router.New().
		Use("g1", tag(&rec, "g1")).
		Use("g2", tag(&rec, "g2")).
		Register(router.Module{
			Name:        "m",
			Mount:       "/v1",
			Middlewares: []router.Middleware{tag(&rec, "mod")},
			Routes: []router.Route{{
				Method:      http.MethodGet,
				Pattern:     "/x",
				Handler:     handler(&rec, "h"),
				Middlewares: []router.Middleware{tag(&rec, "route")},
			}},
		})

	rr := do(build(reg), http.MethodGet, "/v1/x")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	want := []string{"g1", "g2", "mod", "route", "h"}
	if !reflect.DeepEqual(rec, want) {
		t.Errorf("chain order = %v, want %v", rec, want)
	}
}

func TestReplaceAndRemoveGlobalMW(t *testing.T) {
	var rec []string
	reg := router.New().
		Use("a", tag(&rec, "a")).
		Use("b", tag(&rec, "b")).
		Register(router.Module{Name: "m", Routes: []router.Route{{
			Method: http.MethodGet, Pattern: "/x", Handler: handler(&rec, "h"),
		}}})

	reg.ReplaceGlobalMW("a", tag(&rec, "a2")) // keeps position
	reg.RemoveGlobalMW("b")

	do(build(reg), http.MethodGet, "/x")
	want := []string{"a2", "h"}
	if !reflect.DeepEqual(rec, want) {
		t.Errorf("chain = %v, want %v", rec, want)
	}
}

func TestOverrideRouteReplacesByKeyWithoutPanic(t *testing.T) {
	var rec []string
	reg := router.New().
		Register(router.Module{
			Name:        "m",
			Mount:       "/v1",
			Middlewares: []router.Middleware{tag(&rec, "mod")},
			Routes: []router.Route{{
				Method: http.MethodGet, Pattern: "/x", Handler: handler(&rec, "old"),
			}},
		})

	// Override replaces the whole route; the module's shared middleware is not re-applied.
	reg.OverrideRoute(http.MethodGet, "/v1/x", router.Route{
		Handler:     handler(&rec, "new"),
		Middlewares: []router.Middleware{tag(&rec, "newmw")},
	})

	rr := do(build(reg), http.MethodGet, "/v1/x")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	want := []string{"newmw", "new"}
	if !reflect.DeepEqual(rec, want) {
		t.Errorf("chain = %v, want %v (module mw must not re-apply, old handler gone)", rec, want)
	}
}

func TestRemoveRoute(t *testing.T) {
	reg := router.New().Register(router.Module{
		Name: "m", Routes: []router.Route{
			{Method: http.MethodGet, Pattern: "/x", Handler: func(http.ResponseWriter, *http.Request) {}},
			{Method: http.MethodGet, Pattern: "/y", Handler: func(http.ResponseWriter, *http.Request) {}},
		},
	})
	reg.RemoveRoute(http.MethodGet, "/x")

	root := build(reg)
	if got := do(root, http.MethodGet, "/x").Code; got != http.StatusNotFound {
		t.Errorf("/x status = %d, want 404 (removed)", got)
	}
	if got := do(root, http.MethodGet, "/y").Code; got != http.StatusOK {
		t.Errorf("/y status = %d, want 200 (kept)", got)
	}
}

func TestEnabledFalseSkipsModule(t *testing.T) {
	reg := router.New().Register(router.Module{
		Name:    "gated",
		Enabled: func() bool { return false },
		Routes:  []router.Route{{Method: http.MethodGet, Pattern: "/x", Handler: func(http.ResponseWriter, *http.Request) {}}},
	})
	if got := do(build(reg), http.MethodGet, "/x").Code; got != http.StatusNotFound {
		t.Errorf("gated route status = %d, want 404", got)
	}
}

func TestOverrideModuleReplacesWhole(t *testing.T) {
	reg := router.New().Register(router.Module{
		Name:   "m",
		Routes: []router.Route{{Method: http.MethodGet, Pattern: "/old", Handler: func(http.ResponseWriter, *http.Request) {}}},
	})
	reg.Override("m", router.Module{
		Name:   "m",
		Routes: []router.Route{{Method: http.MethodGet, Pattern: "/new", Handler: func(http.ResponseWriter, *http.Request) {}}},
	})

	root := build(reg)
	if got := do(root, http.MethodGet, "/old").Code; got != http.StatusNotFound {
		t.Errorf("/old status = %d, want 404 (module replaced)", got)
	}
	if got := do(root, http.MethodGet, "/new").Code; got != http.StatusOK {
		t.Errorf("/new status = %d, want 200", got)
	}
}

func TestVerbHelpers(t *testing.T) {
	reg := router.New().Register(router.Module{
		Name:  "m",
		Mount: "/v1",
		Routes: []router.Route{
			router.Get("/g", func(http.ResponseWriter, *http.Request) {}),
			router.Post("/p", func(http.ResponseWriter, *http.Request) {}),
			router.Put("/u", func(http.ResponseWriter, *http.Request) {}),
			router.Patch("/pa", func(http.ResponseWriter, *http.Request) {}),
			router.Delete("/d", func(http.ResponseWriter, *http.Request) {}),
			router.Head("/h", func(http.ResponseWriter, *http.Request) {}),
			router.Options("/o", func(http.ResponseWriter, *http.Request) {}),
			router.Handle(http.MethodConnect, "/c", func(http.ResponseWriter, *http.Request) {}),
		},
	})

	root := build(reg)
	cases := []struct {
		method, path string
	}{
		{http.MethodGet, "/v1/g"}, {http.MethodPost, "/v1/p"}, {http.MethodPut, "/v1/u"},
		{http.MethodPatch, "/v1/pa"}, {http.MethodDelete, "/v1/d"}, {http.MethodHead, "/v1/h"},
		{http.MethodOptions, "/v1/o"}, {http.MethodConnect, "/v1/c"},
	}
	for _, c := range cases {
		if got := do(root, c.method, c.path).Code; got != http.StatusOK {
			t.Errorf("%s %s status = %d, want 200", c.method, c.path, got)
		}
	}
}

func TestAnyMatchesEveryMethod(t *testing.T) {
	reg := router.New().Register(router.Module{
		Name:   "m",
		Routes: []router.Route{router.Any("/x", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })},
	})
	root := build(reg)
	for _, m := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		if got := do(root, m, "/x").Code; got != http.StatusOK {
			t.Errorf("Any %s status = %d, want 200", m, got)
		}
	}
}

func TestNestedGroupsAccumulatePrefixAndMiddleware(t *testing.T) {
	var rec []string
	reg := router.New().
		Use("g", tag(&rec, "global")).
		Register(router.Module{
			Name:        "m",
			Mount:       "/v1",
			Middlewares: []router.Middleware{tag(&rec, "mod")},
			Groups: []router.Group{{
				Prefix:      "/orga",
				Middlewares: []router.Middleware{tag(&rec, "grp1")},
				Groups: []router.Group{{
					Prefix:      "/iam",
					Middlewares: []router.Middleware{tag(&rec, "grp2")},
					Routes: []router.Route{
						router.Get("/users", handler(&rec, "h"), tag(&rec, "route")),
					},
				}},
			}},
		})

	rr := do(build(reg), http.MethodGet, "/v1/orga/iam/users")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (nested group path)", rr.Code)
	}
	want := []string{"global", "mod", "grp1", "grp2", "route", "h"}
	if !reflect.DeepEqual(rec, want) {
		t.Errorf("chain order = %v, want %v", rec, want)
	}
}

func TestGroupRouteIsOverridableAndRemovable(t *testing.T) {
	mk := func() *router.Registry {
		return router.New().Register(router.Module{
			Name:        "m",
			Mount:       "/v1",
			Middlewares: []router.Middleware{func(n http.Handler) http.Handler { return n }},
			Groups: []router.Group{{
				Prefix: "/orga",
				Routes: []router.Route{router.Get("/x", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })},
			}},
		})
	}

	// Override a nested-group route by its full path.
	reg := mk()
	reg.OverrideRoute(http.MethodGet, "/v1/orga/x", router.Route{
		Handler: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) },
	})
	if got := do(build(reg), http.MethodGet, "/v1/orga/x").Code; got != http.StatusOK {
		t.Errorf("overridden group route status = %d, want 200", got)
	}

	// Remove a nested-group route by its full path.
	reg2 := mk()
	reg2.RemoveRoute(http.MethodGet, "/v1/orga/x")
	if got := do(build(reg2), http.MethodGet, "/v1/orga/x").Code; got != http.StatusNotFound {
		t.Errorf("removed group route status = %d, want 404", got)
	}
}

func TestResetClearsRegistry(t *testing.T) {
	reg := router.New().
		Use("g", func(n http.Handler) http.Handler { return n }).
		Register(router.Module{Name: "m", Routes: []router.Route{
			{Method: http.MethodGet, Pattern: "/x", Handler: func(http.ResponseWriter, *http.Request) {}},
		}})
	reg.Reset()

	if got := do(build(reg), http.MethodGet, "/x").Code; got != http.StatusNotFound {
		t.Errorf("after Reset status = %d, want 404 (registry emptied)", got)
	}
}

func TestAddRouteRegistersRouteButUnmatchedOverrideDoesNot(t *testing.T) {
	reg := router.New()
	reg.AddRoute(router.Route{Method: http.MethodGet, Pattern: "/added", Handler: func(http.ResponseWriter, *http.Request) {}})
	// An override that matches no declared route is a mistake, not an addition:
	// it must be skipped, not materialised as a new (unguarded) route.
	reg.OverrideRoute(http.MethodGet, "/created", router.Route{
		Method: http.MethodGet, Pattern: "/created", Handler: func(http.ResponseWriter, *http.Request) {},
	})

	root := build(reg)
	if got := do(root, http.MethodGet, "/added").Code; got != http.StatusOK {
		t.Errorf("/added status = %d, want 200 (AddRoute)", got)
	}
	if got := do(root, http.MethodGet, "/created").Code; got != http.StatusNotFound {
		t.Errorf("/created status = %d, want 404 (unmatched override must not create a route)", got)
	}
}

func TestOverrideOfDisabledModuleRouteStaysOff(t *testing.T) {
	rec := false
	reg := router.New().Register(router.Module{
		Name:    "gated",
		Mount:   "/v1",
		Enabled: func() bool { return false }, // module disabled
		Routes: []router.Route{{
			Method: http.MethodGet, Pattern: "/x", Handler: func(http.ResponseWriter, *http.Request) {},
		}},
	})
	// Overriding a route inside the disabled module must not resurrect it as a
	// standalone route (which would bypass the module's gate and shared chain).
	reg.OverrideRoute(http.MethodGet, "/v1/x", router.Route{
		Handler: func(http.ResponseWriter, *http.Request) { rec = true },
	})

	if got := do(build(reg), http.MethodGet, "/v1/x").Code; got != http.StatusNotFound {
		t.Errorf("/v1/x status = %d, want 404 (disabled module route must stay off)", got)
	}
	if rec {
		t.Error("override handler ran; a disabled module's route must not be resurrected by an override")
	}
}
