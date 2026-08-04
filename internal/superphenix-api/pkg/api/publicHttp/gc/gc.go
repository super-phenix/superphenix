package gc

import (
	"context"
	"fmt"
	"net/http"

	argogc "github.com/super-phenix/superphenix/internal/superphenix-api/internal/argo/gc"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/az"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/proxy"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/app"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"
)

const markEndpoint = "mark"

// callMarkEndpoint sends a GC mark request to the given base URL and returns an error if the request fails.
func callMarkEndpoint(ctx context.Context, baseUrl, orgId, projectId, authSecret, serviceName string) error {
	log := logger.GetLogger(ctx)

	url := fmt.Sprintf("%s/%s/%s/%s", baseUrl, orgId, projectId, markEndpoint)
	resp, err := proxy.SendRequest(ctx, url, "GET", http.NoBody, authSecret)
	if err != nil {
		log.Err(err).Str("url", url).Msgf("failed to send request on gc %s", serviceName)
		return fmt.Errorf("failed to send request on gc %s", serviceName)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		log.Err(err).Str("projectId", projectId).Str("url", url).Int("respCode", resp.StatusCode).Msgf("failed to mark resources on %s", serviceName)
		return fmt.Errorf("failed to mark resources on %s", serviceName)
	}

	return nil
}

// MarkResources call Garbage Collector on each AZs to mark every resource on the given project for deletion
func MarkResources(ctx context.Context, orgId, projectId string) error {
	// Mark resources on each SPX Controller (one per AZ)
	azs := az.FindAll(orgId)

	for _, azCfg := range azs {
		if err := callMarkEndpoint(ctx, azCfg.ControllerUrl, orgId, projectId, azCfg.AuthSecret, azCfg.Code); err != nil {
			return err
		}
	}

	// Mark Argo resources in-process. Marking is not gated on
	// garbageCollection.enabled: a project must still be marked on deletion even
	// where the sweep loop is off.
	argoClient := app.ProvideArgo(&config.Global)

	return argogc.Mark(ctx, argoClient, argoClient.Namespace(projectId))
}
