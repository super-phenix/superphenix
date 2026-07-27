// Package router holds HTTP routes and middlewares as data that can be added,
// overridden or removed by key, then materialised onto a chi router at Build.
package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// Middleware is the chi/std middleware shape used across the API.
type Middleware = func(http.Handler) http.Handler

// Route is one endpoint, identified for override/remove by Method + full mounted
// path (module Mount + Pattern).
type Route struct {
	Method      string
	Pattern     string // relative to the owning module's Mount
	Handler     http.HandlerFunc
	Middlewares []Middleware // route-specific chain, applied in order
}

// methodAny is the sentinel Method for a Route that handles every HTTP verb. It
// is materialised with chi's Handle (all-method) rather than Method.
const methodAny = "ANY"

// Get, Post, Put, Patch, Delete, Head and Options build a Route for the matching
// method. The variadic middlewares form the route-specific chain, applied in order
// after the module's (and any group's) shared middlewares.
func Get(pattern string, h http.HandlerFunc, mw ...Middleware) Route {
	return Route{Method: http.MethodGet, Pattern: pattern, Handler: h, Middlewares: mw}
}
func Post(pattern string, h http.HandlerFunc, mw ...Middleware) Route {
	return Route{Method: http.MethodPost, Pattern: pattern, Handler: h, Middlewares: mw}
}
func Put(pattern string, h http.HandlerFunc, mw ...Middleware) Route {
	return Route{Method: http.MethodPut, Pattern: pattern, Handler: h, Middlewares: mw}
}
func Patch(pattern string, h http.HandlerFunc, mw ...Middleware) Route {
	return Route{Method: http.MethodPatch, Pattern: pattern, Handler: h, Middlewares: mw}
}
func Delete(pattern string, h http.HandlerFunc, mw ...Middleware) Route {
	return Route{Method: http.MethodDelete, Pattern: pattern, Handler: h, Middlewares: mw}
}
func Head(pattern string, h http.HandlerFunc, mw ...Middleware) Route {
	return Route{Method: http.MethodHead, Pattern: pattern, Handler: h, Middlewares: mw}
}
func Options(pattern string, h http.HandlerFunc, mw ...Middleware) Route {
	return Route{Method: http.MethodOptions, Pattern: pattern, Handler: h, Middlewares: mw}
}

// Handle builds a Route for an arbitrary HTTP method recognised by chi.
func Handle(method, pattern string, h http.HandlerFunc, mw ...Middleware) Route {
	return Route{Method: method, Pattern: pattern, Handler: h, Middlewares: mw}
}

// Any builds a Route that matches every HTTP method at the pattern.
func Any(pattern string, h http.HandlerFunc, mw ...Middleware) Route {
	return Route{Method: methodAny, Pattern: pattern, Handler: h, Middlewares: mw}
}

// Group is a nested set of routes sharing an extra path prefix and shared
// middlewares, relative to the owning module or parent group. Groups nest.
type Group struct {
	Prefix      string       // prepended to each contained Pattern, after the parent's prefix
	Middlewares []Middleware // shared, appended after the parent's shared middlewares
	Routes      []Route
	Groups      []Group // nested sub-groups
}

// Module is a named group of routes sharing a mount prefix and shared
// middlewares, optionally gated at Build. Routes may be declared directly or
// nested under Groups.
type Module struct {
	Name        string
	Mount       string       // prefix prepended to each route's Pattern
	Middlewares []Middleware // shared, prepended to every route's chain
	Routes      []Route
	Groups      []Group     // nested route groups, relative to Mount
	Enabled     func() bool // nil = always on; evaluated at Build
}

// extensionsModule collects routes added via AddRoute/OverrideRoute that don't
// belong to a registered module.
const extensionsModule = "extensions"

type namedMW struct {
	name string
	fn   Middleware
}

// Registry holds the global middleware chain, the modules and pending
// route-level edits, which Build wires onto a chi router. Not safe for
// concurrent use; configure from one goroutine before Build.
type Registry struct {
	globals []namedMW
	modules []*Module

	routeOverrides map[string]Route // key: routeKey(method, fullPattern)
	routeRemovals  map[string]bool  // key: routeKey(method, fullPattern)
}

// New returns an empty Registry.
func New() *Registry {
	return &Registry{
		routeOverrides: map[string]Route{},
		routeRemovals:  map[string]bool{},
	}
}

