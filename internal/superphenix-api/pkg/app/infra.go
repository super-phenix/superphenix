// Package app holds the infrastructure bootstrap (database, Permify) shared by the API.
package app

import (
	"sync"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"

	pwClient "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/client"
)

var (
	infraOnce sync.Once
	infraErr  error
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
