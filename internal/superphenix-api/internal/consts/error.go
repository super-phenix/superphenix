package consts

import "net/http"

const (
	SpxWrongPathParam = "Wrong path params."

	SpxBodyParseFailureCode = http.StatusBadRequest
	SpxBodyParseFailure     = "Failed to parse given body."

	SpxResourceCreationFailureCode = http.StatusInternalServerError
	SpxResourceCreationFailure     = "Failed to create resource."

	SpxResourceUpdateFailureCode = http.StatusInternalServerError
	SpxResourceUpdateFailure     = "Failed to update resource."

	SpxResourceDeletionFailureCode = http.StatusInternalServerError
	SpxResourceDeletionFailure     = "Failed to delete resource."

	SpxProxyToAZFailureCode = http.StatusInternalServerError
	SpxProxyToAZFailure     = "Failed to contact AZ."

	SpxFindAZsErrorCode = http.StatusInternalServerError
	SpxFindAZsError     = "Failed to find AZs."

	SpxResponseParseFailureCode = http.StatusInternalServerError
	SpxResponseParseFailure     = "Failed to parse response."

	SpxFindAllResourcesErrorCode = http.StatusInternalServerError
	SpxFindAllResourcesError     = "Failed to find resources."

	SpxResourceNotFound      = "Failed to get resource."
	SpxAZNotFound            = "Failed to get az."
	SpxOrgNotFound           = "Failed to get organization."
	SpxProjectNotFound       = "Failed to get project."
	SpxProjectAndOrgMismatch = "Project doesn't belong to the organization provided."
)
