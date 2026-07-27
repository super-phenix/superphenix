package permissionSet

import (
	"strings"

	"github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/entity"
)

const InternalPrefix = "spx_"

// PermissionSets
const (
	SpxOwner          = "spx_owner"
	SpxMember         = "spx_member"
	IAMFullAccess     = "IAMFullAccess"
	IAMReadOnly       = "IAMReadOnly"
	BillingFullAccess = "BillingFullAccess"
	BillingReadOnly   = "BillingReadOnly"
	ProjectManagement = "ProjectManagement"
	SettingsEdition   = "SettingsEdition"

	ProjectInstanceFullAccess     = "ProjectInstanceFullAccess"
	ProjectInstanceTerminalAccess = "ProjectInstanceTerminalAccess"
	ProjectInstanceReadOnly       = "ProjectInstanceReadOnly"

	ProjectDiskFullAccess = "ProjectDiskFullAccess"
	ProjectDiskReadOnly   = "ProjectDiskReadOnly"

	ProjectSnapshotFullAccess = "ProjectSnapshotFullAccess"
	ProjectSnapshotReadOnly   = "ProjectSnapshotReadOnly"

	ProjectVPCFullAccess          = "ProjectVPCFullAccess"
	ProjectVPCReadOnly            = "ProjectVPCReadOnly"
	ProjectSubnetFullAccess       = "ProjectSubnetFullAccess"
	ProjectSubnetReadOnly         = "ProjectSubnetReadOnly"
	ProjectEipFullAccess          = "ProjectEipFullAccess"
	ProjectEipReadOnly            = "ProjectEipReadOnly"
	ProjectLoadBalancerFullAccess = "ProjectLoadBalancerFullAccess"
	ProjectLoadBalancerReadOnly   = "ProjectLoadBalancerReadOnly"
	ProjectFirewallFullAccess     = "ProjectFirewallFullAccess"
	ProjectFirewallReadOnly       = "ProjectFirewallReadOnly"

	ProjectSSHFullAccess = "ProjectSSHFullAccess"
	ProjectSSHReadOnly   = "ProjectSSHReadOnly"

	ProjectKaaSFullAccess    = "ProjectKaaSFullAccess"
	ProjectKaaSClusterAccess = "ProjectKaaSClusterAccess"
	ProjectKaaSReadOnly      = "ProjectKaaSReadOnly"

	ProjectBaaSFullAccess = "ProjectBaaSFullAccess"
	ProjectBaaSReadOnly   = "ProjectBaaSReadOnly"

	ProjectArgoCdAccess = "ProjectArgoCdAccess"
)

// PermissionSetsEntityMap is the association between PermissionSet and Permify Entity
var PermissionSetsEntityMap = map[string]string{
	SpxOwner:          entity.Organization,
	SpxMember:         entity.Organization,
	IAMFullAccess:     entity.Organization,
	IAMReadOnly:       entity.Organization,
	SettingsEdition:   entity.Organization,
	BillingFullAccess: entity.Organization,
	BillingReadOnly:   entity.Organization,
	ProjectManagement: entity.Organization,

	ProjectInstanceFullAccess:     entity.Project,
	ProjectInstanceTerminalAccess: entity.Project,
	ProjectInstanceReadOnly:       entity.Project,

	ProjectDiskFullAccess: entity.Project,
	ProjectDiskReadOnly:   entity.Project,

	ProjectSnapshotFullAccess: entity.Project,
	ProjectSnapshotReadOnly:   entity.Project,

	ProjectVPCFullAccess: entity.Project,
	ProjectVPCReadOnly:   entity.Project,

	ProjectSubnetFullAccess: entity.Project,
	ProjectSubnetReadOnly:   entity.Project,

	ProjectEipFullAccess: entity.Project,
	ProjectEipReadOnly:   entity.Project,

	ProjectLoadBalancerFullAccess: entity.Project,
	ProjectLoadBalancerReadOnly:   entity.Project,

	ProjectFirewallFullAccess: entity.Project,
	ProjectFirewallReadOnly:   entity.Project,

	ProjectSSHFullAccess: entity.Project,
	ProjectSSHReadOnly:   entity.Project,

	ProjectKaaSFullAccess:    entity.Project,
	ProjectKaaSClusterAccess: entity.Project,
	ProjectKaaSReadOnly:      entity.Project,

	ProjectBaaSFullAccess: entity.Project,
	ProjectBaaSReadOnly:   entity.Project,

	ProjectArgoCdAccess: entity.Project,
}

var EntityPermissionSetsMap = make(map[string][]string)

func init() {
	var permissionSetsMap = make(map[string][]string)
	for k, v := range PermissionSetsEntityMap {
		// We don't want to list internal permissionSet
		// It's a default permissionSet that every user should have
		if !strings.HasPrefix(k, InternalPrefix) {
			permissionSetsMap[v] = append(permissionSetsMap[v], k)
		}
	}
	EntityPermissionSetsMap = permissionSetsMap
}
