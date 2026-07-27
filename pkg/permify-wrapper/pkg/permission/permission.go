package permission

import (
	"context"
	"fmt"
	utils "github.com/super-phenix/superphenix/pkg/permify-wrapper/internal"
	"github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/entity"
	"github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/client"

	permifyPayload "buf.build/gen/go/permifyco/permify/protocolbuffers/go/base/v1"
	"github.com/rs/zerolog/log"
)

func CanAccess(ctx context.Context, tenantId, permission, entityId, subjectId string) bool {
	if !client.IsConnected() {
		return false
	}

	tuple := utils.GetTuple(permission, entityId, entity.User, subjectId)

	cr, err := client.Permission.Check(ctx, &permifyPayload.PermissionCheckRequest{
		TenantId: tenantId,
		Metadata: &permifyPayload.PermissionCheckRequestMetadata{
			SnapToken:     "",
			SchemaVersion: "",
			Depth:         20,
		},
		Entity:     tuple.Entity,
		Permission: tuple.Relation,
		Subject:    tuple.Subject,
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to check permission")
		return false
	}

	return cr.Can.String() == permifyPayload.CheckResult_CHECK_RESULT_ALLOWED.String()
}

// ListUserPermission returns an array with all Permissions allowed for a specific user
func ListUserPermission(ctx context.Context, tenantId, entityType, entityId, subjectId string) ([]string, error) {
	if !client.IsConnected() {
		return []string{}, fmt.Errorf("client not connected")
	}

	resp, err := client.Permission.SubjectPermission(ctx, &permifyPayload.PermissionSubjectPermissionRequest{
		TenantId: tenantId,
		Metadata: &permifyPayload.PermissionSubjectPermissionRequestMetadata{
			SnapToken:      "",
			SchemaVersion:  "",
			OnlyPermission: true,
			Depth:          20,
		},
		Entity: &permifyPayload.Entity{
			Type: entityType,
			Id:   entityId,
		},
		Subject: &permifyPayload.Subject{
			Type: entity.User,
			Id:   subjectId,
		},
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to list permissions")
		return []string{}, err
	}

	var permissions []string
	for perm, can := range resp.GetResults() {
		if can.String() == permifyPayload.CheckResult_CHECK_RESULT_ALLOWED.String() {
			permissions = append(permissions, perm)
		}
	}

	return permissions, nil
}
