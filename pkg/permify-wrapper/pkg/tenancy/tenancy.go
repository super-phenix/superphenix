package tenancy

import (
	"context"
	"fmt"
	"github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/client"

	permifyPayload "buf.build/gen/go/permifyco/permify/protocolbuffers/go/base/v1"
	"github.com/rs/zerolog/log"
)

// Create creates a new tenant in Permify. Note: multi-tenancy has no impact for now.
func Create(ctx context.Context, tenantId, tenantName string) error {
	if !client.IsConnected() {
		return fmt.Errorf("client not connected")
	}

	_, err := client.Tenancy.Create(ctx, &permifyPayload.TenantCreateRequest{
		Id:   tenantId,
		Name: tenantName,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to create tenant")
	}
	return err
}

// Delete allow to remove a tenant from Permify
func Delete(ctx context.Context, tenantId string) error {
	if !client.IsConnected() {
		return fmt.Errorf("client not connected")
	}

	_, err := client.Tenancy.Delete(ctx, &permifyPayload.TenantDeleteRequest{
		Id: tenantId,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to delete tenant")
	}
	return err
}
