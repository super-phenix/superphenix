package ctrlutils

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/consts"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/proxy"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	httpError "github.com/super-phenix/superphenix/pkg/utils/error"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"
)

const generatedLabel = "superphenix.net/generated"

// AZStatusError is a non-200 answer of the AZ controller.
type AZStatusError struct {
	StatusCode int
	Message    string
}

func (e *AZStatusError) Error() string {
	return fmt.Sprintf("superphenix-controller returned %d", e.StatusCode)
}

// GetFromAZ decodes the JSON answer of a GET on the AZ controller.
func GetFromAZ(ctx context.Context, azDb config.AZConfig, url string, out any) error {
	log := logger.GetLogger(ctx)
	resp, err := proxy.SendRequest(ctx, url, http.MethodGet, http.NoBody, azDb.AuthSecret)
	if err != nil {
		log.Err(err).Str("az", azDb.Code).Msg(consts.SpxProxyToAZFailure)
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		errorBody := httpError.RetrieveHttpError(resp)
		log.Error().Any("responseBody", errorBody).
			Str("status", resp.Status).Int("statusCode", resp.StatusCode).
			Str("azCode", azDb.Code).Str("url", url).
			Msg("Request on superphenix-controller failed")
		statusErr := &AZStatusError{StatusCode: resp.StatusCode}
		if errorBody != nil {
			statusErr.Message = errorBody.Message
		}
		return statusErr
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %w", err)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("error unmarshaling response: %w", err)
	}
	return nil
}

// VIPAvailable reports whether vip is free in the VPC of the subnet, for every load balancer of the VPC,
// including the ones rendered by charts.
func VIPAvailable(ctx context.Context, azDb config.AZConfig, orgId, projectId, vip, subnetEId string) (bool, error) {
	var res struct {
		Available bool `json:"available"`
	}
	query := url.Values{"vip": {vip}, "subnetEId": {subnetEId}}
	endpoint := fmt.Sprintf("%s/%s/%s/load-balancer/vip-available?%s", azDb.ControllerUrl, orgId, projectId, query.Encode())
	if err := GetFromAZ(ctx, azDb, endpoint, &res); err != nil {
		return false, err
	}
	return res.Available, nil
}

// IsGenerated reports whether the resource under key of a controller list item carries the
// superphenix.net/generated label, set on resources a chart renders for another product.
func IsGenerated(item map[string]any, key string) bool {
	resource, _ := item[key].(map[string]any)
	metadata, _ := resource["metadata"].(map[string]any)
	labels, _ := metadata["labels"].(map[string]any)
	return labels[generatedLabel] == "true"
}
