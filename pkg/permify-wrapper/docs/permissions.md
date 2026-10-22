# Permissions Documentation

## Glossary

- **Permission**: A single right on one or more products.
- **PermissionSet**: A grouping of permissions.

## How Permissions Work

Permission management uses a group-based system.
A group is associated with one or more PermissionSets. Groups are cumulative.
A user can be assigned to one or more groups.
If assigned to multiple groups, the user benefits from all permissions granted by all their groups combined.

Permissions and PermissionSets are divided into two categories:
those associated with an **organization** and those associated with a **project**.

## Predefined vs. custom groups

**Predefined groups** are Owner, Admin, Billing and Developer, defined by the `PredefinedGroups`
catalog in `pkg/base/v1/schema_default.go`. Each is persisted with its catalog key (`owner`,
`admin`, `billing`, `developer`) in `groups.predefined_key`. The key never changes, so the display
name can be relabelled without losing track of the group. Predefined groups cannot be edited or
deleted through the API (403) and do not count against the organization's IAM group quota.

**Custom groups** have a `NULL` `predefined_key`, which is what keeps them out of the reconciler.
`POST /v1/organization/{orgaId}/iam/group/{groupId}/duplicate` returns an editable copy of any
group, with no key.

### Keeping predefined groups up to date

Each organization records the catalog version it is aligned on in
`organizations.predefined_catalog_version`. On API startup, and from
`POST /admin/permission/update-schema`, the reconciler selects the organizations below
`PredefinedCatalogVersion`, pushes the default Permify schema, then upserts every catalog entry by
key: missing ones are created, existing ones overwritten with the catalog's name and PermissionSets.
The version is recorded only once every group of that organization succeeded, so a failure is
retried on the next pass. Pass `{"force": true}` to the admin endpoint to re-run against every
organization regardless of its version.

Adding a PermissionSet to a predefined group is therefore:

1. Declare the relation in `pkg/base/v1/schema/schema_default.perm`.
2. Add the constant in `pkg/base/v1/permissionSet/permissionSet.go` and its entity mapping.
3. Add it to the relevant entry of `PredefinedGroups`.
4. Bump `PredefinedCatalogVersion`.

Every live organization picks the change up on the next API start.

## Permissions by Right

### Organization Permissions

| Permission                    | Associated Right                                                                                                  |
|-------------------------------|-------------------------------------------------------------------------------------------------------------------|
| OrganizationRead              | Read basic information about the organization                                                                     |
| OrganizationWrite             | Modify basic information of the organization                                                                      |
| OrganizationIAMRead           | View the organization's users, their groups, and the list of groups defined in the organization                   |
| OrganizationIAMWrite          | Modify the organization's user list, their groups, and the permissions associated with each group                 |
| OrganizationBillingRead       | View the organization's billing information                                                                       |
| OrganizationBillingWrite      | Modify the organization's billing information                                                                     |
| OrganizationProjectManagement | Manage the organization's projects (creation, renaming, and deletion)                                             |
| OrganizationProjectQuotaRead  | View the organization quota and the quotas allocated to its projects                                              |
| OrganizationProjectQuotaWrite | Modify quotas allocated to organization projects                                                                  |

### Project Permissions

| Permission               | Associated Right                                                             |
|--------------------------|------------------------------------------------------------------------------|
| ProjectRead              | Read basic information about the project                                     |
| ProjectInstanceRead      | View the list and details of the project's instances                         |
| ProjectInstanceTerminal  | Access the console / VNC of an instance                                      |
| ProjectInstanceControl   | Control an instance (start, stop, restart)                                   |
| ProjectInstanceWrite     | Create, modify, and delete an instance                                       |
| ProjectDiskRead          | View the list and details of the project's disk storage                      |
| ProjectDiskWrite         | Create, modify, and delete disk storage                                      |
| ProjectSnapshotRead      | View the list and details of the project's snapshots (or instance snapshots) |
| ProjectSnapshotWrite     | Create, modify, and delete a snapshot (or instance snapshot)                 |
| ProjectVPCRead           | View the list and details of the project's VPCs                              |
| ProjectVPCWrite          | Create, modify, and delete a VPC                                             |
| ProjectSubnetRead        | View the list and details of the project's Subnets                           |
| ProjectSubnetWrite       | Create, modify, and delete a Subnet                                          |
| ProjectEipRead           | View the list and details of the project's EIPs                              |
| ProjectEipWrite          | Create, modify, and delete an EIP                                            |
| ProjectSSHRead           | View the list and details of the project's SSH keys                          |
| ProjectSSHWrite          | Create, modify, and delete SSH keys                                          |
| ProjectLoadBalancerRead  | View the list and details of the project's Load Balancers                    |
| ProjectLoadBalancerWrite | Create, modify, and delete a Load Balancer                                   |
| ProjectSecurityGroupRead      | View the list and details of the project's Security Groups                         |
| ProjectSecurityGroupWrite     | Create, modify, and delete a Security Group                                        |
| ProjectKaaSRead          | View the list and details of the project's KaaS clusters                     |
| ProjectKaaSKubeConfig    | Retrieve the KubeConfig configuration file of a cluster                      |
| ProjectKaaSWrite         | Create, modify, and delete a KaaS cluster                                    |
| ProjectBaaSRead          | View the list and details of the project's BaaS backups                      |
| ProjectBaaSWrite         | Create, modify, and delete a BaaS backup                                     |
| ProjectAZQuotaRead       | View the project quota and its availability zone quotas                      |
| ProjectAZQuotaWrite      | Modify availability zone quotas for the project                              |
| ProjectArgoCdRead        | View the list and details of the project's Argo CD applications              |