// Reset clears all globals, modules and route edits, returning the Registry to empty.
func (r *Registry) Reset() *Registry {
	r.globals = nil
	r.modules = nil
	r.routeOverrides = map[string]Route{}
	r.routeRemovals = map[string]bool{}
	return r
}

func routeKey(method, fullPattern string) string { return method + " " + fullPattern }

// --- Global middleware chain ---

// Use appends a global middleware, keyed by name, to the router-level chain.
func (r *Registry) Use(name string, mw Middleware) *Registry {
	r.globals = append(r.globals, namedMW{name: name, fn: mw})
	return r
}

// ReplaceGlobalMW swaps the global middleware registered under name, keeping its
// position. If no global middleware has that name it is appended.
func (r *Registry) ReplaceGlobalMW(name string, mw Middleware) *Registry {
	for i := range r.globals {
		if r.globals[i].name == name {
			r.globals[i].fn = mw
			return r
		}
	}
	return r.Use(name, mw)
}

// RemoveGlobalMW drops the global middleware registered under name (no-op if
// absent).
func (r *Registry) RemoveGlobalMW(name string) *Registry {
	out := r.globals[:0]
	for _, g := range r.globals {
		if g.name != name {
			out = append(out, g)
		}
	}
	r.globals = out
	return r
}

// --- Modules ---

// Register appends a module. Registration order is the mount order at Build.
func (r *Registry) Register(m Module) *Registry {
	mod := m
	r.modules = append(r.modules, &mod)
	return r
}

// Override replaces the module registered under name, keeping its position. If
// no module has that name the replacement is appended.
func (r *Registry) Override(name string, m Module) *Registry {
	mod := m
	for i := range r.modules {
		if r.modules[i].Name == name {
			r.modules[i] = &mod
			return r
		}
	}
	return r.Register(m)
}

// RemoveModule drops the module registered under name (no-op if absent).
func (r *Registry) RemoveModule(name string) *Registry {
	out := r.modules[:0]
	for _, m := range r.modules {
		if m.Name != name {
			out = append(out, m)
		}
	}
	r.modules = out
	return r
}

// --- Route-level edits (resolved against full Method+Pattern keys at Build) ---

// AddRoute registers a standalone route under the synthetic "extensions"
// module. Its Pattern is treated as the full path (no module mount applied).
func (r *Registry) AddRoute(rt Route) *Registry {
	for i := range r.modules {
		if r.modules[i].Name == extensionsModule {
			r.modules[i].Routes = append(r.modules[i].Routes, rt)
			return r
		}
	}
	return r.Register(Module{Name: extensionsModule, Routes: []Route{rt}})
}

// OverrideRoute replaces the route at method+pattern (the full mounted path).
// The replacement's Middlewares are the complete post-global chain: the module's
// shared middlewares are not re-applied, so the caller states exactly what wraps
// the route. An override that matches no live route at Build is skipped (logged),
// not materialised — this includes routes whose module is disabled or removed, so
// an override can never resurrect a gated-off route as a standalone, unguarded
// one. Use AddRoute to add a genuinely new route. The replacement's Method and
// Pattern are normalised to the arguments, so they always match the key.
func (r *Registry) OverrideRoute(method, pattern string, rt Route) *Registry {
	rt.Method = method
	rt.Pattern = pattern
	r.routeOverrides[routeKey(method, pattern)] = rt
	delete(r.routeRemovals, routeKey(method, pattern))
	return r
}

// RemoveRoute drops the route identified by method+pattern (the full mounted
// path), whether it comes from a module or a prior AddRoute/OverrideRoute.
func (r *Registry) RemoveRoute(method, pattern string) *Registry {
	key := routeKey(method, pattern)
	r.routeRemovals[key] = true
	delete(r.routeOverrides, key)
	return r
}

// resolvedRoute is a route after mount/override/middleware resolution.
type resolvedRoute struct {
	method  string
	pattern string
	handler http.HandlerFunc
	chain   []Middleware
}

