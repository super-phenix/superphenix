package schema

import (
	"context"
	"fmt"

	"github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/client"

	permifyPayload "buf.build/gen/go/permifyco/permify/protocolbuffers/go/base/v1"
)

func Write(ctx context.Context, tenantId string, schema string) error {
	if !client.IsConnected() {
		return fmt.Errorf("client not connected")
	}

	_, err := client.Schema.Write(ctx, &permifyPayload.SchemaWriteRequest{
		TenantId: tenantId,
		Schema:   schema,
	})

	return err
}
