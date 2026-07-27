package group

import (
	"encoding/json"
	"net/http"

	groupDb "github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/group"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/project"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/utils"
	httpModel "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/model"
	"github.com/super-phenix/superphenix/pkg/utils/decoder"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	"context"

	"github.com/go-chi/chi/v5"

	ch "github.com/super-phenix/superphenix/pkg/chi-helper"

	pw "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg"
	v1 "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1"
	v1PSet "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/permissionSet"

	"github.com/google/uuid"
)

type UpdateOrganizationGroupBody struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name" validate:"max=63"`
	// If true, then ignore ProjectIds
	AllProjects bool `json:"allProjects,omitempty"`
	// List all projects concerned by the Group
	ProjectIds     []string `json:"projectIds,omitempty"`
	PermissionSets []string `json:"permissionSets,omitempty"`
}

// ListPermissionSets
//
//	@Summary		List permission sets
//	@Description	List of permission sets
//	@Tags			v1, iam
//	@Produce		json
//	@Param			orgaId	path		string				true	"Organization ID"
//	@Success		200		{object}	map[string][]string	"Permission Sets"
//	@Router			/v1/organization/{orgaId}/iam/permissionSets [get]
//	@Security		Bearer[OrganizationRead, OrganizationIAMRead]
func (h *Service) ListPermissionSets(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	marshal, err := json.Marshal(v1PSet.EntityPermissionSetsMap)
	if err != nil {
		log.Error().Err(err).Msg("Error marshalling response")
	}
	ch.Data(w, http.StatusOK, ch.MIMEJSON, marshal)
}

