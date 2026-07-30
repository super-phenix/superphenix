package organization

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/authorization"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/group"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/user"
	httpModel "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/model"
	groupsvc "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/iam/group"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/project/project"
	httpError "github.com/super-phenix/superphenix/pkg/utils/error"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	pwV1 "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/permission"

	"github.com/google/uuid"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/organization"
	crudProject "github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/project"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/utils"
	"github.com/super-phenix/superphenix/pkg/utils/decoder"

	pw "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg"
	pwPermission "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/permission"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

type UpdateOrganizationBody struct {
	Name                  string  `json:"name" validate:"max=63"`
	AdministrativeContact *string `json:"administrativeContact,omitempty" validate:"omitempty,max=255"`
	BillingContact        *string `json:"billingContact,omitempty" validate:"omitempty,max=255"`
	TechnicalContact      *string `json:"technicalContact,omitempty" validate:"omitempty,max=255"`
}

type CreateOrganizationBody struct {
	Name string `json:"name" validate:"max=63"`
}

type TransferOrganizationBody struct {
	NewOwnerInviteCode uuid.UUID `json:"newOwnerInviteCode"`
}

// Get organization by id
//
//	@Summary		Get an Organization
//	@Description	Get an Organization by id
//	@Tags			v1, organization
//	@Produce		json
//	@Param			orgaId	path		string					true	"Organization ID"
//	@Success		200		{object}	model.APIOrganization	"Organization"
//	@Failure		400
//	@Failure		500
//	@Router			/v1/organization/{orgaId} [get]
//	@Security		Bearer[OrganizationRead]
func (e *Service) Get(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	orgaId := chi.URLParam(r, "orgaId")
	orga, err := organization.FindCompleteOrgById(orgaId)

	if err != nil {
		log.Error().Err(err).Str("orgaId", orgaId).Msg("Fail to get organization")
	}

	// Check if the user can access to IAM
	// If not, remove the user list from the organization
	if !authorization.CheckPermissionForRequest(r, pwV1.OrganizationIAMRead) {
		orga.Users = nil
	} else {
		// If the user have the permission, fetch orga owner information
		owner, err := user.FindById(orga.OwnerId.String())
		if err != nil {
			log.Error().Err(err).Str("orgaId", orgaId).Str("userId", orga.OwnerId.String()).Msg("Fail to get owner")
		}

		orga.Owner = &httpModel.APIUserReduce{
			APIModel: httpModel.APIModel{
				ID: owner.ID,
			},
			Firstname: owner.Firstname,
			Lastname:  owner.Lastname,
			Email:     owner.Email,
		}
	}

	if len(orga.Projects) > 0 {
		userUuid, err := utils.GetUserUuid(r)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get user uuid")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		// Check for Super Admin bypass
		canBypass := authorization.IsSuperAdminUser(userUuid.String())
		if canBypass {
			log.Info().Ctx(r.Context()).
				Str("method", "GetOrganization").
				Str("url", r.URL.String()).
				Str("orgaId", orgaId).
				Str("userId", userUuid.String()).
				Msg("Fetch all project for organization")
		}

		filteredProject := utils.Filter(orga.Projects, func(p *httpModel.APIProject) bool {
			if canBypass {
				return true
			}
			return pwPermission.CanAccess(r.Context(), orgaId, pwV1.ProjectRead, p.ID.String(), userUuid.String())
		})

		orga.Projects = filteredProject
	}

	w.Header().Set("Content-Type", "application/json")
	marshal, err := json.Marshal(orga)
	if err != nil {
		log.Error().Err(err).Msg("Error marshalling response")
	}
	_, _ = w.Write(marshal)
}

