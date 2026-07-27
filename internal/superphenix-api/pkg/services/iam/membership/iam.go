package membership

import (
	"encoding/json"
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/group"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/organization"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/user"
	httpModel "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/model"
	"github.com/super-phenix/superphenix/pkg/utils/decoder"
	httpError "github.com/super-phenix/superphenix/pkg/utils/error"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	ch "github.com/super-phenix/superphenix/pkg/chi-helper"

	pw "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg"
	v1 "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type InviteIntoOrganizationBody struct {
	UserInviteCode uuid.UUID `json:"userInviteCode"`
	GroupIds       []string  `json:"groupIds"`
}

// InviteIntoOrganization
//
//	@Summary		Invite User in Org
//	@Description	Invite a user into organization with given roles (or update his roles)
//	@Tags			v1, iam
//	@Accept			json
//	@Produce		json
//	@Param			orgaId	path		string						true	"Organization ID"
//	@Param			Body	body		InviteIntoOrganizationBody	true	"Invite Info"
//	@Success		200		{object}	model.APIUserGroup			"User Group Relation"
//	@Failure		400
//	@Failure		403
//	@Failure		404
//	@Failure		500
//	@Router			/v1/organization/{orgaId}/iam/invite [post]
//	@Security		Bearer[OrganizationRead, OrganizationIAMWrite]
func (h *Service) InviteIntoOrganization(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	orgaId := chi.URLParam(r, "orgaId")

	var body InviteIntoOrganizationBody
	if err := decoder.HandleHTTPJSON(w, r, &body, h.cfg.PublicHTTP.MaxBodySize); err != nil {
		return
	}

	if body.UserInviteCode == uuid.Nil {
		log.Error().Msg("User invite code is null UUID")
		httpError.Http(w, r, http.StatusBadRequest).Msg("Invalid invite code")
		return
	}

	userToInvite, err := user.FindByInviteCode(body.UserInviteCode)
	if err != nil {
		log.Err(err).Msg("User to invite not found")
		httpError.Http(w, r, http.StatusNotFound).Msg("Failed to invite user")
		return
	}

	// If method failed or user is owner
	isOwner, err := organization.IsOwner(orgaId, userToInvite.ID.String())
	if isOwner {
		log.Error().Str("orgaId", orgaId).Str("userInviteCode", body.UserInviteCode.String()).Str("userId", userToInvite.ID.String()).Msg("Cannot invite organization owner")
		httpError.Http(w, r, http.StatusUnauthorized).Msg("Cannot invite organization owner")
		return
	}
	if err != nil {
		log.Err(err).Str("orgaId", orgaId).Str("userInviteCode", body.UserInviteCode.String()).Str("userId", userToInvite.ID.String()).Msg("Failed to check organization owner")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to invite user")
		return
	}

	if len(body.GroupIds) <= 0 {
		log.Err(err).Str("orgaId", orgaId).Str("userInviteCode", body.UserInviteCode.String()).Str("userId", userToInvite.ID.String()).Msg("No groups provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("No groups provided")
		return
	}

	// Filter unexisting group
	groups, err := group.FindAllByIdsAndOrgaId(body.GroupIds, orgaId)
	if err != nil {
		log.Error().Err(err).Str("orgaId", orgaId).Str("userInviteCode", body.UserInviteCode.String()).Str("userId", userToInvite.ID.String()).Strs("groups", body.GroupIds).Msg("Failed to check group ids")
		httpError.Http(w, r, http.StatusBadRequest).Msg(http.StatusText(http.StatusBadRequest))
		return
	}

	// Check if Owner group is in the list
	// And reduce to an ID group list
	var listGroupUuid []uuid.UUID
	var listGroupId []string
	for _, g := range groups {
		if g.Name == v1.DefaultGroupOwnerName {
			log.Error().Err(err).Str("orgaId", orgaId).Str("userInviteCode", body.UserInviteCode.String()).Str("userId", userToInvite.ID.String()).Strs("groups", body.GroupIds).Msg("Cannot add user as owner")
		} else {
			listGroupUuid = append(listGroupUuid, g.ID)
			listGroupId = append(listGroupId, g.ID.String())
		}
	}

	if _, err := organization.InviteUserToOrg(r.Context(), orgaId, userToInvite.ID.String(), listGroupUuid); err != nil {
		log.Error().Err(err).Str("orgaId", orgaId).Str("userInviteCode", body.UserInviteCode.String()).Str("userId", userToInvite.ID.String()).Strs("groups", listGroupId).Msg("Failed to invite to org")
		httpError.Http(w, r, http.StatusInternalServerError).Msg(http.StatusText(http.StatusInternalServerError))
		return
	}

	err = pw.CreateOrUpdateRelationUserGroup(r.Context(), orgaId, userToInvite.ID.String(), listGroupId)
	if err != nil {
		log.Error().Err(err).Str("orgaId", orgaId).Str("userInviteCode", body.UserInviteCode.String()).Str("userId", userToInvite.ID.String()).Strs("groups", listGroupId).Msg("Failed to update user permission")
		_ = organization.RemoveUserFromOrg(r.Context(), orgaId, userToInvite.ID.String())

		httpError.Http(w, r, http.StatusInternalServerError).Msg(http.StatusText(http.StatusInternalServerError))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	marshal, err := json.Marshal(httpModel.APIUserGroup{
		User: &httpModel.APIUserReduce{
			APIModel: httpModel.APIModel{ID: userToInvite.ID},
		},
		Groups: listGroupUuid,
	})
	if err != nil {
		log.Error().Err(err).Msg("Error marshalling response")
	}
	_, _ = w.Write(marshal)

}

// RemoveFromOrganization
//
//	@Summary		Remove User from Org
//	@Description	Remove a user from an organization
//	@Tags			v1, iam
//	@Produce		json
//	@Param			orgaId	path	string	true	"Organization ID"
//	@Param			userId	query	string	true	"User to Remove"
//	@Success		200
//	@Failure		400
//	@Failure		403
//	@Failure		500
//	@Router			/v1/organization/{orgaId}/iam/invite [delete]
//	@Security		Bearer[OrganizationRead, OrganizationIAMWrite]
func (h *Service) RemoveFromOrganization(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	orgaId := chi.URLParam(r, "orgaId")
	userId, ok := ch.GetQuery(r, "userId")
	if !ok {
		log.Error().Msg("No id found")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// If method failed or user is owner
	if isOwner, err := organization.IsOwner(orgaId, userId); err != nil || isOwner {
		log.Error().Err(err).Str("orgaId", orgaId).Str("userToRemove", userId).Msg("Cannot remove organization owner")
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	// Clean previous access in permify
	if err := pw.DeleteRelationUser(r.Context(), orgaId, userId); err != nil {
		log.Error().Err(err).Str("orgaId", orgaId).Str("userToRemove", userId).Msg("Failed to remove access")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if err := organization.RemoveUserFromOrg(r.Context(), orgaId, userId); err != nil {
		log.Error().Err(err).Str("orgaId", orgaId).Str("userToRemove", userId).Msg("Failed to remove user from org")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
