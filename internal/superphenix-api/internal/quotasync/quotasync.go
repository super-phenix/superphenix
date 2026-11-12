// Package quotasync forwards the quotas stored by the API to the controller of each AZ,
// which enforces them through a Kubernetes ResourceQuota in the project namespace.
//
// The API database is the source of truth: a push sends the whole effective quota of a
// project in an AZ (never a diff), so pushing again is always safe and fixes any drift.
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
	"golang.org/x/sync/errgroup"

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

	// pushConcurrency bounds the number of AZ controller push calls running at once.
	pushConcurrency = 8
)

// ErrNotApplied is returned when the quota is saved but could not be pushed to every AZ.
var ErrNotApplied = errors.New("quota saved but not applied on every availability zone, retry the request")

// controllerBody is the body of PUT {controller}/{orgId}/{projectId}/quota.
type controllerBody struct {
	Hard     map[string]string `json:"hard"`
	Revision int64             `json:"revision,omitempty"`
}

// Project pushes the quota of a project to the given AZs (every AZ of the organization
// when none is given).
//
// When every AZ is targeted and the push succeeds, the project is reactivated if it was
// deactivated because its quota could not be applied (see Initialize).
//
// The push is not canceled with ctx (e.g. when the client disconnects), so that a quota
// saved in the database is not left half applied.
func Project(ctx context.Context, orgId, projectId uuid.UUID, azCodes ...string) error {
	ctx = context.WithoutCancel(ctx)
	failed, _, err := apply(ctx, orgId, projectId, azCodes)
	if err != nil {
		return err
	}
	if len(failed) > 0 {
		return fmt.Errorf("%w: %s", ErrNotApplied, strings.Join(failed, ", "))
	}
	if len(azCodes) == 0 {
		if err := crudProject.Reactivate(ctx, projectId, model.DeactivationQuotaFailedToApply); err != nil {
			return err
		}
	}
	return nil
}

// Initialize pushes the quota of a new project to every AZ. Project creation does not fail
// when the quota cannot be applied: the project is kept but deactivated (every operation on
// its resources is refused) until a later push of its whole quota succeeds, e.g. by setting
// its project quota or the organization quota again. A project with nothing to enforce in
// the failing AZs is not deactivated.
//
// It is not canceled with ctx: otherwise a client disconnecting during the creation would make
// both the push and the deactivation fail, leaving an active project without any quota.
func Initialize(ctx context.Context, orgId, projectId uuid.UUID) error {
	ctx = context.WithoutCancel(ctx)
	failed, unenforced, err := apply(ctx, orgId, projectId, nil)
	if err == nil && len(failed) > 0 {
		err = fmt.Errorf("%w: %s", ErrNotApplied, strings.Join(failed, ", "))
	}
	if err == nil {
		return nil
	}

	// An error without any failed AZ means the quota could not even be computed.
	if len(failed) == 0 || len(unenforced) > 0 {
		msg := "quota not applied"
		if len(unenforced) > 0 {
			msg += " on: " + strings.Join(unenforced, ", ")
		}
		if errDeactivate := crudProject.Deactivate(ctx, projectId, model.DeactivationQuotaFailedToApply, msg); errDeactivate != nil {
			return errors.Join(err, errDeactivate)
		}
	}
	return err
}

// apply pushes the quota of a project to the given AZs (every AZ when none is given). It
// returns the AZs where the push failed, and among them those where a quota had to be
// enforced (a failed push of an empty quota leaves at worst a stricter quota in place).
func apply(ctx context.Context, orgId, projectId uuid.UUID, azCodes []string) (failed, unenforced []string, err error) {
	h, err := crudQuota.GetHierarchy(ctx, orgId, projectId)
	if err != nil {
		return nil, nil, err
	}

	targetAZs := targets(orgId, azCodes)
	if len(targetAZs) == 0 {
		return nil, nil, nil
	}

	type azResult struct {
		code       string
		failed     bool
		unenforced bool
	}

	results := make([]azResult, len(targetAZs))
	log := logger.GetLogger(ctx)
	g := new(errgroup.Group)
	g.SetLimit(pushConcurrency)

	for i := range targetAZs {
		azCfg := targetAZs[i]
		hard := h.AZ(azCfg.Code).ToKubernetesQuota()
		rev := h.Revision(azCfg.Code)
		g.Go(func() error {
			if err := push(ctx, azCfg, orgId, projectId, hard, rev); err != nil {
				log.Err(err).Str("az", azCfg.Code).Str("projectId", projectId.String()).Msg("failed to apply quota")
				results[i] = azResult{
					code:       azCfg.Code,
					failed:     true,
					unenforced: len(hard) > 0,
				}
			}
			return nil
		})
	}
	_ = g.Wait()

	for _, res := range results {
		if res.failed {
			failed = append(failed, res.code)
			if res.unenforced {
				unenforced = append(unenforced, res.code)
			}
		}
	}
	return failed, unenforced, nil
}

// Organization pushes the quota of every project of the organization to every AZ.
func Organization(ctx context.Context, orgId uuid.UUID) error {
	ctx = context.WithoutCancel(ctx)
	ids, err := crudQuota.ListProjectIds(ctx, orgId)
	if err != nil {
		return err
	}
	var errs []error
	for _, projectId := range ids {
		if err := Project(ctx, orgId, projectId); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%w (%d project(s))", ErrNotApplied, len(errs))
	}
	return nil
}

func targets(orgId uuid.UUID, azCodes []string) []config.AZConfig {
	azs := az.FindAll(orgId.String())
	if len(azCodes) > 0 {
		azs = slices.DeleteFunc(azs, func(a config.AZConfig) bool { return !slices.Contains(azCodes, a.Code) })
	}
	slices.SortFunc(azs, func(a, b config.AZConfig) int { return strings.Compare(a.Code, b.Code) })
	return azs
}

func init() {
	crudQuota.PushAZ = pushAZ
}

// pushAZ pushes the Kubernetes quota of a project to one AZ (see crudQuota.PushAZ).
func pushAZ(ctx context.Context, orgId, projectId uuid.UUID, codeAZ string, hard map[string]string) error {
	azCfg, err := az.GetByCode(codeAZ, orgId.String())
	if err != nil {
		return err
	}
	return push(ctx, azCfg, orgId, projectId, hard, 0)
}

// push sends the Kubernetes quota of a project to the controller of an AZ. An empty
// quota removes the ResourceQuota. A limit lowered below its usage is refused by the
// controller (409), reported as crudQuota.ErrBelowUsage.
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
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if resp.StatusCode == http.StatusConflict {
			return fmt.Errorf("%w in %s", crudQuota.ErrBelowUsage, azCfg.Code)
		}
		return fmt.Errorf("controller answered %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return nil
}
