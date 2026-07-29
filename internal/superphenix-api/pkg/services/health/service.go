package health

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"
)

const ModuleName = "health"

// API is the overridable seam for the health endpoints.
type API interface {
	Readyz(http.ResponseWriter, *http.Request)
}

// Service is the default implementation of API.
type Service struct {
	cfg *config.Config
}

var _ API = (*Service)(nil)

// New constructs the default service.
func New(cfg *config.Config) *Service { return &Service{cfg: cfg} }

// Readyz checks that both the public and admin HTTP servers are
// reachable by opening a TCP connection to each configured address.
//
//	@Summary		Kubernetes readiness probe
//	@Description	Check that both public and admin HTTP servers are reachable
//	@Tags			Health
//	@Produce		json
//	@Success		200	{object}	map[string]string	"All servers are up"
//	@Failure		503	{object}	map[string]string	"One or more servers are down"
//	@Router			/readyz [get]
func (h *Service) Readyz(w http.ResponseWriter, _ *http.Request) {
	type target struct {
		name    string
		address string
	}

	targets := []target{
		{name: "public", address: h.cfg.PublicHTTP.Address},
	}
	if h.cfg.AdminHTTP.Enabled {
		targets = append(targets, target{name: "admin", address: h.cfg.AdminHTTP.Address})
	}

	type result struct {
		name string
		ok   bool
		err  error
	}

	var wg sync.WaitGroup
	results := make([]result, len(targets))

	for i, t := range targets {
		wg.Add(1)
		go func(idx int, tgt target) {
			defer wg.Done()
			conn, err := net.DialTimeout("tcp", tgt.address, 2*time.Second)
			if err != nil {
				results[idx] = result{name: tgt.name, ok: false, err: err}
				return
			}
			conn.Close()
			results[idx] = result{name: tgt.name, ok: true}
		}(i, t)
	}
	wg.Wait()

	allReady := true
	status := make(map[string]string, len(results))
	for _, r := range results {
		if r.ok {
			status[r.name] = "up"
		} else {
			allReady = false
			status[r.name] = fmt.Sprintf("down: %v", r.err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if allReady {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}
	marshal, err := json.Marshal(status)
	if err != nil {
		log.Error().Err(err).Msg("Error marshalling response")
	}
	_, _ = w.Write(marshal)
}

// Module builds the route module for any API implementation.
func Module(_ *config.Config, s API) router.Module {
	return router.Module{
		Name: ModuleName,
		Routes: []router.Route{
			router.Get("/readyz", s.Readyz),
		},
	}
}

// ProvideService constructs the default service and registers its routes on reg.
func ProvideService(cfg *config.Config, reg *router.Registry) {
	h := New(cfg)
	reg.Register(Module(cfg, h))
}
