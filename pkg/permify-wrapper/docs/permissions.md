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
| ProjectFirewallRead      | View the list and details of the project's Firewalls                         |
| ProjectFirewallWrite     | Create, modify, and delete a Firewall                                        |
| ProjectKaaSRead          | View the list and details of the project's KaaS clusters                     |
| ProjectKaaSKubeConfig    | Retrieve the KubeConfig configuration file of a cluster                      |
| ProjectKaaSWrite         | Create, modify, and delete a KaaS cluster                                    |
| ProjectBaaSRead          | View the list and details of the project's BaaS backups                      |
| ProjectBaaSWrite         | Create, modify, and delete a BaaS backup                                     |
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
| ProjectFirewallFullAccess     | `ProjectFirewallRead`, `ProjectFirewallWrite`                                                      |
| ProjectFirewallReadOnly       | `ProjectFirewallRead`                                                                              |
| ProjectKaaSFullAccess         | `ProjectKaaSRead`, `ProjectKaaSKubeConfig`, `ProjectKaaSWrite`                                     |
| ProjectKaaSClusterAccess      | `ProjectKaaSRead`, `ProjectKaaSKubeConfig`                                                         |
| ProjectKaaSReadOnly           | `ProjectKaaSRead`                                                                                  |
| ProjectBaaSFullAccess         | `ProjectBaaSRead`, `ProjectBaaSWrite`                                                              |
| ProjectBaaSReadOnly           | `ProjectBaaSRead`                                                                                  |
| ProjectArgoCdAccess           | `ProjectArgoCdRead`                                                                                |

### Internal PermissionSets

These PermissionSets are not visible to API users.
They define minimum rights when a user joins an organization (`SpxMember`)
and maximum rights for organization owners (`SpxOwner`).

| PermissionSet | Associated Permissions                                                        |
|---------------|-------------------------------------------------------------------------------|
| SpxOwner      | Grants all permissions on the organization and all its projects               |
| SpxMember     | Grants the `OrganizationRead` permission                                      |
