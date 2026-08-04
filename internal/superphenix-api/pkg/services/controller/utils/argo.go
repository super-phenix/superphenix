package ctrlutils

import (
	"errors"
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/argo"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/consts"
	httpError "github.com/super-phenix/superphenix/pkg/utils/error"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

// HandleArgoError is the in-process counterpart of HandleControllerError: the
// Argo calls no longer go over HTTP, so there is no response to read a status
// from — the Kubernetes error class is the status.
func HandleArgoError(w http.ResponseWriter, r *http.Request, err error, failureCode int, failureMessage string) {
	log := logger.GetLogger(r.Context())
	log.Error().Err(err).Msg(failureMessage)

	switch {
	case k8serrors.IsNotFound(err):
		httpError.Http(w, r, http.StatusNotFound).Msg(consts.SpxResourceNotFound)
	case k8serrors.IsAlreadyExists(err):
		httpError.Http(w, r, http.StatusConflict).Msg(failureMessage)
	case errors.Is(err, argo.ErrGitopsManaged):
		httpError.Http(w, r, failureCode).Str("reason", argo.ErrGitopsManaged.Error()).Msg(failureMessage)
	default:
		httpError.Http(w, r, failureCode).Msg(failureMessage)
	}
}
