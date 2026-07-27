package superphenixId

import "regexp"

const (
	nameRegex     = `^[a-z0-9]+(-[a-z0-9]+)*$`
	nameMaxLength = 63
)

const (
	SpxLabelPrefix              = "superphenix.net/"
	SpxLabelOrganizationID      = "superphenix.net/organizationID"
	SpxLabelOrganizationName    = "superphenix.net/organizationName" // used in gitops resources
	SpxLabelProjectID           = "superphenix.net/projectID"
	SpxLabelProjectName         = "superphenix.net/projectName" // used in gitops resources
	SpxLabelResourceLocalID     = "superphenix.net/resourceLocalID"
	SpxLabelResourceEffectiveID = "superphenix.net/resourceEffectiveID"
	SpxLabelResourceName        = "superphenix.net/resourceName" // used in gitops resources
	SpxLabelGitops              = "superphenix.net/gitops"
)

const (
	SpxAnnotationAllowedProjects = "superphenix.net/allowedProjects"
)

var (
	nameEnforcer = regexp.MustCompile(nameRegex)
)
