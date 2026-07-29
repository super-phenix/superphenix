package billing

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/consts"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	httpError "github.com/super-phenix/superphenix/pkg/utils/error"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	ch "github.com/super-phenix/superphenix/pkg/chi-helper"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// GetProjectName retrieve the project name by spx ID
//
//	@Summary		Get Project Name
//	@Description	Get Project Name by SPX Project ID
//	@Tags			Admin endpoint, Billing, v1
//	@Produce		json
//	@Param			code	path		string	true	"AZ Code to delete"
//	@Success		200		string		"Project Name"
//	@Failure		400		{string}	string	"Error"
//	@Failure		404		{string}	string	"Error"
//
//	@Router			/billing/project/{projectId} [get]
func (h *Service) GetProjectName(w http.ResponseWriter, r *http.Request) {
	spxProjectId := chi.URLParam(r, "projectId")
	projectId, _ := strings.CutPrefix(spxProjectId, fmt.Sprintf("%s-", spxId.FrameworkPrefix()))

	projectUuid, err := uuid.Parse(projectId)
	if err != nil {
		log.Error().Err(err).Str("projectId", projectId).Msg("Failed to parse projectId")
		httpError.Http(w, r, http.StatusBadRequest).Msg(consts.SpxWrongPathParam)
		return
	}
	project, err := crud.FindUnscoped[model.Project, model.Project](model.Project{
		Model: model.Model{
			ID: projectUuid,
		},
	})
	if err != nil {
		log.Error().Err(err).Str("projectId", projectId).Msg(consts.SpxProjectNotFound)
		httpError.Http(w, r, http.StatusNotFound).Msg(consts.SpxProjectNotFound)
		return
	}
	ch.String(w, http.StatusOK, project.Name)
}

// GetOrgaName retrieve the organization name by SPX Organization ID
//
//	@Summary		Get Organization Name
//	@Description	Get Organization Name by SPX Organization ID
//	@Tags			Admin endpoint, Billing, v1
//	@Produce		json
//	@Param			orgaId	path		string	true	"Organization ID"
//	@Success		200		string		"Organization Name"
//	@Failure		400		{string}	string	"Error"
//	@Failure		500		{string}	string	"Error"
//
//	@Router			/billing/organization/{orgaId} [get]
func (h *Service) GetOrgaName(w http.ResponseWriter, r *http.Request) {
	spxOrgaId := chi.URLParam(r, "orgaId")
	orgaId, _ := strings.CutPrefix(spxOrgaId, fmt.Sprintf("%s-", spxId.FrameworkPrefix()))

	orgaUuid, err := uuid.Parse(orgaId)
	if err != nil {
		log.Error().Err(err).Str("orgaId", orgaId).Msg("Failed to parse Orga Id")
		httpError.Http(w, r, http.StatusBadRequest).Msg(consts.SpxWrongPathParam)
		return
	}
	orga, err := crud.FindUnscoped[model.Organization, model.Organization](model.Organization{
		Model: model.Model{
			ID: orgaUuid,
		},
	})
	if err != nil {
		log.Error().Err(err).Str("orgaId", orgaId).Msg(consts.SpxOrgNotFound)
		httpError.Http(w, r, http.StatusNotFound).Msg(consts.SpxOrgNotFound)
		return
	}
	ch.String(w, http.StatusOK, orga.Name)
}
