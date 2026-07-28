package permission

import "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/entity"

// Permissions
const (
	OrganizationRead              = "OrganizationRead"
	OrganizationWrite             = "OrganizationWrite"
	OrganizationIAMRead           = "OrganizationIAMRead"
	OrganizationIAMWrite          = "OrganizationIAMWrite"
	OrganizationBillingRead       = "OrganizationBillingRead"
	OrganizationBillingWrite      = "OrganizationBillingWrite"
	OrganizationProjectManagement = "OrganizationProjectManagement"
	ProjectRead                   = "ProjectRead"

	ProjectInstanceRead     = "ProjectInstanceRead"
	ProjectInstanceTerminal = "ProjectInstanceTerminal"
	ProjectInstanceControl  = "ProjectInstanceControl"
	ProjectInstanceWrite    = "ProjectInstanceWrite"

	ProjectDiskRead  = "ProjectDiskRead"
	ProjectDiskWrite = "ProjectDiskWrite"

	ProjectSnapshotRead  = "ProjectSnapshotRead"
	ProjectSnapshotWrite = "ProjectSnapshotWrite"

	ProjectVPCRead  = "ProjectVPCRead"
	ProjectVPCWrite = "ProjectVPCWrite"

	ProjectSubnetRead  = "ProjectSubnetRead"
	ProjectSubnetWrite = "ProjectSubnetWrite"

	ProjectEipRead  = "ProjectEipRead"
	ProjectEipWrite = "ProjectEipWrite"

	ProjectLoadBalancerRead  = "ProjectLoadBalancerRead"
	ProjectLoadBalancerWrite = "ProjectLoadBalancerWrite"

	ProjectFirewallRead  = "ProjectFirewallRead"
	ProjectFirewallWrite = "ProjectFirewallWrite"

	ProjectSSHRead  = "ProjectSSHRead"
	ProjectSSHWrite = "ProjectSSHWrite"

	ProjectKaaSRead       = "ProjectKaaSRead"
	ProjectKaaSKubeConfig = "ProjectKaaSKubeConfig"
	ProjectKaaSWrite      = "ProjectKaaSWrite"

	ProjectBaaSRead  = "ProjectBaaSRead"
	ProjectBaaSWrite = "ProjectBaaSWrite"

	ProjectBucketRead        = "ProjectBucketRead"
	ProjectBucketCredentials = "ProjectBucketCredentials"
	ProjectBucketWrite       = "ProjectBucketWrite"

	ProjectArgoCdRead = "ProjectArgoCdRead"
)

// PermissionsEntityMap is the association between Permission and Permify Entity
var PermissionsEntityMap = map[string]string{
	OrganizationRead:              entity.Organization,
	OrganizationWrite:             entity.Organization,
	OrganizationIAMRead:           entity.Organization,
	OrganizationIAMWrite:          entity.Organization,
	OrganizationBillingRead:       entity.Organization,
	OrganizationBillingWrite:      entity.Organization,
	OrganizationProjectManagement: entity.Organization,
	ProjectRead:                   entity.Project,

	ProjectInstanceRead:     entity.Project,
	ProjectInstanceTerminal: entity.Project,
	ProjectInstanceControl:  entity.Project,
	ProjectInstanceWrite:    entity.Project,

	ProjectDiskRead:  entity.Project,
	ProjectDiskWrite: entity.Project,

	ProjectSnapshotRead:  entity.Project,
	ProjectSnapshotWrite: entity.Project,

	ProjectVPCRead:           entity.Project,
	ProjectVPCWrite:          entity.Project,
	ProjectSubnetRead:        entity.Project,
	ProjectSubnetWrite:       entity.Project,
	ProjectEipRead:           entity.Project,
	ProjectEipWrite:          entity.Project,
	ProjectLoadBalancerRead:  entity.Project,
	ProjectLoadBalancerWrite: entity.Project,
	ProjectFirewallRead:      entity.Project,
	ProjectFirewallWrite:     entity.Project,

	ProjectSSHRead:  entity.Project,
	ProjectSSHWrite: entity.Project,

	ProjectKaaSRead:       entity.Project,
	ProjectKaaSKubeConfig: entity.Project,
	ProjectKaaSWrite:      entity.Project,

	ProjectBaaSRead:  entity.Project,
	ProjectBaaSWrite: entity.Project,

	ProjectBucketRead:        entity.Project,
	ProjectBucketCredentials: entity.Project,
	ProjectBucketWrite:       entity.Project,

	ProjectArgoCdRead: entity.Project,
}
