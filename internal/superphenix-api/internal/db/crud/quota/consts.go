package quota

const (
	UserLimitOrganization = "User_Limit_Organization"
	OrgaLimitProject      = "Orga_Limit_Project"
	OrgaLimitIAMGroup     = "Orga_Limit_IAMGroup"
	ProjectLimitProduct   = "Project_Limit_Product"
)

const (
	EntityUser    = "User"
	EntityOrga    = "Orga"
	EntityProject = "Project"
)

var (
	MapQuotaEntityType = map[string]string{
		UserLimitOrganization: EntityUser,
		OrgaLimitProject:      EntityOrga,
		OrgaLimitIAMGroup:     EntityOrga,
		ProjectLimitProduct:   EntityProject,
	}
)
