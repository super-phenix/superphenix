# Superphenix ID

ID management module for the Superphenix project. Implements the SPX ID framework defined in SPX-RFC0001 and the resource labeling scheme defined in SPX-RFC0002.

## ID Format

Every Superphenix ID is presented as `{prefix}-{uuid}` (e.g., `spx-550e8400-e29b-41d4-a716-446655440000`).

- **Prefix** — Configurable via `Configure(prefix, contextName)`. Default: `spx`.
- **Organization ID** — UUIDv4 identifying the organization.
- **Project ID** — UUIDv4 identifying the project within an organization.
- **Resource Local ID** — A human-readable name (`^[a-z0-9]+(-[a-z0-9]+)*$`, max 63 characters).
- **Resource Effective ID** — UUIDv5 derived from the project ID (namespace) and the resource local ID (name). This guarantees a deterministic, unique identifier per resource within a project.

## Packages

### `superphenix-id` (root)

Core ID generation and validation.

- `Metadata` — Embeddable struct carrying `OrgId`, `ProjectId`, `ResourceEffectiveId`, and `ResourceLocalId`.
- `GenerateMetadata(projectId, orgId, localID)` — Populates metadata and computes the effective ID.
- `GenerateFromProject(project, localID)` — Derives resource metadata from a parent project.
- `GetLabels()` — Returns Kubernetes labels (organization, project, local ID, effective ID) as defined by SPX-RFC0002.
- `ToSPXID(id)` — Formats a raw UUID as a presentable SPX ID (`{prefix}-{uuid}`).
- `Configure(prefix, contextName)` — Sets the global framework prefix and context key.

### `superphenix-id/middleware`

Chi middleware for automatic SPX ID resolution from HTTP route parameters.

- Extracts organization ID, project ID, and resource local ID from URL parameters.
- Computes the effective ID and injects it into the request context.

