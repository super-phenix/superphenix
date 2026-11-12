package quotasync

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/advisorylock"
	crudQuota "github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/quota"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"
)

// resyncLockID identifies the Postgres advisory lock serialising the re-sync across replicas.
const resyncLockID int64 = 0x535058515553 // "SPXQUS"

// Resyncer periodically pushes the quota of every project to every AZ, so that the clusters
// converge to the database: failed pushes, AZs added to the configuration, quotas edited or
// deleted in a cluster. A successful push of the whole quota of a project also reactivates it
// when it was deactivated because its quota could not be applied (see Project).
type Resyncer struct {
	Interval time.Duration
	Timeout  time.Duration
	// Lock returns acquired false when another replica re-syncs.
	Lock func(ctx context.Context) (release func(), acquired bool, err error)
	// List lists the live projects (ID and organization ID).
	List func(ctx context.Context) ([]model.Project, error)
	// Push pushes the whole quota of a project to every AZ.
	Push func(ctx context.Context, orgId, projectId uuid.UUID) error
}

// ResyncReport counts the projects of a re-sync.
type ResyncReport struct {
	Projects int
	Failed   int
}

// NewResyncer returns the Resyncer over the database and the AZ controllers.
func NewResyncer(cfg config.QuotaResyncConfig) *Resyncer {
	return &Resyncer{
		Interval: cfg.Interval,
		Timeout:  cfg.Timeout,
		Lock: func(ctx context.Context) (func(), bool, error) {
			return advisorylock.TryLock(ctx, db.Client, resyncLockID)
		},
		List: crudQuota.ListAllProjects,
		// No organization lock is needed: the quota revision stored in the DB and passed
		// to the controller ensures the controller ignores stale pushes from a read that
		// was overtaken by a concurrent write.
		Push: func(ctx context.Context, orgId, projectId uuid.UUID) error {
			return Project(ctx, orgId, projectId)
		},
	}
}

// Run re-syncs on a timer until ctx is cancelled.
func (r *Resyncer) Run(ctx context.Context) {
	log.Info().Msgf("Initialize quota re-sync - running every : %s and timeout after %s", r.Interval, r.Timeout)

	ticker := time.NewTicker(r.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.resyncWithTimeout(ctx)
		}
	}
}

func (r *Resyncer) resyncWithTimeout(ctx context.Context) {
	resyncCtx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()
	resyncCtx = context.WithValue(resyncCtx, logger.ProcessIdKey, fmt.Sprintf("quota-resync-%s", uuid.NewString()))
	log := logger.GetProcessLogger(resyncCtx)

	report, err := r.Resync(resyncCtx)
	if err != nil {
		log.Error().Err(err).Int("projects", report.Projects).Int("failed", report.Failed).Msg("Quota re-sync failed")
		return
	}
	log.Info().Int("projects", report.Projects).Int("failed", report.Failed).Msg("Quota re-sync finished")
}

// Resync pushes the quota of every live project once. A project that fails (logged by the push)
// does not stop the others; an interrupted re-sync resumes from the start at the next tick.
func (r *Resyncer) Resync(ctx context.Context) (ResyncReport, error) {
	var report ResyncReport
	release, acquired, err := r.Lock(ctx)
	if err != nil {
		return report, fmt.Errorf("acquire re-sync lock: %w", err)
	}
	if !acquired {
		return report, nil
	}
	defer release()

	projects, err := r.List(ctx)
	if err != nil {
		return report, fmt.Errorf("list projects: %w", err)
	}
	for _, p := range projects {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		report.Projects++
		if err := r.Push(ctx, p.OrgaId, p.ID); err != nil {
			report.Failed++
		}
	}
	return report, nil
}
