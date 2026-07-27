package k8s

import (
	"encoding/json"
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/k8s"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/k8s/ssh"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/api/utils"
	"github.com/super-phenix/superphenix/pkg/utils/decoder"
	httpError "github.com/super-phenix/superphenix/pkg/utils/error"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	spxIdMiddleware "github.com/super-phenix/superphenix/pkg/superphenix-id/middleware"

	ch "github.com/super-phenix/superphenix/pkg/chi-helper"

	"github.com/go-chi/chi/v5"
	"k8s.io/apimachinery/pkg/api/errors"
)

const baseSshEndpoint = "/ssh"

func SSHEndpoint(router chi.Router) {
	router.Route(baseSshEndpoint, func(r chi.Router) {
		r.Get("/", listSSHs)
		r.Post("/", createSSH)

		r.With(spxIdMiddleware.AddEffectiveIdToContext()).Get("/localId/{localId}", getSSHByLocalId)
		r.Route("/{effectiveId}", func(r chi.Router) {
			r.Get("/", getSSHByEffectiveId)

			r.Delete("/", deleteSSH)
		})
	})
}

// listSSHs
//
//	@Summary		Retrieve all SSHs
//	@Description	Retrieve all SSHs for a project
//	@Tags			v1, SSH
//	@Produce		json
//	@Param			orgId		path	string		true	"Organization ID"
//	@Param			projectId	path	string		true	"Project ID"
//	@Success		200			{array}	view.SSH	"SSHs"
//	@Failure		500
//	@Router			/{orgId}/{projectId}/ssh [get]
func listSSHs(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespaceParam := utils.GetRequestNamespace(r)

	sshList, err := ssh.ListSSHKey(r.Context(), namespaceParam)
	if err != nil {
		log.Err(err).Msg("failed to retrieve resource")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to retrieve resource")
		return
	}

	var sshs []view.SSH
	for _, sshItem := range sshList {
		sshR := view.SSHViewToResource(sshItem)
		sshs = append(sshs, sshR)
	}

	b, _ := json.Marshal(sshs)
	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// getSSHByLocalId
//
//	@Summary		Get SSH by local ID
//	@Description	Get SSH by local ID
//	@Tags			v1, SSH
//	@Produce		json
//	@Param			orgId		path		string		true	"Organization ID"
//	@Param			projectId	path		string		true	"Project ID"
//	@Param			localId		path		string		true	"SSH Local ID"
//	@Success		200			{object}	view.SSH	"SSH"
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/ssh/localId/{localId} [get]
func getSSHByLocalId(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	effectiveId := r.Context().Value(spxId.EffectiveIdContext())
	if effectiveId == nil {
		log.Error().Msg("failed to retrieve Resource Effective Id")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to retrieve resource effective Id")
		return
	}

	getSSH(w, r, effectiveId.(string))
}

// getSSHByEffectiveId
//
//	@Summary		Get SSH by effective ID
//	@Description	Get SSH by effective ID
//	@Tags			v1, SSH
//	@Produce		json
//	@Param			orgId		path		string		true	"Organization ID"
//	@Param			projectId	path		string		true	"Project ID"
//	@Param			effectiveId	path		string		true	"SSH Effective ID"
//	@Success		200			{object}	view.SSH	"SSH"
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/ssh/{effectiveId} [get]
func getSSHByEffectiveId(w http.ResponseWriter, r *http.Request) {
	effectiveId := chi.URLParam(r, "effectiveId")

	getSSH(w, r, effectiveId)
}

func getSSH(w http.ResponseWriter, r *http.Request, effectiveId string) {
	log := logger.GetLogger(r.Context())
	namespaceParam := utils.GetRequestNamespace(r)
	if namespaceParam == "" {
		log.Error().Msg("no namespace provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no project Id provided")
		return
	}

	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	sshView, err := ssh.GetSSHKey(r.Context(), namespaceParam, effectiveId)
	if errors.IsNotFound(err) {
		log.Err(err).Msg("Resource not found")
		httpError.Http(w, r, http.StatusNotFound).Msg("Resource not found")
		return
	}
	if err != nil {
		log.Err(err).Msg("failed to retrieve resource")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to retrieve resource")
		return
	}

	sshResource := view.SSHViewToResource(sshView)

	b, _ := json.Marshal(sshResource)
	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// createSSH
//
//	@Summary		Create an SSH
//	@Description	Create an SSH
//	@Tags			v1, SSH
//	@Accept			json
//	@Produce		plain
//	@Param			orgId		path	string				true	"Organization ID"
//	@Param			projectId	path	string				true	"Project ID"
//	@Param			Body		body	ssh.CreateSSHInfo	true	"SSH info"
//	@Success		200
//	@Failure		400
//	@Failure		500
//	@Router			/{orgId}/{projectId}/ssh [post]
func createSSH(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	orgId := chi.URLParam(r, "orgId")
	projectId := chi.URLParam(r, "projectId")

	err := k8s.CreateNamespaceIfNotExists(r.Context(), orgId, projectId)
	if err != nil {
		log.Error().Msg("Failed to create namespace")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to create namespace")
		return
	}

	var body ssh.CreateSSHInfo
	err = decoder.HandleHTTPJSON(w, r, &body, 5)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decode body")
		httpError.Http(w, r, http.StatusBadRequest).Msg("Failed to decode body")
		return
	}

	err = body.CreateSSHKey(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to create ssh key")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to create ssh key")
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

// deleteSSH
//
//	@Summary		Delete an SSH
//	@Description	Delete an SSH by Effective ID
//	@Tags			v1, SSH
//	@Produce		plain
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"SSH Effective ID"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/ssh [delete]
func deleteSSH(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespaceParam := utils.GetRequestNamespace(r)
	effectiveId := chi.URLParam(r, "effectiveId")
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	if err := ssh.DeleteSSHKey(r.Context(), namespaceParam, effectiveId); err != nil {
		if errors.IsNotFound(err) {
			log.Err(err).Msg("Failed to delete resource - Not found")
			httpError.Http(w, r, http.StatusNotFound).Msg(http.StatusText(http.StatusNotFound))
		} else {
			log.Err(err).Msg("Failed to delete resource")
			httpError.Http(w, r, http.StatusInternalServerError).Msg(http.StatusText(http.StatusInternalServerError))
		}
	} else {
		w.WriteHeader(http.StatusOK)
	}
}
