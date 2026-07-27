package permission

import (
	"encoding/json"
	"net/http"
	"slices"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/iam/group"
	"github.com/super-phenix/superphenix/pkg/utils/decoder"

	ch "github.com/super-phenix/superphenix/pkg/chi-helper"

	v1 "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1"
	"github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/schema"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

const batchSize = 20

type UpdateDefaultSchemaBody struct {
	RenamedRelations []struct {
		Old string
		New string
	}
	DeletedRelations []string
}
type UpdateDefaultSchemaResponse struct {
	OrganizationUpdated   int                       `json:"organization_updated"`
	OrganizationFailedIds []uuid.UUID               `json:"organizationFailedIds"`
	GroupFailedIds        map[uuid.UUID][]uuid.UUID `json:"groupFailedIds"`
	GroupMissingIds       map[uuid.UUID][]uuid.UUID `json:"groupMissingIds"`
}

// UpdateDefaultSchema update permission schema
//
//	@Summary		Update permission schema
//	@Description	Update permission schema in database and Permify
//	@Tags			Admin endpoint
//	@Accept			json
//	@Produce		json
//	@Param			message	body		UpdateDefaultSchemaBody		true	"Update Information"
//	@Success		200		{object}	UpdateDefaultSchemaResponse	"Response"
//	@Failure		400		{string}	string						"Error"
//	@Failure		500		{string}	string						"Error"
//
//	@Router			/admin/permission/update-schema [post]
func (h *Service) UpdateDefaultSchema(w http.ResponseWriter, r *http.Request) {
	var organizations []model.Organization
	var orgaSucessfulIds []uuid.UUID
	var orgaFailedIds []uuid.UUID
	groupsFailedIds := make(map[uuid.UUID][]uuid.UUID)
	groupsMissingIds := make(map[uuid.UUID][]uuid.UUID)

	var body UpdateDefaultSchemaBody
	if err := decoder.HandleHTTPJSON(w, r, &body, h.cfg.PublicHTTP.MaxBodySize); err != nil {
		return
	}

	// Update default schema for each non deleted organization
	db.Client.FindInBatches(&organizations, batchSize, func(tx *gorm.DB, batch int) error {
		// tx.RowsAffected provides the count of records in the current batch
		// The variable 'batch' indicates the current batch number

		for _, result := range organizations {
			// Operations on each record in the batch
			if err := schema.Write(r.Context(), result.ID.String(), v1.DefaultSchema); err != nil {
				orgaFailedIds = append(orgaFailedIds, result.ID)
				log.Error().Err(err).Str("orgaId", result.ID.String()).Msg("Failed to update permify schema")
			} else {
				orgaSucessfulIds = append(orgaSucessfulIds, result.ID)
			}
		}

		// Returning an error will stop further batch processing
		return nil
	})

	if len(body.DeletedRelations) > 0 || len(body.RenamedRelations) > 0 {
		var groups []model.Group
		db.Client.FindInBatches(&groups, batchSize, func(tx *gorm.DB, batch int) error {
			for _, result := range groups {

				// Try to update only if the group is
				if slices.Contains(orgaSucessfulIds, result.OrgaId) {
					// Operations on each record in the batch
					newPSets := make([]string, 0)
					if len(body.DeletedRelations) > 0 {
						for _, pSet := range result.PermissionSets {
							if !slices.Contains(body.DeletedRelations, pSet) {
								newPSets = append(newPSets, pSet)
							}
						}
					} else {
						newPSets = append(newPSets, result.PermissionSets...)
					}

					if len(body.RenamedRelations) > 0 {
						for i, pSet := range newPSets {
							for _, renamedRelation := range body.RenamedRelations {
								if pSet == renamedRelation.Old {
									newPSets[i] = renamedRelation.New
								}
							}
						}
					}

					result.PermissionSets = newPSets

					if _, err := group.SaveGroup(r.Context(), result); err != nil {
						groupsFailedIds[result.OrgaId] = append(groupsMissingIds[result.OrgaId], result.ID)
						log.Error().Err(err).Str("groupId", result.ID.String()).Msg("Failed to update group permify")
					}
				} else {
					groupsMissingIds[result.OrgaId] = append(groupsMissingIds[result.OrgaId], result.ID)
				}
			}

			// Returning an error will stop further batch processing
			return nil
		})
	}

	result := UpdateDefaultSchemaResponse{
		OrganizationUpdated:   len(orgaSucessfulIds),
		OrganizationFailedIds: orgaFailedIds,
		GroupFailedIds:        groupsFailedIds,
		GroupMissingIds:       groupsMissingIds,
	}

	d, _ := json.Marshal(result)

	ch.Data(w, http.StatusOK, ch.MIMEJSON, d)
}
