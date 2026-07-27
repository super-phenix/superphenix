package superphenixId

import (
	"errors"
	"fmt"
)

var (
	OrgIDEmpty                      = errors.New("organization ID cannot be empty")
	InvalidOrgID                    = errors.New("organization ID is not a valid UUID")
	ProjectIDEmpty                  = errors.New("project ID cannot be empty if there's a resource local ID")
	InvalidProjectID                = errors.New("project ID is not a valid UUID")
	InvalidResourceLocalID          = errors.New(fmt.Sprintf("resource local ID must match regex %s", nameRegex))
	InvalidProjectOrResourceLocalID = errors.New("project ID and resource local ID cannot be empty")
)