// Update an organization
//
//	@Summary		Update an Organization
//	@Description	Update an Organization
//	@Tags			v1, organization
//	@Accept			json
//	@Produce		json
//	@Param			orgaId	path		string					true	"Organization ID"
//	@Param			Body	body		UpdateOrganizationBody	true	"Update Info"
//	@Success		200		{object}	model.Organization		"Organization"
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/v1/organization/{orgaId} [post]
//	@Security		Bearer[OrganizationRead, OrganizationWrite]
func (e *Service) Update(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	orgaId := chi.URLParam(r, "orgaId")

	var body UpdateOrganizationBody
	if err := decoder.HandleHTTPJSON(w, r, &body, e.cfg.PublicHTTP.MaxBodySize); err != nil {
		return
	}

	orga, err := organization.FindById(orgaId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Error().Err(err).Str("orgaId", orgaId).Msg("Not found")
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		} else {
			log.Error().Err(err).Str("orgaId", orgaId).Msg("Failed find organization")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	// Update name
	orga.Name = body.Name
	orga.AdministrativeContact = body.AdministrativeContact
	orga.BillingContact = body.BillingContact
	orga.TechnicalContact = body.TechnicalContact
	orga, err = organization.Save(orga)
	if err != nil {
		log.Error().Err(err).Str("orgaId", orgaId).Any("body", body).Msg("Failed save organization")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	marshal, err := json.Marshal(orga)
	if err != nil {
		log.Error().Err(err).Msg("Error marshalling response")
	}
	_, _ = w.Write(marshal)
}

// Create initialize a new organization, a default project and the associated permissions
//
//	@Summary		Create an Organization
//	@Description	Initialize a new organization, a default project and the associated permissions
//	@Tags			v1, organization
//	@Produce		json
//	@Success		200	{object}	model.Organization	"Created Organization"
//	@Failure		400
//	@Failure		500
//	@Router			/v1/organization [post]
//	@Security		Bearer
func (e *Service) Create(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	orgaId := chi.URLParam(r, "orgaId")

	var body CreateOrganizationBody
	if err := decoder.HandleHTTPJSON(w, r, &body, e.cfg.PublicHTTP.MaxBodySize); err != nil {
		return
	}

	userUuid, err := utils.GetUserUuid(r)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get user uuid")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	reached, err := organization.IsQuotaCreationReached(r.Context(), userUuid)
	if err != nil {
		log.Error().Err(err).Msg("Failed to check quota")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if reached {
		log.Error().Msg("Quota to create organization reached")
		http.Error(w, "Quota to create organization reached", http.StatusBadRequest)
		return
	}

	orga := model.Organization{
		Name:    body.Name,
		OwnerId: userUuid,
	}
	orga, err = organization.Save(orga)
	if err != nil {
		log.Error().Err(err).Str("orgaId", orgaId).Any("body", body).Msg("Failed save organization")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Organization initialization in Permify
	if err := InitOrganization(r.Context(), orga, userUuid); err != nil {
		log.Error().Err(err).Msg("Failed to init organization")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Init default project (DB and Permify)
	if _, err = project.InitializeProject(r.Context(), userUuid, orga.ID, "default"); err != nil {
		log.Error().Err(err).Str("orgaId", orgaId).Any("body", body).Msg("Failed init default project for organization")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	marshal, err := json.Marshal(orga)
	if err != nil {
		log.Error().Err(err).Msg("Error marshalling response")
	}
	_, _ = w.Write(marshal)
}

// InitOrganization initialize an organization in Permify
// Create new tenant, setup permission schema, bundles and default groups
func InitOrganization(ctx context.Context, organization model.Organization, userUuid uuid.UUID) error {
	log := logger.GetLogger(ctx)
	orgaId := organization.ID.String()

	// Organization initialization in Permify
	err := pw.CreateRelationOrganization(ctx, pw.Organization{
		Id:   orgaId,
		Name: organization.Name,
	}, map[string][]string{})
	if err != nil {
		log.Error().Err(err).Any("userUuid", userUuid).Any("organization", organization).Msg("Failed to create organization in Permify")
		return err
	}

	// Initialize default group for organization, and add user as admin
	if err := groupsvc.InitializeDefaultGroups(ctx, organization.ID, userUuid); err != nil {
		log.Error().Err(err).Any("userUuid", userUuid).Any("organization", organization).Msg("Failed init groups for organization")
		return err
	}

	return nil
}

// Delete an organization
//
//	@Summary		Delete an Organization
//	@Description	Delete an Organization by id
//	@Tags			v1, organization
//	@Produce		json
//	@Param			orgaId	path	string	true	"Organization ID"
//	@Success		200
//	@Failure		400
//	@Failure		403
//	@Failure		500
//	@Router			/v1/organization/{orgaId} [delete]
//	@Security		Bearer[OrganizationRead, OrganizationWrite]
func (e *Service) Delete(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	orgaId := chi.URLParam(r, "orgaId")
	userUuid, err := utils.GetUserUuid(r)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get user uuid")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	org, err := organization.FindById(orgaId)
	if err != nil {
		log.Error().Err(err).Any("userId", userUuid).Msg("failed to find organization")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if org.OwnerId != userUuid {
		log.Error().Str("orgaId", org.ID.String()).Str("orgOwner", org.OwnerId.String()).Msgf("Organization owner is not %s", userUuid)
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	count, err := organization.CountOrgaByUser(userUuid)
	if err != nil {
		log.Error().Err(err).Any("userId", userUuid).Msg("failed to count organization")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if count <= 1 {
		log.Error().Str("orgaId", orgaId).Any("userId", userUuid).Msg("can't delete last organization")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	projects, err := crudProject.FindAllByOrgaId(orgaId)
	if err != nil {
		log.Error().Err(err).Any("userId", userUuid).Msg("failed to find projects for the organization")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Removing all projects inside the organization
	for _, p := range projects {
		err := project.RemoveProject(r.Context(), orgaId, p.ID.String())
		if err != nil {
			log.Error().Err(err).Any("userId", userUuid).Str("projectId", p.ID.String()).Msg("failed to remove project for the organization")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	if err := pw.DeleteRelationOrganization(r.Context(), orgaId); err != nil {
		log.Error().Err(err).Any("userId", userUuid).Msg("failed to delete organization from Permify")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if err := organization.Delete(orgaId); err != nil {
		log.Error().Err(err).Any("userId", userUuid).Msg("failed to delete organization")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// Transfer organization ownership to another user
//
//	@Summary		Transfer organization ownership
//	@Description	Transfer organization ownership to another user
//	@Tags			v1, organization
//	@Accept			json
//	@Produce		json
//	@Param			orgaId	path	string						true	"Organization ID"
//	@Param			Body	body	TransferOrganizationBody	true	"Transfer Body"
//	@Success		200
//	@Failure		400
//	@Failure		403
//	@Failure		500
//	@Router			/v1/organization/{orgaId}/transfer [post]
//	@Security		Bearer[OrganizationRead, OrganizationWrite]
func (e *Service) Transfer(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	orgaId := chi.URLParam(r, "orgaId")
	userUuid, err := utils.GetUserUuid(r)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get user uuid")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	org, err := organization.FindById(orgaId)
	if err != nil {
		log.Error().Err(err).Any("orgaId", orgaId).Msg("failed to find organization")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if org.OwnerId != userUuid {
		log.Error().Str("orgaId", org.ID.String()).Str("orgOwner", org.OwnerId.String()).Msgf("Organization owner is not %s", userUuid)
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	count, err := organization.CountOrgaByUser(userUuid)
	if err != nil {
		log.Error().Err(err).Any("userId", userUuid).Msg("failed to count organization")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if count <= 1 {
		log.Error().Str("orgaId", orgaId).Any("userId", userUuid).Msg("can't transfer last organization")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	ownerGroup, err := group.FindOwnerGroup(orgaId)
	if err != nil {
		log.Error().Err(err).Str("orgaId", orgaId).Msg("Failed to find owner group for organization")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	var body TransferOrganizationBody
	if err := decoder.HandleHTTPJSON(w, r, &body, e.cfg.PublicHTTP.MaxBodySize); err != nil {
		return
	}

	if body.NewOwnerInviteCode == uuid.Nil {
		log.Error().Msg("Invite code is empty")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	newOwner, err := user.FindByInviteCode(body.NewOwnerInviteCode)
	if err != nil {
		log.Err(err).Msg("User to invite not found")
		httpError.Http(w, r, http.StatusNotFound).Msg("Failed to invite user")
		return
	}
	if !newOwner.IsActive {
		log.Error().Msg("User to invite is not active - Cannot transfer organization")
		httpError.Http(w, r, http.StatusNotFound).Msg("User to invite not found")
		return
	} else if newOwner.ID == userUuid {
		log.Error().Msg("Cannot transfer organization to the current user")
		httpError.Http(w, r, http.StatusBadRequest).Msg("Cannot transfer organization to yourself")
		return
	}

	reached, err := organization.IsQuotaCreationReached(r.Context(), newOwner.ID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to check quota")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if reached {
		log.Error().Str("orgaId", org.ID.String()).Str("newId", newOwner.ID.String()).Msg("Quota to create organization reached for the new user - Cannot transfer")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	//	Change ownership in db
	org.OwnerId = newOwner.ID
	org, err = organization.Save(org)
	if err != nil {
		log.Error().Err(err).Str("orgaId", orgaId).Any("org", org).Msg("Failed save organization")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Update new owner (add owner rights)
	if err := pw.CreateOrUpdateRelationUserGroup(r.Context(), org.ID.String(), org.OwnerId.String(), []string{ownerGroup.ID.String()}); err != nil {
		log.Error().Err(err).Str("orgaId", orgaId).Msg("Failed to update new owner relation")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	// remove from users_organization
	if err := organization.RemoveUserFromOrg(r.Context(), org.ID.String(), org.OwnerId.String()); err != nil {
		log.Error().Err(err).Str("orgaId", org.ID.String()).Str("userToRemove", org.OwnerId.String()).Msg("Failed to remove user from org")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Update permify old owner (remove rights)
	if err := pw.DeleteRelationUser(r.Context(), orgaId, userUuid.String()); err != nil {
		log.Error().Err(err).Str("orgaId", orgaId).Any("oldOwner", userUuid.String()).Msg("Failed remove old owner permissions")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}
