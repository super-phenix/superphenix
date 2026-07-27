package ctrlutils

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/az"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/consts"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/organization"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/product"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/project"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	httpModel "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/proxy"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	httpError "github.com/super-phenix/superphenix/pkg/utils/error"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SimpleRedirect request are directly send to the match az
func SimpleRedirect() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.GetLogger(r.Context())
		azCode := chi.URLParam(r, "az")
		orgaId := chi.URLParam(r, "orgaId")
		azCfg, err := az.GetByCode(azCode, orgaId)
		if err != nil {
			log.Error().Err(err).Str("az", azCode).Msg("Failed to get az")
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		// Transfer to the right AZ
		// Remove the pattern and the code from the call
		reverseProxy, err := proxy.ReverseProxy(azCfg.ControllerUrl, config.Global.Controller.ApiPrefix+"/"+azCode)
		if err != nil {
			log.Error().Err(err).Str("az", azCode).Msg("Failed to proxy request")
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		reverseProxy.ServeHTTP(w, r)
	}
}

// ConcatResponses regroups all http.Response by AZ code into a flattened 1D slice.
func ConcatResponses(ctx context.Context, responses map[string]*http.Response) map[string][]interface{} {
	log := logger.GetLogger(ctx)
	results := make(map[string][]interface{})
	for azCode, resp := range responses {
		if resp != nil && resp.StatusCode == 200 {
			res := ReadResponse(resp)

			// If no response just ignore it
			if res != nil {
				// If slice we flatten it to keep our result slice 1D
				if IsSlice(res) {
					results[azCode] = res.([]interface{})
				} else {
					results[azCode] = []interface{}{res}
				}
			}
		} else {
			logLine := log.Error().Str("az", azCode)
			if resp != nil {
				logLine = logLine.Str("status", resp.Status).Int("statusCode", resp.StatusCode)
			}
			logLine.Msg("Failed to get response from az")
		}
	}

	return results
}

// ReadResponse read a body from http.Response and return an interface{}
func ReadResponse(resp *http.Response) interface{} {
	log := logger.GetLogger(resp.Request.Context())
	// read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error().Err(err).Msg("Error reading response body")
	}
	// close it right after we don't need it
	if err := resp.Body.Close(); err != nil {
		log.Error().Err(err).Msg("Error closing response body")
	}
	var res interface{}
	if err := json.Unmarshal(body, &res); err != nil {
		log.Error().Err(err).Str("request-url", resp.Request.URL.String()).Msg("Failed to unmarshal")
	}

	return res
}

// CheckListPathParams validate path param of a request
// Check if organization and project exists
// And also if the project belongs to the organization
// If success, return each db object and status code 0
// Else return empty object and http.Status code (404 or 400)
func CheckListPathParams(r *http.Request) (model.Organization, httpModel.APIProject, int, string) {
	log := logger.GetLogger(r.Context())
	orgaId := chi.URLParam(r, "orgaId")
	orga, err := organization.FindById(orgaId)
	if err != nil {
		log.Error().Err(err).Str("orgaId", orgaId).Msg(consts.SpxOrgNotFound)
		return model.Organization{}, httpModel.APIProject{}, http.StatusNotFound, consts.SpxOrgNotFound
	}

	projectId := chi.URLParam(r, "projectId")
	projectEntity, err := project.FindById(projectId)
	if err != nil {
		log.Error().Err(err).Str("projectId", projectId).Msg(consts.SpxProjectNotFound)
		return model.Organization{}, httpModel.APIProject{}, http.StatusNotFound, consts.SpxProjectNotFound
	}

	if projectEntity.OrgaId != orga.ID {
		log.Error().Err(err).Str("Project Orga", projectEntity.OrgaId.String()).Str("Orga Id provided", orga.ID.String()).Msg(consts.SpxProjectAndOrgMismatch)
		return model.Organization{}, httpModel.APIProject{}, http.StatusBadRequest, consts.SpxProjectAndOrgMismatch
	}

	return orga, projectEntity, 0, ""
}

// CheckPathParams validate path param of a request
// Check if az, organization and project exists
// And also if the project belongs to the organization
// If success, return each db object and status code 0
// Else return empty object and http.Status code (404 or 400)
func CheckPathParams(r *http.Request) (config.AZConfig, model.Organization, httpModel.APIProject, int, string) {
	log := logger.GetLogger(r.Context())

	// Fetch all required information
	azCode := chi.URLParam(r, "az")

	orga, projectEntity, errCode, errMessage := CheckListPathParams(r)

	if errCode != 0 {
		return config.AZConfig{}, model.Organization{}, httpModel.APIProject{}, errCode, errMessage
	}

	azCfg, err := az.GetByCode(azCode, orga.ID.String())
	if err != nil {
		log.Error().Err(err).Str("az", azCode).Msg(consts.SpxAZNotFound)
		return config.AZConfig{}, model.Organization{}, httpModel.APIProject{}, http.StatusNotFound, consts.SpxAZNotFound
	}

	return azCfg, orga, projectEntity, errCode, errMessage
}

// CheckProductBelongsToProject is the API-side tenancy pre-flight. It only
// rejects requests that target a DB row owned by a different project or AZ —
// the original cross-tenant attack. When no DB row exists at all (shared
// subnets reached from the consumer project, gitops/argo-managed objects,
// or out-of-band desync) it defers the decision to the AZ controller, which
// runs CheckProjectLabel (and, for subnets, isAllowedSharedSubnet).
func CheckProductBelongsToProject(w http.ResponseWriter, r *http.Request, projectUuid uuid.UUID, azCode string) bool {
	log := logger.GetLogger(r.Context())
	productEId := chi.URLParam(r, "effectiveId")
	dbProduct, err := product.FindByEId(productEId)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return true
	}
	if err != nil {
		log.Error().Err(err).Str("effectiveId", productEId).Msg("Failed to look up product by Effective ID")
		httpError.Http(w, r, http.StatusInternalServerError).Str("eid", productEId).Msg(consts.SpxResourceNotFound)
		return false
	}
	if dbProduct.ProjectId != projectUuid {
		log.Warn().Str("effectiveId", productEId).Str("projectId", projectUuid.String()).Str("az", azCode).Msg("Cross-tenant resource access denied")
		httpError.Http(w, r, http.StatusNotFound).Str("eid", productEId).Msg(consts.SpxResourceNotFound)
		return false
	}
	return true
}

func CleanDb(ctx context.Context, id uuid.UUID) {
	log := logger.GetLogger(ctx)
	// Clean db if the creation failed
	err := product.DeleteById(id)
	if err != nil {
		log.Error().Err(err).Msg("Failed to delete product")
	}
}

func HandleControllerError(w http.ResponseWriter, r *http.Request, resp *http.Response, failureCode int, failureMessage string) {
	log := logger.GetLogger(r.Context())
	errorBody := httpError.RetrieveHttpError(resp)
	log.Error().Any("responseBody", errorBody).Str("status", resp.Status).Int("statusCode", resp.StatusCode).Msg(failureMessage)
	if errorBody != nil {
		httpError.Http(w, r, failureCode).Str("reason", errorBody.Message).Msg(failureMessage)
	} else {
		httpError.Http(w, r, failureCode).Msg(failureMessage)
	}
}

// IsSlice check is an interface{} is a slice
func IsSlice(v interface{}) bool {
	return reflect.TypeOf(v).Kind() == reflect.Slice || reflect.TypeOf(v).Kind() == reflect.Array
}