// Build resolves gating, overrides and removals, then wires everything onto root.
// Middleware order (globals, module-shared, group-shared, route-specific) is
// preserved because authentication must run before authorization. Routes are
// materialised flat at their full path rather than through per-module sub-routers,
// since module mounts overlap (e.g. "/v1" and "/v1/project-manager") and chi panics
// on overlapping sub-router mounts.
func (r *Registry) Build(root chi.Router) {
	// Global middlewares must precede any route registration.
	for _, g := range r.globals {
		root.Use(g.fn)
	}

	// Record every declared route key, including those in disabled or gated-off
	// modules, so an unmatched override can tell "targets a route that was gated
	// off" (drop it — the route stays off) from "targets nothing" (a mistake).
	declared := map[string]bool{}
	for _, m := range r.modules {
		r.collectDeclaredKeys(m.Mount, m.Routes, m.Groups, declared)
	}

	// Flatten modules (and their nested groups) into resolved routes, honouring
	// gating, overrides and removals. Track consumed override keys so we can tell
	// which overrides matched a live route.
	consumed := map[string]bool{}
	var resolved []resolvedRoute

	for _, m := range r.modules {
		if m.Enabled != nil && !m.Enabled() {
			log.Info().Str("module", m.Name).Msg("router: module disabled, skipping")
			continue
		}
		r.flatten(m.Mount, m.Middlewares, m.Routes, m.Groups, consumed, &resolved)
	}

	// An override that matched no live route must not become a standalone route:
	// materialising it would carry only the override's own chain, bypassing the
	// owning module's enable gate and shared (auth) middlewares.
	for key := range r.routeOverrides {
		if consumed[key] || r.routeRemovals[key] {
			continue
		}
		if declared[key] {
			// Targets a real route whose module is disabled/removed: keep it off.
			log.Info().Str("route", key).Msg("router: override targets a disabled or removed route, skipping")
			continue
		}
		// Targets nothing declared — almost certainly a typo. Additions must go
		// through AddRoute, which states the route's full chain deliberately.
		log.Warn().Str("route", key).Msg("router: override matches no route, skipping (use AddRoute to add a new route)")
	}

	// Materialise onto chi.
	for _, rt := range resolved {
		if rt.method == methodAny {
			root.With(rt.chain...).Handle(rt.pattern, rt.handler)
			continue
		}
		root.With(rt.chain...).Method(rt.method, rt.pattern, rt.handler)
	}
}

// collectDeclaredKeys records the full Method+path key of every route in a module
// level (and its nested groups), mirroring flatten's prefixing. Unlike flatten it
// ignores gating, so keys from disabled modules are recorded too.
func (r *Registry) collectDeclaredKeys(prefix string, routes []Route, groups []Group, declared map[string]bool) {
	for _, rt := range routes {
		declared[routeKey(rt.Method, prefix+rt.Pattern)] = true
	}
	for _, g := range groups {
		r.collectDeclaredKeys(prefix+g.Prefix, g.Routes, g.Groups, declared)
	}
}

// flatten resolves the routes of one module level (and recurses into nested
// groups) into resolvedRoute entries. prefix is the accumulated mount/group path
// and shared is the accumulated shared-middleware chain; both grow as groups nest.
func (r *Registry) flatten(prefix string, shared []Middleware, routes []Route, groups []Group, consumed map[string]bool, out *[]resolvedRoute) {
	for _, rt := range routes {
		full := prefix + rt.Pattern
		key := routeKey(rt.Method, full)

		if r.routeRemovals[key] {
			log.Info().Str("route", key).Msg("router: route removed")
			continue
		}
		if ov, ok := r.routeOverrides[key]; ok {
			// An override replaces the whole post-global chain: shared middlewares
			// are intentionally not re-applied.
			consumed[key] = true
			*out = append(*out, resolvedRoute{
				method:  rt.Method,
				pattern: full,
				handler: ov.Handler,
				chain:   ov.Middlewares,
			})
			continue
		}

		chain := make([]Middleware, 0, len(shared)+len(rt.Middlewares))
		chain = append(chain, shared...)
		chain = append(chain, rt.Middlewares...)
		*out = append(*out, resolvedRoute{
			method:  rt.Method,
			pattern: full,
			handler: rt.Handler,
			chain:   chain,
		})
	}

	for _, g := range groups {
		childShared := make([]Middleware, 0, len(shared)+len(g.Middlewares))
		childShared = append(childShared, shared...)
		childShared = append(childShared, g.Middlewares...)
		r.flatten(prefix+g.Prefix, childShared, g.Routes, g.Groups, consumed, out)
	}
}
