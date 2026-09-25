package v1

import (
	_ "embed"

	"github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/permissionSet"
)

//go:embed schema/schema_default.perm
var DefaultSchema string

const (
	DefaultGroupOwnerName     = "Owner"
	DefaultGroupAdminName     = "Admin"
	DefaultGroupBillingName   = "Billing"
	DefaultGroupDeveloperName = "Developer"
)

// PredefinedCatalogVersion must be bumped on every change to PredefinedGroups or to
// schema/schema_default.perm. Organizations below this version are re-synchronized.
const PredefinedCatalogVersion = 2

// Keys linking a persisted group to its catalog entry. Never user-visible, never change.
const (
	PredefinedGroupOwner     = "owner"
	PredefinedGroupAdmin     = "admin"
	PredefinedGroupBilling   = "billing"
	PredefinedGroupDeveloper = "developer"
)

// PredefinedGroup is one entry of the catalog: a group owned by the platform, locked against user
// edits and kept up to date in every organization.
type PredefinedGroup struct {
	// Key is persisted on the group row and never changes.
	Key string
	// Name is the display label and can be renamed, rows still match on Key.
	Name string
	// PermissionSets holds the organization-level and project-level sets merged together.
	PermissionSets []string
}

// PredefinedGroups is the catalog every organization is reconciled against. To add a permission:
// declare it in schema/schema_default.perm, add the const in permissionSet/permissionSet.go, add
// it to the entry below, then bump PredefinedCatalogVersion.
var PredefinedGroups = []PredefinedGroup{
	{
		Key:            PredefinedGroupOwner,
		Name:           DefaultGroupOwnerName,
		PermissionSets: concat(DefaultOrganizationOwner, DefaultProjectOwner),
	},
	{
		Key:            PredefinedGroupAdmin,
		Name:           DefaultGroupAdminName,
		PermissionSets: concat(DefaultOrganizationAdmin, DefaultProjectAdmin),
	},
	{
		Key:            PredefinedGroupBilling,
		Name:           DefaultGroupBillingName,
		PermissionSets: concat(DefaultOrganizationBilling, DefaultProjectBilling),
	},
	{
		Key:            PredefinedGroupDeveloper,
		Name:           DefaultGroupDeveloperName,
		PermissionSets: concat(DefaultOrganizationDeveloper, DefaultProjectDeveloper),
	},
}

// Sets returns a copy of the group's permission sets, so a caller appending to it cannot corrupt
// the catalog for the next one.
func (g PredefinedGroup) Sets() []string {
	sets := make([]string, len(g.PermissionSets))
	copy(sets, g.PermissionSets)
	return sets
}

// FindPredefinedGroup returns the catalog entry for key.
func FindPredefinedGroup(key string) (PredefinedGroup, bool) {
	for _, group := range PredefinedGroups {
		if group.Key == key {
			return group, true
		}
	}
	return PredefinedGroup{}, false
}

// IsPredefinedGroupName reports whether name collides with a catalog display name.
func IsPredefinedGroupName(name string) bool {
	for _, group := range PredefinedGroups {
		if group.Name == name {
			return true
		}
	}
	return false
}

// concat merges the organization-level and project-level sets into a fresh slice, aliasing neither.
func concat(orga, project []string) []string {
	merged := make([]string, 0, len(orga)+len(project))
	merged = append(merged, orga...)
	merged = append(merged, project...)
	return merged
}

var (
	DefaultOrganizationOwner = []string{permissionSet.SpxOwner}
	DefaultProjectOwner      = []string{}

	DefaultOrganizationAdmin = []string{
		permissionSet.SpxMember,
		permissionSet.IAMFullAccess,
		permissionSet.SettingsEdition,
		permissionSet.BillingFullAccess,
		permissionSet.AuditLogFullAccess,
		permissionSet.ProjectManagement,
	}
	DefaultProjectAdmin = []string{
		permissionSet.ProjectInstanceFullAccess,
		permissionSet.ProjectDiskFullAccess,
		permissionSet.ProjectSnapshotFullAccess,
		permissionSet.ProjectVPCFullAccess,
		permissionSet.ProjectSubnetFullAccess,
		permissionSet.ProjectEipFullAccess,
		permissionSet.ProjectLoadBalancerFullAccess,
		permissionSet.ProjectSecurityGroupFullAccess,
		permissionSet.ProjectSSHFullAccess,
		permissionSet.ProjectKaaSFullAccess,
		permissionSet.ProjectBaaSFullAccess,
		permissionSet.ProjectBucketFullAccess,
		permissionSet.ProjectArgoCdAccess,
	}

	DefaultOrganizationBilling = []string{
		permissionSet.SpxMember,
		permissionSet.BillingFullAccess,
	}
	DefaultProjectBilling = []string{}

	DefaultOrganizationDeveloper = []string{
		permissionSet.SpxMember,
		permissionSet.ProjectManagement,
	}
	DefaultProjectDeveloper = []string{
		permissionSet.ProjectInstanceFullAccess,
		permissionSet.ProjectDiskFullAccess,
		permissionSet.ProjectSnapshotFullAccess,
		permissionSet.ProjectVPCFullAccess,
		permissionSet.ProjectSubnetFullAccess,
		permissionSet.ProjectEipFullAccess,
		permissionSet.ProjectLoadBalancerFullAccess,
		permissionSet.ProjectSecurityGroupFullAccess,
		permissionSet.ProjectSSHFullAccess,
		permissionSet.ProjectKaaSFullAccess,
		permissionSet.ProjectBaaSFullAccess,
		permissionSet.ProjectBucketFullAccess,
		permissionSet.ProjectArgoCdAccess,
	}
)
