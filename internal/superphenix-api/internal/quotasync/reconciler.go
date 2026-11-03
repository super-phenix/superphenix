package quotasync

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	crudQuota "github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/quota"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"
)

const (
	// pollInterval is how often the reconciler looks for quotas due for a push.
	pollInterval = 2 * time.Second
	// claimBatch bounds the quotas the reconciler claims at once.
	claimBatch = 50
	// lease is how long a claimed quota stays reserved to its reconciler. Past it, another
	// reconciler can take it over.
	lease = time.Minute
	// pushConcurrency bounds the number of AZ controller push calls running at once.
	pushConcurrency = 8
	// retryBackoff and maxRetryBackoff bound the delay before the reconciler retries a failed push.
	retryBackoff    = 5 * time.Second
	maxRetryBackoff = 5 * time.Minute
)

// prepareSlots bounds the Prepare transactions running at once in this replica, each holding a
// database connection while it waits for the lock of its organization.
var prepareSlots = make(chan struct{}, 2)

// Reconciler pushes the quotas due to the AZ controllers. Each API replica runs one, and one
// reconciler at a time holds a claimed quota.
type Reconciler struct {
	Interval time.Duration
	// EnsureInterval is how often Run calls Ensure.
	EnsureInterval time.Duration
	// Ensure creates the missing sync states, at start and every EnsureInterval.
	Ensure func(ctx context.Context) error
	// Claim claims up to limit quotas due, for token.
	Claim func(ctx context.Context, token uuid.UUID, limit int) ([]crudQuota.StateKey, error)
	// Prepare prepares the pushes of quotas of one organization claimed with token.
	Prepare func(ctx context.Context, token, organizationId uuid.UUID, keys []crudQuota.StateKey) ([]crudQuota.Push, error)
	// Push sends a prepared push to its AZ.
	Push func(ctx context.Context, push crudQuota.Push) error
	// Record records the outcome of a push.
	Record func(ctx context.Context, push crudQuota.Push, pushErr error) error
	// Applied runs for each project with an applied push.
	Applied func(ctx context.Context, orgId, projectId uuid.UUID) error
}

// NewReconciler returns the Reconciler of the database and the AZ controllers.
func NewReconciler(cfg config.QuotaResyncConfig) *Reconciler {
	return &Reconciler{
		Interval:       pollInterval,
		EnsureInterval: cfg.Interval,
		Ensure:         crudQuota.EnsureStates,
		Claim: func(ctx context.Context, token uuid.UUID, limit int) ([]crudQuota.StateKey, error) {
			return crudQuota.Claim(ctx, token, configuredAZCodes(), limit, lease)
		},
		Prepare: func(ctx context.Context, token, organizationId uuid.UUID, keys []crudQuota.StateKey) ([]crudQuota.Push, error) {
			return crudQuota.Prepare(ctx, token, organizationId, keys, cfg.Interval)
		},
		Push: pushToAZ,
		Record: func(ctx context.Context, push crudQuota.Push, pushErr error) error {
			return crudQuota.Record(ctx, push, pushErr, cfg.Interval, retryBackoff, maxRetryBackoff)
		},
		Applied: reactivate,
	}
}

// configuredAZCodes returns the codes of every AZ of the configuration.
func configuredAZCodes() []string {
	codes := make([]string, 0, len(config.Global.AZs))
	for code := range config.Global.AZs {
		codes = append(codes, code)
	}
	return codes
}

// Run pushes the quotas due every Interval until ctx is done. It calls Ensure at start and every
// EnsureInterval.
func (r *Reconciler) Run(ctx context.Context) {
	ctx = context.WithValue(ctx, logger.ProcessIdKey, fmt.Sprintf("quota-reconciler-%s", uuid.NewString()))
	processLog := logger.GetProcessLogger(ctx)
	processLog.Info().Msgf("Initialize quota reconciler - running every %s", r.Interval)
	ensure := func() {
		if err := r.Ensure(ctx); err != nil {
			processLog.Err(err).Msg("failed to create the missing quota sync states")
		}
	}
	ensure()

	ticker := time.NewTicker(r.Interval)
	defer ticker.Stop()
	ensureTicker := time.NewTicker(r.EnsureInterval)
	defer ensureTicker.Stop()
	for {
		r.ReconcileDue(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ensureTicker.C:
			ensure()
		case <-ticker.C:
		}
	}
}

// ReconcileDue claims and pushes the quotas due, batch after batch, until none is left.
func (r *Reconciler) ReconcileDue(ctx context.Context) {
	for ctx.Err() == nil {
		token := uuid.New()
		keys, err := r.Claim(ctx, token, claimBatch)
		if err != nil {
			processLog := logger.GetProcessLogger(ctx)
			processLog.Err(err).Msg("failed to claim the quotas due")
			return
		}
		if len(keys) == 0 {
			return
		}
		r.process(ctx, token, keys)
		if len(keys) < claimBatch {
			return
		}
	}
}

// process prepares, pushes and records the quotas of keys claimed with token, one organization
// after another. It returns true when the AZs applied all of them.
func (r *Reconciler) process(ctx context.Context, token uuid.UUID, keys []crudQuota.StateKey) bool {
	log := logger.GetLogger(ctx)
	organizationIds, groups := crudQuota.GroupByOrganization(keys)
	applied := true
	for _, organizationId := range organizationIds {
		group := groups[organizationId]
		prepareSlots <- struct{}{}
		pushes, err := r.Prepare(ctx, token, organizationId, group)
		<-prepareSlots
		if err != nil {
			log.Err(err).Str("organizationId", organizationId.String()).Msg("failed to prepare the quota pushes")
			applied = false
			continue
		}
		if len(pushes) < len(group) {
			applied = false
		}
		if !r.pushAll(ctx, pushes) {
			applied = false
		}
	}
	return applied
}

// pushAll pushes and records pushes concurrently, then calls Applied for every project with an
// applied push. It returns true when every push was applied and recorded.
func (r *Reconciler) pushAll(ctx context.Context, pushes []crudQuota.Push) bool {
	log := logger.GetLogger(ctx)
	var mutex sync.Mutex
	appliedProjects := map[[2]uuid.UUID]bool{}
	allApplied := true

	g := new(errgroup.Group)
	g.SetLimit(pushConcurrency)
	for _, prepared := range pushes {
		g.Go(func() error {
			pushErr := r.Push(ctx, prepared)
			if pushErr != nil {
				log.Warn().Err(pushErr).Str("az", prepared.CodeAZ).Str("projectId", prepared.ProjectId.String()).
					Int64("revision", prepared.Revision).Msg("failed to push quota")
			}
			recordErr := r.Record(ctx, prepared, pushErr)
			if recordErr != nil {
				log.Err(recordErr).Str("az", prepared.CodeAZ).Str("projectId", prepared.ProjectId.String()).
					Msg("failed to record the quota push")
			}
			mutex.Lock()
			defer mutex.Unlock()
			if pushErr != nil || recordErr != nil {
				allApplied = false
				return nil
			}
			appliedProjects[[2]uuid.UUID{prepared.OrganizationId, prepared.ProjectId}] = true
			return nil
		})
	}
	_ = g.Wait()

	for project := range appliedProjects {
		if err := r.Applied(ctx, project[0], project[1]); err != nil {
			log.Err(err).Str("projectId", project[1].String()).Msg("failed to reactivate the project")
		}
	}
	return allApplied
}