// GetAllOrganizationGroups
//
//	@Summary		List organization IAM groups
//	@Description	List organization IAM groups
//	@Tags			v1, iam
//	@Produce		json
//	@Param			orgaId	path	string			true	"Organization ID"
//	@Success		200		{array}	model.APIGroup	"Groups"
//	@Failure		500
//	@Router			/v1/organization/{orgaId}/iam/group [get]
//	@Security		Bearer[OrganizationRead, OrganizationIAMRead]
func (h *Service) GetAllOrganizationGroups(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	orgaId := chi.URLParam(r, "orgaId")

	groups, err := groupDb.FindAllByOrgaIdExceptOwner(orgaId)
	if err != nil {
		log.Error().Err(err).Str("orgaId", orgaId).Msg("Failed to retrieve all groups")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	marshal, err := json.Marshal(groups)
	if err != nil {
		log.Error().Err(err).Msg("Error marshalling response")
	}
	ch.Data(w, http.StatusOK, ch.MIMEJSON, marshal)
}

// GetOrganizationGroup
//
//	@Summary		Get an IAM group by ID
//	@Description	Get an IAM group by ID
//	@Tags			v1, iam
//	@Produce		json
//	@Param			orgaId	path		string			true	"Organization ID"
//	@Param			groupId	path		string			true	"Group ID"
//	@Success		200		{object}	model.APIGroup	"Group"
//	@Failure		500
//	@Router			/v1/organization/{orgaId}/iam/group/{groupId} [get]
//	@Security		Bearer[OrganizationRead, OrganizationIAMRead]
func (h *Service) GetOrganizationGroup(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	orgaId := chi.URLParam(r, "orgaId")
	groupId := chi.URLParam(r, "groupId")

	group, err := groupDb.FindByIdAndOrgaId(orgaId, groupId)
	if err != nil {
		log.Error().Err(err).Str("orgaId", orgaId).Str("groupId", groupId).Msg("Failed to find group")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	marshal, err := json.Marshal(group)
	if err != nil {
		log.Error().Err(err).Msg("Error marshalling response")
	}
	ch.Data(w, http.StatusOK, ch.MIMEJSON, marshal)
}

// CreateOrUpdateOrganizationGroup
//
//	@Summary		Create Or Update an IAM group
//	@Description	Create Or Update an IAM group
//	@Tags			v1, iam
//	@Accept			json
//	@Produce		json
//	@Param			orgaId	path		string						true	"Organization ID"
//	@Param			Body	body		UpdateOrganizationGroupBody	true	"Group"
//	@Success		200		{object}	model.APIGroup				"Group"
//	@Failure		400
//	@Failure		403
//	@Failure		500
//	@Router			/v1/organization/{orgaId}/iam/group [post]
//	@Security		Bearer[OrganizationRead, OrganizationIAMWrite]
func (h *Service) CreateOrUpdateOrganizationGroup(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	orgaId := chi.URLParam(r, "orgaId")
	orgaUuid, _ := uuid.Parse(orgaId)

	var body UpdateOrganizationGroupBody
	if err := decoder.HandleHTTPJSON(w, r, &body, h.cfg.PublicHTTP.MaxBodySize); err != nil {
		return
	}

	// If we have an ID, then do some check
	if body.ID != uuid.Nil {
		group, err := groupDb.FindByIdAndOrgaId(body.ID.String(), orgaId)
		if err != nil {
			log.Error().Err(err).Msg("Failed to find group")
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		// Hard limit, we don't allow modification on admin group
		if group.Name == v1.DefaultGroupOwnerName {
			const reason = "Cannot update owner group"
			log.Error().Err(err).Msg(reason)
			http.Error(w, reason, http.StatusForbidden)
			return
		}
	} else {
		// New group - Check quota
		reached, err := groupDb.IsQuotaCreationReached(r.Context(), orgaUuid)
		if err != nil {
			log.Error().Err(err).Msg("Failed to check quota")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if reached {
			log.Error().Msg("Quota to create group reached")
			http.Error(w, "Quota to create group reached", http.StatusBadRequest)
			return
		}
	}
	// No problem found !
	var cast model.Group
	if err := utils.Cast(body, &cast); err != nil {
		log.Error().Err(err).Msg("Failed to cast body")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	// Set orgaId matching with the url called
	cast.OrgaId = orgaUuid
	// As this is a hidden relationship, we have to re-add it manually each time.
	cast.PermissionSets = append(cast.PermissionSets, v1PSet.SpxMember)
	group, err := SaveGroup(r.Context(), cast)
	if err != nil {
		log.Error().Err(err).Any("group", body).Msg("Failed to save group")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	var result httpModel.APIGroup
	if err := utils.Cast(group, &result); err != nil {
		log.Error().Err(err).Msg("Failed to cast body")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	marshal, err := json.Marshal(result)
	if err != nil {
		log.Error().Err(err).Msg("Error marshalling response")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	ch.Data(w, http.StatusOK, ch.MIMEJSON, marshal)

}

// DeleteOrganizationGroup
//
//	@Summary		Delete an IAM group
//	@Description	Delete an IAM group
//	@Tags			v1, iam
//	@Produce		json
//	@Param			orgaId	path	string	true	"Organization ID"
//	@Param			groupId	query	string	true	"Group ID"
//	@Success		200
//	@Failure		400
//	@Failure		403
//	@Failure		500
//	@Router			/v1/organization/{orgaId}/iam/group [delete]
//	@Security		Bearer[OrganizationRead, OrganizationIAMWrite]
func (h *Service) DeleteOrganizationGroup(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	orgaId := chi.URLParam(r, "orgaId")
	groupId, ok := ch.GetQuery(r, "groupId")
	if !ok {
		log.Error().Msg("Cannot find group id")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	group, err := groupDb.FindByIdAndOrgaId(groupId, orgaId)
	if err != nil {
		log.Error().Err(err).Msg("Failed to find group")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Hard limit, we don't allow modification on admin group
	if group.Name == v1.DefaultGroupOwnerName {
		const reason = "Cannot update owner group"
		log.Error().Err(err).Msg(reason)
		http.Error(w, reason, http.StatusForbidden)
		return
	}

	count, err := groupDb.CountUserAssignedToGroup(groupId, orgaId)
	if err != nil {
		log.Error().Err(err).Msg("Failed to count user assigned group")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if count > 0 {
		const reason = "Cannot delete group if some user are still assigned"
		log.Error().Err(err).Msg(reason)
		http.Error(w, reason, http.StatusBadRequest)
		return
	}

	if err := groupDb.DeleteById(group.ID); err != nil {
		log.Error().Err(err).Any("group", group).Msg("Failed to delete group")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// InitializeDefaultGroups Initialize default groups for organization, and add user as admin
func InitializeDefaultGroups(ctx context.Context, orgaUuid, ownerUuid uuid.UUID) error {

	// Default Owner
	ownerGroup, err := SaveGroup(ctx, model.Group{
		Name:           v1.DefaultGroupOwnerName,
		OrgaId:         orgaUuid,
		AllProjects:    true,
		PermissionSets: append(v1.DefaultOrganizationOwner, v1.DefaultProjectOwner...),
	})
	if err != nil {
		return err
	}

	// Default Admin
	_, err = SaveGroup(ctx, model.Group{
		Name:           v1.DefaultGroupAdminName,
		OrgaId:         orgaUuid,
		AllProjects:    true,
		PermissionSets: append(v1.DefaultOrganizationAdmin, v1.DefaultProjectAdmin...),
	})
	if err != nil {
		return err
	}

	// Default Billing
	_, err = SaveGroup(ctx, model.Group{
		Name:           v1.DefaultGroupBillingName,
		OrgaId:         orgaUuid,
		AllProjects:    true,
		PermissionSets: append(v1.DefaultOrganizationBilling, v1.DefaultProjectBilling...),
	})
	if err != nil {
		return err
	}

	// Default Developer
	_, err = SaveGroup(ctx, model.Group{
		Name:           v1.DefaultGroupDeveloperName,
		OrgaId:         orgaUuid,
		AllProjects:    true,
		PermissionSets: append(v1.DefaultOrganizationDeveloper, v1.DefaultProjectDeveloper...),
	})
	if err != nil {
		return err
	}

	if err := pw.CreateOrUpdateRelationUserGroup(ctx, ownerGroup.OrgaId.String(), ownerUuid.String(), []string{ownerGroup.ID.String()}); err != nil {
		return err
	}

	return nil
}

// SaveGroup save a permission group in DB and Permify (update or create if ID not provided)
func SaveGroup(ctx context.Context, group model.Group) (model.Group, error) {
	log := logger.GetLogger(ctx)
	group, err := groupDb.Save(group)

	if err != nil {
		log.Error().Err(err).Any("group", group).Msgf("Error initializing group %s", group.Name)
		return model.Group{}, err
	}

	projectIds := group.ProjectIds
	if group.AllProjects {
		projectIds = []string{}
		projects, _ := project.FindAllByOrgaId(group.OrgaId.String())
		for _, p := range projects {
			projectIds = append(projectIds, p.ID.String())
		}
	}

	if err := pw.UpdateRelationGroupPermissions(ctx, group.OrgaId.String(), pw.Group{
		Id:             group.ID.String(),
		OrgaId:         group.OrgaId.String(),
		ProjectIds:     projectIds,
		PermissionSets: group.PermissionSets,
	}); err != nil {
		return model.Group{}, err
	}

	return group, nil
}
