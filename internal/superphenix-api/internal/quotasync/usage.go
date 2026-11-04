package quotasync

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/az"
	crudQuota "github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/quota"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/proxy"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	spxQuota "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/quota"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"
)

const (
	// usageTimeout bounds the call to one AZ controller, so that an unreachable AZ only
	// makes its own usage unavailable.
	usageTimeout = 10 * time.Second
	// usageConcurrency bounds the number of AZ controller calls running at once.
	usageConcurrency = 8
	// maxUsageBodySize bounds the size of a controller answer.
	maxUsageBodySize = 1 << 20
)

// controllerStatus is the body answered by GET {controller}/{orgId}/{projectId}/quota.
type controllerStatus struct {
	Hard map[string]string `json:"hard"`
	Used map[string]string `json:"used"`
}

func init() {
	crudQuota.ReadUsage = readAZUsage
}

// readAZUsage reads the quota enforced and used by a project in one AZ (see crudQuota.ReadUsage).
func readAZUsage(ctx context.Context, orgId, projectId uuid.UUID, codeAZ string) (crudQuota.AZUsage, error) {
	azCfg, err := az.GetByCode(codeAZ, orgId.String())
	if err != nil {
		return crudQuota.AZUsage{}, err
	}
	status, err := fetch(ctx, azCfg, orgId, projectId)
	if err != nil {
		return crudQuota.AZUsage{}, err
	}
	return crudQuota.AZUsage{
		Hard: spxQuota.FromKubernetesUsage(status.Hard),
		Used: spxQuota.FromKubernetesUsage(status.Used),
	}, nil
}

// GetProjectQuotaOverview returns the quota overview of a project of the organization (see
// crudQuota.GetProjectQuotaOverview) with, in each AZ, what the project currently uses there.
// Every AZ of the organization is listed, with or without an AZ quota. An AZ whose usage
// cannot be read does not fail the call: it is reported as unavailable. The AZ controllers are
// only called once the project is known to belong to the organization.
func GetProjectQuotaOverview(ctx context.Context, orgId, projectId uuid.UUID) (crudQuota.ProjectQuotaOverview, error) {
	overview, err := crudQuota.GetProjectQuotaOverview(ctx, orgId, projectId)
	if err != nil {
		return overview, err
	}

	azs := targets(orgId, nil)
	log := logger.GetLogger(ctx)
	g := new(errgroup.Group)
	g.SetLimit(usageConcurrency)
	for i := range overview.AZQuotas {
		azq := &overview.AZQuotas[i]
		idx := slices.IndexFunc(azs, func(a config.AZConfig) bool { return a.Code == azq.CodeAZ })
		if idx < 0 {
			// AZ quota of an AZ no longer available to the organization.
			continue
		}
		g.Go(func() error {
			status, err := fetch(ctx, azs[idx], orgId, projectId)
			if err != nil {
				log.Err(err).Str("az", azq.CodeAZ).Str("projectId", projectId.String()).Msg("failed to get quota usage")
				return nil
			}
			azq.Available = true
			azq.Used = spxQuota.FromKubernetesUsage(status.Used).Resources()
			return nil
		})
	}
	_ = g.Wait()
	return overview, nil
}

// fetch reads the ResourceQuota of a project from the controller of an AZ.
func fetch(ctx context.Context, azCfg config.AZConfig, orgId, projectId uuid.UUID) (controllerStatus, error) {
	var status controllerStatus
	ctx, cancel := context.WithTimeout(ctx, usageTimeout)
	defer cancel()

	url := fmt.Sprintf("%s/%s/%s/%s", azCfg.ControllerUrl, orgId, projectId, quotaEndpoint)
	resp, err := proxy.SendRequest(ctx, url, http.MethodGet, nil, azCfg.AuthSecret)
	if err != nil {
		return status, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return status, fmt.Errorf("controller answered %d", resp.StatusCode)
	}
	err = json.NewDecoder(io.LimitReader(resp.Body, maxUsageBodySize)).Decode(&status)
	return status, err
}
