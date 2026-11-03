// Package quotasync pushes the quotas stored by the API to the controller of each AZ, which
// enforces them through a Kubernetes ResourceQuota in the project namespace.
//
// A push sends the whole effective quota of a project in an AZ. A quota write marks the quotas it
// changes as dirty, and the request pushes them with Apply. The Reconciler of each API replica
// retries the failed pushes and pushes each applied quota again after the resync interval.
package quotasync

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/az"
	crudProject "github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/project"
	crudQuota "github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/quota"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/proxy"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"
)

const (
	quotaEndpoint = "quota"

	// pushTimeout bounds the call to one AZ controller when pushing a quota.
	pushTimeout = 10 * time.Second
)

// ErrNotApplied means the API saved the quota but some AZ has not applied it yet.
var ErrNotApplied = errors.New("quota saved but not applied on every availability zone yet")

// controllerBody is the body of PUT {controller}/{orgId}/{projectId}/quota.
type controllerBody struct {
	Hard     map[string]string `json:"hard"`
	Revision int64             `json:"revision,omitempty"`
}

// Scope selects the quotas a write changed: those of an organization, of one of its projects
// (ProjectId), or of a project in one AZ (ProjectId and CodeAZ).
type Scope struct {
	OrganizationId uuid.UUID
	ProjectId      *uuid.UUID
	CodeAZ         string
}

// Apply pushes the changes of scope that the AZs have not applied yet and waits for them at most
// quota.applyWait. It returns true when the AZs applied them, and false when a push failed or is
// still running in the background. Canceling ctx does not cancel the pushes.
func Apply(ctx context.Context, scope Scope) bool {
	ctx = context.WithoutCancel(ctx)
	reconciler := NewReconciler(config.Global.Quota.Resync)
	done := make(chan bool, 1)
	go func() {
		token := uuid.New()
		keys, err := crudQuota.ClaimDirty(ctx, token, scope.OrganizationId, scope.ProjectId, scope.CodeAZ, lease)
		if err != nil {
			log := logger.GetLogger(ctx)
			log.Err(err).Str("organizationId", scope.OrganizationId.String()).Msg("failed to claim the quotas to push")
			done <- false
			return
		}
		done <- reconciler.process(ctx, token, keys)
	}()

	timer := time.NewTimer(config.Global.Quota.ApplyWait)
	defer timer.Stop()
	select {
	case applied := <-done:
		return applied
	case <-timer.C:
		return false
	}
}

// Initialize pushes the quota of a new project to every AZ of its organization. It deactivates the
// project (model.DeactivationQuotaFailedToApply) when an AZ has limits to enforce or when it cannot
// compute the quota. The deactivation lasts until the AZs apply the quota. It returns
// ErrNotApplied when some AZ did not apply the quota within quota.applyWait.
func Initialize(ctx context.Context, orgId, projectId uuid.UUID) error {
	ctx = context.WithoutCancel(ctx)
	if err := crudQuota.MarkNewProject(ctx, orgId, projectId); err != nil {
		if errDeactivate := crudProject.Deactivate(ctx, projectId, model.DeactivationQuotaFailedToApply, "quota not applied"); errDeactivate != nil {
			return errors.Join(err, errDeactivate)
		}
		return err
	}
	if !Apply(ctx, Scope{OrganizationId: orgId, ProjectId: &projectId}) {
		return ErrNotApplied
	}
	return nil
}

// reactivate lifts the DeactivationQuotaFailedToApply of a project once every AZ with limits to
// enforce applied its quota.
func reactivate(ctx context.Context, orgId, projectId uuid.UUID) error {
	deactivations, err := crudProject.ListDeactivations(ctx, orgId, projectId)
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(deactivations, func(deactivation model.ProjectDeactivation) bool {
		return deactivation.Reason == model.DeactivationQuotaFailedToApply
	}) {
		return nil
	}
	unapplied, err := crudQuota.UnappliedAZs(ctx, orgId, projectId)
	if err != nil || len(unapplied) > 0 {
		return err
	}
	return crudProject.Reactivate(ctx, projectId, model.DeactivationQuotaFailedToApply)
}

// pushToAZ sends a prepared push to the controller of its AZ.
func pushToAZ(ctx context.Context, prepared crudQuota.Push) error {
	azCfg, err := az.GetByCode(prepared.CodeAZ, prepared.OrganizationId.String())
	if err != nil {
		return err
	}
	return push(ctx, azCfg, prepared.OrganizationId, prepared.ProjectId, prepared.Hard, prepared.Revision)
}

// push sends the Kubernetes quota of a project to the controller of an AZ at a quota revision of
// its organization. An empty quota clears the limits of the ResourceQuota. The controller answers
// 409 to a limit below its usage, which push returns as crudQuota.ErrBelowUsage, and 412 to a push
// older than the revision it holds.
func push(ctx context.Context, azCfg config.AZConfig, orgId, projectId uuid.UUID, hard map[string]string, revision int64) error {
	body, err := json.Marshal(controllerBody{Hard: hard, Revision: revision})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, pushTimeout)
	defer cancel()
	url := fmt.Sprintf("%s/%s/%s/%s", azCfg.ControllerUrl, orgId, projectId, quotaEndpoint)
	resp, err := proxy.SendRequest(ctx, url, http.MethodPut, bytes.NewReader(body), azCfg.AuthSecret)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if resp.StatusCode == http.StatusConflict {
			return fmt.Errorf("%w in %s", crudQuota.ErrBelowUsage, azCfg.Code)
		}
		return fmt.Errorf("controller answered %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return nil
}
