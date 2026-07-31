package utils

import (
	"context"
	"fmt"
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	httpError "github.com/super-phenix/superphenix/pkg/utils/error"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	"github.com/go-chi/chi/v5"
)

const (
	UserContext = "UserId"
	UserHeader  = "X-User-Id"
)

func AddUserToContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userId := r.Header.Get(UserHeader)
		r = r.WithContext(context.WithValue(r.Context(), UserContext, userId))
		next.ServeHTTP(w, r)
	})
}

func GetRequestParams(r *http.Request) (string, string, string) {
	orgId := chi.URLParam(r, "orgId")
	projectId := chi.URLParam(r, "projectId")
	effectiveId := chi.URLParam(r, "effectiveId")
	return orgId, projectId, effectiveId
}

func GetRequestNamespace(r *http.Request) string {
	projectId := chi.URLParam(r, "projectId")
	return GetNamespace(projectId)
}

func GetNamespace(projectId string) string {
	return fmt.Sprintf(`%s-%s`, config.Global.SpxPrefix, projectId)
}

func RetrieveNamespaceAndEid(w http.ResponseWriter, r *http.Request) (string, string, error) {
	log := logger.GetLogger(r.Context())
	namespace := GetRequestNamespace(r)
	effectiveId := chi.URLParam(r, "effectiveId")
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return "", "", fmt.Errorf("no Resource Effective Id provided")
	}

	return namespace, effectiveId, nil
}
