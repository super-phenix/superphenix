package eip

import (
	"context"

	logger "github.com/super-phenix/superphenix/pkg/utils/log"
)

// cleanEip deletes the matching EIP, used when IpTablesRule creation fails.
func cleanEip(ctx context.Context, namespace, eipEID string) {
	log := logger.GetLogger(ctx)
	if err := DeleteEip(ctx, namespace, eipEID); err != nil {
		log.Err(err).Any("eipEID", eipEID).Msg("Failed to remove EIP after creation failure")
	}
}
