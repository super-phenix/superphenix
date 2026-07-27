package bundle

import (
	"context"
	"fmt"

	"github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/client"

	permifyPayload "buf.build/gen/go/permifyco/permify/protocolbuffers/go/base/v1"
	"github.com/rs/zerolog/log"
)

type DataBundle = permifyPayload.DataBundle

func Delete(ctx context.Context, tenantId, bundleName string) error {
	if !client.IsConnected() {
		return fmt.Errorf("client not connected")
	}

	_, err := client.Bundle.Delete(ctx, &permifyPayload.BundleDeleteRequest{
		TenantId: tenantId,
		Name:     bundleName,
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to create bundle")
	}
	return err
}

func Write(ctx context.Context, tenantId string, bundles []*DataBundle) error {
	if !client.IsConnected() {
		return fmt.Errorf("client not connected")
	}

	_, err := client.Bundle.Write(ctx, &permifyPayload.BundleWriteRequest{
		TenantId: tenantId,
		Bundles:  bundles,
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to create bundle")
	}
	return err
}

func Run(ctx context.Context, tenantId, name string, args map[string]string) error {
	if !client.IsConnected() {
		return fmt.Errorf("client not connected")
	}

	_, err := client.Data.RunBundle(ctx, &permifyPayload.BundleRunRequest{
		TenantId:  tenantId,
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to run bundle")
	}
	return err
}
