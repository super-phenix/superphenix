package manager

import (
	"encoding/json"
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/authorization"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/consts"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm/clause"
)

// checkSuperAdminList rejects users that are not super admins.
func checkSuperAdminList(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userId := r.Context().Value(consts.ContextUserId)
		if userId == nil {
			log.Error().Msg("No user id found in context")
			userId = ""
		}

		if authorization.IsSuperAdminUser(userId.(string)) {
			log.Info().Ctx(r.Context()).
				Str("method", "checkSuperAdminList").
				Str("url", r.URL.String()).
				Str("userId", userId.(string)).
				Msg("Check Project manager list")
			next.ServeHTTP(w, r)
			return
		} else {
			log.Error().Str("userId", userId.(string)).Msg("User not allowed to discover this endpoint")
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		}
	})
}

// listUsers
//
//	@Summary		Retrieve all users
//	@Description	Retrieve all users with their associations. Requires super-admin privileges.
//	@Tags			Project Manager
//	@Produce		json
//	@Success		200	{array}	model.User	"Users"
//	@Failure		404
//	@Failure		500
//	@Router			/v1/project-manager/users [get]
func (h *Service) ListUsers(w http.ResponseWriter, r *http.Request) {
	all, err := crud.FindAll[model.User, model.User](model.User{}, clause.Associations)
	handleResult(w, r, "listUsers", all, err)
}

// listOrgas
//
//	@Summary		Retrieve all organizations
//	@Description	Retrieve all organizations with their associations. Requires super-admin privileges.
//	@Tags			Project Manager
//	@Produce		json
//	@Success		200	{array}	model.Organization	"Organizations"
//	@Failure		404
//	@Failure		500
//	@Router			/v1/project-manager/organizations [get]
func (h *Service) ListOrganizations(w http.ResponseWriter, r *http.Request) {
	all, err := crud.FindAll[model.Organization, model.Organization](model.Organization{}, clause.Associations)
	handleResult(w, r, "listOrgas", all, err)
}

// listProjects
//
//	@Summary		Retrieve all projects
//	@Description	Retrieve all projects with their associations. Requires super-admin privileges.
//	@Tags			Project Manager
//	@Produce		json
//	@Success		200	{array}	model.Project	"Projects"
//	@Failure		404
//	@Failure		500
//	@Router			/v1/project-manager/projects [get]
func (h *Service) ListProjects(w http.ResponseWriter, r *http.Request) {
	all, err := crud.FindAll[model.Project, model.Project](model.Project{}, clause.Associations)
	handleResult(w, r, "listProjects", all, err)

}

// listProducts
//
//	@Summary		Retrieve all products
//	@Description	Retrieve all products with their associations. Requires super-admin privileges.
//	@Tags			Project Manager
//	@Produce		json
//	@Success		200	{array}	model.Product	"Products"
//	@Failure		404
//	@Failure		500
//	@Router			/v1/project-manager/products [get]
func (h *Service) ListProducts(w http.ResponseWriter, r *http.Request) {
	all, err := crud.FindAll[model.Product, model.Product](model.Product{}, clause.Associations)
	handleResult(w, r, "listProducts", all, err)
}

func handleResult(w http.ResponseWriter, r *http.Request, method string, results any, err error) {
	if err != nil {
		log.Error().Ctx(r.Context()).Err(err).Str("method", method).Msg("Failed to retrieve datas")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	marshal, err2 := json.Marshal(results)
	if err2 != nil {
		log.Error().Ctx(r.Context()).Err(err2).Str("method", method).Msg("Error marshalling response")
	}
	_, _ = w.Write(marshal)
}