## PermissionSets by Right

### Organization PermissionSets

| PermissionSet     | Associated Permissions                                |
|-------------------|-------------------------------------------------------|
| IAMFullAccess     | `OrganizationIAMRead`, `OrganizationIAMWrite`         |
| IAMReadOnly       | `OrganizationIAMRead`                                 |
| BillingFullAccess | `OrganizationBillingRead`, `OrganizationBillingWrite` |
| BillingReadOnly   | `OrganizationBillingRead`                             |
| ProjectManagement | `OrganizationProjectManagement`                       |
| ProjectQuotaFullAccess | `OrganizationProjectQuotaRead`, `OrganizationProjectQuotaWrite`, `ProjectAZQuotaRead` (every project) |
| ProjectQuotaReadOnly   | `OrganizationProjectQuotaRead`, `ProjectAZQuotaRead` (every project) |
| SettingsEdition   | `OrganizationWrite`                                   |

### Project PermissionSets

| PermissionSet                 | Associated Permissions                                                                             |
|-------------------------------|----------------------------------------------------------------------------------------------------|
| ProjectInstanceFullAccess     | `ProjectInstanceRead`, `ProjectInstanceTerminal`, `ProjectInstanceControl`, `ProjectInstanceWrite` |
| ProjectInstanceTerminalAccess | `ProjectInstanceRead`, `ProjectInstanceTerminal`                                                   |
| ProjectInstanceReadOnly       | `ProjectInstanceRead`                                                                              |
| ProjectDiskFullAccess         | `ProjectDiskRead`, `ProjectDiskWrite`                                                              |
| ProjectDiskReadOnly           | `ProjectDiskRead`                                                                                  |
| ProjectSnapshotFullAccess     | `ProjectSnapshotRead`, `ProjectSnapshotWrite`                                                      |
| ProjectSnapshotReadOnly       | `ProjectSnapshotRead`                                                                              |
| ProjectVPCFullAccess          | `ProjectVPCRead`, `ProjectVPCWrite`                                                                |
| ProjectVPCReadOnly            | `ProjectVPCRead`                                                                                   |
| ProjectSubnetFullAccess       | `ProjectSubnetRead`, `ProjectSubnetWrite`                                                          |
| ProjectSubnetReadOnly         | `ProjectSubnetRead`                                                                                |
| ProjectEipFullAccess          | `ProjectEipRead`, `ProjectEipWrite`                                                                |
| ProjectEipReadOnly            | `ProjectEipRead`                                                                                   |
| ProjectSSHFullAccess          | `ProjectSSHRead`, `ProjectSSHWrite`                                                                |
| ProjectSSHReadOnly            | `ProjectSSHRead`                                                                                   |
| ProjectLoadBalancerFullAccess | `ProjectLoadBalancerRead`, `ProjectLoadBalancerWrite`                                              |
| ProjectLoadBalancerReadOnly   | `ProjectLoadBalancerRead`                                                                          |
| ProjectSecurityGroupFullAccess     | `ProjectSecurityGroupRead`, `ProjectSecurityGroupWrite`                                                      |
| ProjectSecurityGroupReadOnly       | `ProjectSecurityGroupRead`                                                                              |
| ProjectKaaSFullAccess         | `ProjectKaaSRead`, `ProjectKaaSKubeConfig`, `ProjectKaaSWrite`                                     |
| ProjectKaaSClusterAccess      | `ProjectKaaSRead`, `ProjectKaaSKubeConfig`                                                         |
| ProjectKaaSReadOnly           | `ProjectKaaSRead`                                                                                  |
| ProjectBaaSFullAccess         | `ProjectBaaSRead`, `ProjectBaaSWrite`                                                              |
| ProjectBaaSReadOnly           | `ProjectBaaSRead`                                                                                  |
| ProjectAZQuotaFullAccess      | `ProjectAZQuotaRead`, `ProjectAZQuotaWrite`                                                        |
| ProjectAZQuotaReadOnly        | `ProjectAZQuotaRead`                                                                               |
| ProjectArgoCdAccess           | `ProjectArgoCdRead`                                                                                |

### Internal PermissionSets

These PermissionSets are not visible to API users.
They define minimum rights when a user joins an organization (`SpxMember`)
and maximum rights for organization owners (`SpxOwner`).

| PermissionSet | Associated Permissions                                                        |
|---------------|-------------------------------------------------------------------------------|
| SpxOwner      | Grants all permissions on the organization and all its projects               |
| SpxMember     | Grants the `OrganizationRead` permission                                      |
