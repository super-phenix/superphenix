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

var (
	DefaultOrganizationOwner = []string{permissionSet.SpxOwner}
	DefaultProjectOwner      = []string{}

	DefaultOrganizationAdmin = []string{
		permissionSet.SpxMember,
		permissionSet.IAMFullAccess,
		permissionSet.SettingsEdition,
		permissionSet.BillingFullAccess,
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
		permissionSet.ProjectFirewallFullAccess,
		permissionSet.ProjectSSHFullAccess,
		permissionSet.ProjectKaaSFullAccess,
		permissionSet.ProjectBaaSFullAccess,
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
		permissionSet.ProjectFirewallFullAccess,
		permissionSet.ProjectSSHFullAccess,
		permissionSet.ProjectKaaSFullAccess,
		permissionSet.ProjectBaaSFullAccess,
		permissionSet.ProjectArgoCdAccess,
	}
)
