// Package app holds the infrastructure bootstrap (database, Permify) shared by the API.
package app

import (
	"context"
	"sync"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/argo"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/argo/gc"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"

	pwClient "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/client"

	"github.com/rs/zerolog/log"
)

var (
	infraOnce sync.Once
	infraErr  error

	argoOnce   sync.Once
	argoClient *argo.Client
)

// ProvideInfra connects the Permify and database clients. It is idempotent: the
// connection and migration run once regardless of how many composition roots
// (public, admin) call it, and every caller sees the same result. These
// initialisers set the package globals the current CRUD/permify helpers use.
func ProvideInfra(cfg *config.Config) error {
	infraOnce.Do(func() {
		if err := pwClient.InitPermify(cfg.Permify.Url); err != nil {
			infraErr = err
			return
		}
		infraErr = db.InitDatabase(
			cfg.Database.Host,
			cfg.Database.Username,
			cfg.Database.Password,
			cfg.Database.Database,
			cfg.Database.Port,
		)
	})
	return infraErr
}

// ProvideArgo connects the Kubernetes and Argo CD clients, once, and returns the
// shared client. A cluster connection is required.
func ProvideArgo(cfg *config.Config) *argo.Client {
	argoOnce.Do(func() {
		gc := cfg.ArgoController.GarbageCollection
		client, err := argo.NewClientFromKubeconfig(argo.Options{
			Kubeconfig:          cfg.ArgoController.Kubeconfig,
			AppProjectNamespace: cfg.ArgoController.AppProjectNamespace,
			GC: argo.GCOptions{
				Enabled:      gc.Enabled,
				Interval:     gc.Interval,
				Timeout:      gc.Timeout,
				Delay:        gc.Delay,
				LabelMarkKey: gc.LabelMarkKey,
				Debug:        gc.Debug,
			},
		})
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to connect the Argo client")
		}
		argoClient = client
	})
	return argoClient
}

// StartGarbageCollection runs the Argo garbage collection sweep in the
// background until ctx is cancelled. It needs the database, the sweep takes a
// Postgres advisory lock so only one replica sweeps per tick and is skipped
// entirely when the sweep is disabled.
func StartGarbageCollection(ctx context.Context, cfg *config.Config) {
	if !cfg.ArgoController.GarbageCollection.Enabled {
		return
	}

	argo := ProvideArgo(cfg)

	if err := ProvideInfra(cfg); err != nil {
		log.Error().Err(err).Msg("Failed to initialize infrastructure, garbage collection not started")
		return
	}

	go gc.InitGarbageCollection(ctx, argo, db.Client)
}
