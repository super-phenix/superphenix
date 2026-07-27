# Permify Wrapper

A Go wrapper module for [Permify](https://permify.co/), providing middleware and utilities for permission verification in Superphenix services.

## Permission Model Overview

The wrapper implements a group-based permission system with two levels:

- **Organization permissions** — Control access to organization-level operations (settings, IAM, billing, project management).
- **Project permissions** — Control access to resource-level operations within a project (instances, disks, VPCs, etc.).

Permissions are grouped into **PermissionSets** (e.g., `ProjectInstanceFullAccess`, `IAMReadOnly`). Users are assigned to **groups**, and each group is associated with one or more PermissionSets. Groups are cumulative — a user in multiple groups receives the union of all their permissions.

Two internal PermissionSets are automatically applied:
- `SpxOwner` — Grants all permissions on the organization and all its projects.
- `SpxMember` — Grants `OrganizationRead` (minimum access for any member).

For the full list of permissions and PermissionSets, see [Permissions documentation](./docs/permissions.md).

## Packages

- [base](./pkg/base) — Permission and PermissionSet constants, entity definitions, and default schema configuration.
- [bundle](./pkg/bundle) — Schema bundle management for Permify.
- [client](./pkg/client) — Permify gRPC client initialization and connection management.
- [data](./pkg/data) — Data structures for permission relationships and tuples.
- [middleware](./pkg/middleware) — HTTP middleware for permission checks in Chi routers.
- [permission](./pkg/permission) — Permission checking and listing functions (check single permission, list user permissions).
- [schema](./pkg/schema) — Permify schema writing and management.
- [tenancy](./pkg/tenancy) — Multi-tenant support for Permify.

## Usage

Initialize the Permify client before using any wrapper methods:

```go
import "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/client"

err := client.InitPermify("permify.server:3478")
```

### Checking permissions

```go
import "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/permission"

allowed, err := permission.CheckPermission(ctx, organizationId, entity.Project, projectId, userId, "ProjectInstanceRead")
```

### Using the middleware

```go
import "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/middleware"

r.With(middleware.CheckPermission("ProjectInstanceWrite")).Post("/instances", handler)
```

### Managing groups and relationships

```go
import pkg "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg"

err := pkg.UpdateRelationGroupPermissions(ctx, orgId, pkg.Group{
    Id:             groupId,
    OrgaId:         orgId,
    ProjectIds:     []string{projectId},
    PermissionSets: v1.DefaultProjectDeveloper,
})
```
