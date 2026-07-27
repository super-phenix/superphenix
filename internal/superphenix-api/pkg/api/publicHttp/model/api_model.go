package model

import (
	"time"

	"github.com/google/uuid"
)

var APIUserEmpty = []byte("{}")

type APIModel struct {
	ID uuid.UUID `json:"id"`
}
type APIUser struct {
	APIModel
	Firstname  *string `json:"firstname,omitempty"`
	Lastname   *string `json:"lastname,omitempty"`
	Email      string  `json:"email,omitempty"`
	Provider   string  `json:"provider,omitempty"`
	ProviderId string  `json:"providerId,omitempty"`

	InviteCode uuid.UUID `json:"inviteCode,omitempty"`

	PersonalOrg []APIOrganizationReduce  `json:"personalOrg"`
	GuestOrg    []*APIOrganizationReduce `json:"guestOrg,omitempty"`
}

type APIUserReduce struct {
	APIModel
	Firstname  *string   `json:"firstname,omitempty"`
	Lastname   *string   `json:"lastname,omitempty"`
	Email      string    `json:"email,omitempty"`
	InviteCode uuid.UUID `json:"inviteCode,omitempty"`
}

type APIUserGroup struct {
	User   *APIUserReduce `json:"user,omitempty"`
	Groups []uuid.UUID    `json:"groups,omitempty"`
}

type APIOrganization struct {
	APIModel
	Name string `json:"name,omitempty"`

	OwnerId  uuid.UUID       `json:"ownerId,omitempty"`
	Owner    *APIUserReduce  `json:"owner,omitempty"`
	Users    []*APIUserGroup `json:"users,omitempty"`
	Projects []*APIProject   `json:"projects,omitempty"`

	AdministrativeContact *string `json:"administrativeContact,omitempty"`
	BillingContact        *string `json:"billingContact,omitempty"`
	TechnicalContact      *string `json:"technicalContact,omitempty"`
}

type APIOrganizationReduce struct {
	APIModel
	OwnerId uuid.UUID `json:"ownerId,omitempty"`
	Name    string    `json:"name,omitempty"`
}

type APIProject struct {
	APIModel
	Name string `json:"name,omitempty"`

	OrgaId uuid.UUID `json:"orgaId,omitempty"`
}

type APIGroup struct {
	APIModel
	Name string `json:"name,omitempty"`
	// If true, then ignore ProjectIds
	AllProjects bool `json:"allProjects,omitempty"`
	// List all projects concerned by the Group
	ProjectIds     []string `json:"projectIds,omitempty"`
	PermissionSets []string `json:"permissionSets,omitempty"`
}

type APIAz struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	LogoUrl string `json:"logoUrl"`
}

type APIApiToken struct {
	APIModel
	Name      string    `json:"name"`
	Prefix    string    `json:"prefix"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
}
