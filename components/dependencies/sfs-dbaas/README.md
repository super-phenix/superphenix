# SFS-DBAAS

![Version: 0.0.0](https://img.shields.io/badge/Version-0.0.0-informational?style=flat-square)

This Helm Chart is used by the self-service ArgoCDs of Superphénix to create PostgreSQL databases.

Each entry of `.databases` creates a [CloudNative-PG](https://cloudnative-pg.io/) cluster in the project's namespace.

### Resource names

Resources have a unique ID (the key under `.databases`) and a name.

Names do not have to be unique across resources and can be identical. They are simply used as a tag to be filtered and found more easily.

To set a name on a resource, use the `.name` value.

> [!note]
> Names **must** begin with a letter or number, and may contain letters, numbers, hyphens, dots, and underscores, up to 63 characters each.

### Setting a resource location

Resources are located within a specific AZ (or *availability zone*). AZs are represented by a unique code.

To deploy your resource in a specific AZ, simply use the code in the `.location` value of the resource.

### Network access

Instances get an interface on the subnet set in `.subnet`. Only the operator and the instances of the same database can reach them by default: add a network policy to the project to allow clients on port 5432.

Set `.loadBalancer.enabled` to get a fixed virtual IP that always targets the primary instance, including after a failover. The VIP must be part of `198.18.0.0/16`.

To expose the database outside of the VPC, point a DNAT of an Elastic IP at the VIP on port 5432.

### Updating a database

The following values can be changed on an existing database:

| Value | Effect |
|-------|--------|
| `.instances` | Adds or removes replicas |
| `.storage` | Grows the data volume of each instance, it cannot be shrunk |
| `.cpu`, `.memory` | Rolling restart of the instances, the primary last |
| `.version` | Rolling update to another minor version of the same major version |

---

## Values

<h3>Organization parameters</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>gitops</td>
			<td>bool</td>
			<td><pre lang="json">
false
</pre>
</td>
			<td>Whether this chart is deployed through GitOps or not. This parameter cannot be overriden by the user.</td>
		</tr>
		<tr>
			<td>location</td>
			<td>string</td>
			<td><pre lang="json">
""
</pre>
</td>
			<td>AZ in which we're deploying this chart. This parameter cannot be overriden by the user.</td>
		</tr>
		<tr>
			<td>organizationID</td>
			<td>string</td>
			<td><pre lang="json">
"00000000-0000-0000-0000-000000000000"
</pre>
</td>
			<td>Superphenix organization to which this project belongs, must be a generated UUIDv4. This ID must come from the SPX API, this parameter cannot be overriden by the user.</td>
		</tr>
		<tr>
			<td>organizationName</td>
			<td>string</td>
			<td><pre lang="json">
"null"
</pre>
</td>
			<td>Superphenix organization's friendly name. This parameter cannot be overriden by the user.</td>
		</tr>
		<tr>
			<td>projectID</td>
			<td>string</td>
			<td><pre lang="json">
"00000000-0000-0000-0000-000000000000"
</pre>
</td>
			<td>Superphenix project to which this project belongs, must be a project within the organization, must be a generated UUIDv4. This ID must come from the SPX API, this parameter cannot be overriden by the user.</td>
		</tr>
		<tr>
			<td>projectName</td>
			<td>string</td>
			<td><pre lang="json">
"null"
</pre>
</td>
			<td>Superphenix project's friendly name. This ID must come from the SPX API, this parameter cannot be overriden by the user.</td>
		</tr>
		<tr>
			<td>storageClassMapping</td>
			<td>object</td>
			<td><pre lang="json">
{}
</pre>
</td>
			<td>Mapping between user-friendly names of storage classes and their real names. The mapping is specified for the current AZ, meaning that each AZ can have a different mapping. This parameter cannot be overriden by the user.</td>
		</tr>
	</tbody>
</table>

<h3>Other Values</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
	<tr>
		<td>databases</td>
		<td>object</td>
		<td><pre lang="">
{}
</pre>
</td>
		<td>Defines PostgreSQL databases. Each key defines a new database and should be unique See values.yaml for the complete syntax.</td>
	</tr>
	</tbody>
</table>

