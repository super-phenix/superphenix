package metadata

import (
	"net/http"

	ctrlutils "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/utils"
)

// StorageClass
//
//	@Summary		Retrieve storage classes
//	@Description	Retrieve all storage classes for a specific AZ
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path	string	true	"Organization ID"
//	@Param			az			path	string	true	"AZ Code"
//	@Param			projectId	path	string	true	"Project ID"
//	@Success		200			{array}	string	"Storage Classes"
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/storage-class [get]
//	@Security		Bearer[OrganizationRead]
func (h *Service) StorageClass(w http.ResponseWriter, r *http.Request) {
	ctrlutils.SimpleRedirect()(w, r)
}

// VMType
//
//	@Summary		Retrieve VM types
//	@Description	Retrieve all available VM types for a specific AZ
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path	string	true	"Organization ID"
//	@Param			az			path	string	true	"AZ Code"
//	@Param			projectId	path	string	true	"Project ID"
//	@Success		200			{array}	string	"VM Types"
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/vm-type [get]
//	@Security		Bearer[OrganizationRead]
func (h *Service) VMType(w http.ResponseWriter, r *http.Request) {
	ctrlutils.SimpleRedirect()(w, r)
}

// VMTypeByName
//
//	@Summary		Retrieve a VM type by name
//	@Description	Retrieve a specific VM type (cluster preference) by name for a specific AZ
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path		string	true	"Organization ID"
//	@Param			az			path		string	true	"AZ Code"
//	@Param			projectId	path		string	true	"Project ID"
//	@Param			name		path		string	true	"VM Cluster Preference Name"
//	@Success		200			{object}	object	"VM Cluster Preference"
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/vm-type/{name} [get]
//	@Security		Bearer[OrganizationRead]
func (h *Service) VMTypeByName(w http.ResponseWriter, r *http.Request) {
	ctrlutils.SimpleRedirect()(w, r)
}

// VMTypeAdvancedOptions
//
//	@Summary		Get the advanced options a VM type would apply
//	@Description	Resolve the advanced device/firmware options defaulted by a preference (no VM), for the create form
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path		string	true	"Organization ID"
//	@Param			az			path		string	true	"AZ Code"
//	@Param			projectId	path		string	true	"Project ID"
//	@Param			name		path		string	true	"VM Cluster Preference Name"
//	@Success		200			{object}	object	"Advanced options"
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/vm-type/{name}/advanced-options [get]
//	@Security		Bearer[OrganizationRead]
func (h *Service) VMTypeAdvancedOptions(w http.ResponseWriter, r *http.Request) {
	ctrlutils.SimpleRedirect()(w, r)
}
