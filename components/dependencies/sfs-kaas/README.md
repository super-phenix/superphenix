![Version: 0.0.0](https://img.shields.io/badge/Version-0.0.0-informational?style=flat-square)

This Helm Chart is used by the self-service ArgoCDs of Superphénix to create Kubernetes clusters.

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
			<td>organizationID</td>
			<td>string</td>
			<td><pre lang="json">
"00000000-0000-0000-0000-000000000000"
</pre>
</td>
			<td>Superphénix organization to which this project belongs, must be a generated UUIDv4. This ID must come from the SPX API, this parameter cannot be overriden by the user.</td>
		</tr>
		<tr>
			<td>organizationName</td>
			<td>string</td>
			<td><pre lang="json">
"null"
</pre>
</td>
			<td>Superphénix organization's friendly name. This parameter cannot be overriden by the user.</td>
		</tr>
		<tr>
			<td>projectID</td>
			<td>string</td>
			<td><pre lang="json">
"00000000-0000-0000-0000-000000000000"
</pre>
</td>
			<td>Superphénix project to which this project belongs, must be a project within the organization, must be a generated UUIDv4. This ID must come from the SPX API, this parameter cannot be overriden by the user.</td>
		</tr>
		<tr>
			<td>projectName</td>
			<td>string</td>
			<td><pre lang="json">
"null"
</pre>
</td>
			<td>Superphénix project's friendly name. This ID must come from the SPX API, this parameter cannot be overriden by the user.</td>
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
		<td>azCount</td>
		<td>int</td>
		<td><pre lang="json">
10
</pre>
</td>
		<td>Maximum mumber of AZs in a region (for cluster migrations). This parameter cannot be overriden by the user.</td>
	</tr>
	<tr>
		<td>azDomains</td>
		<td>object</td>
		<td><pre lang="json">
{}
</pre>
</td>
		<td>Per AZ mapping of URLs to use in cluster configuration This parameter cannot be overriden by the user.</td>
	</tr>
	<tr>
		<td>clusters</td>
		<td>object</td>
		<td><pre lang="">
{}
</pre>
</td>
		<td>Defines Kubernetes clusters. Each key defines a new cluster and should be unique See values.yaml for the complete syntax.</td>
	</tr>
	<tr>
		<td>defaultImageUrl</td>
		<td>string</td>
		<td><pre lang="json">
"docker://ghcr.io/super-phenix/kaas-image"
</pre>
</td>
		<td>Image URL for the worker VMs This parameter cannot be overriden by the user.</td>
	</tr>
	<tr>
		<td>gitops</td>
		<td>bool</td>
		<td><pre lang="json">
false
</pre>
</td>
		<td>Whether this chart is deployed through GitOps or not This parameter cannot be overriden by the user.</td>
	</tr>
	<tr>
		<td>kubevirtCsi."v1.35".csiAttacherVersion</td>
		<td>string</td>
		<td><pre lang="json">
"v4.11.0"
</pre>
</td>
		<td></td>
	</tr>
	<tr>
		<td>kubevirtCsi."v1.35".csiDriverVersion</td>
		<td>string</td>
		<td><pre lang="json">
"latest"
</pre>
</td>
		<td></td>
	</tr>
	<tr>
		<td>kubevirtCsi."v1.35".csiLivenessprobeVersion</td>
		<td>string</td>
		<td><pre lang="json">
"v2.18.0"
</pre>
</td>
		<td></td>
	</tr>
	<tr>
		<td>kubevirtCsi."v1.35".csiProvisionerVersion</td>
		<td>string</td>
		<td><pre lang="json">
"v6.2.0"
</pre>
</td>
		<td></td>
	</tr>
	<tr>
		<td>kubevirtCsi."v1.35".csiResizerVersion</td>
		<td>string</td>
		<td><pre lang="json">
"v2.1.0"
</pre>
</td>
		<td></td>
	</tr>
	<tr>
		<td>kubevirtCsi."v1.35".csiSnapshotterVersion</td>
		<td>string</td>
		<td><pre lang="json">
"v8.5.0"
</pre>
</td>
		<td></td>
	</tr>
	<tr>
		<td>kubevirtCsi."v1.36".csiAttacherVersion</td>
		<td>string</td>
		<td><pre lang="json">
"v4.12.0"
</pre>
</td>
		<td></td>
	</tr>
	<tr>
		<td>kubevirtCsi."v1.36".csiDriverVersion</td>
		<td>string</td>
		<td><pre lang="json">
"latest"
</pre>
</td>
		<td></td>
	</tr>
	<tr>
		<td>kubevirtCsi."v1.36".csiLivenessprobeVersion</td>
		<td>string</td>
		<td><pre lang="json">
"v2.19.0"
</pre>
</td>
		<td></td>
	</tr>
	<tr>
		<td>kubevirtCsi."v1.36".csiProvisionerVersion</td>
		<td>string</td>
		<td><pre lang="json">
"v6.3.0"
</pre>
</td>
		<td></td>
	</tr>
	<tr>
		<td>kubevirtCsi."v1.36".csiResizerVersion</td>
		<td>string</td>
		<td><pre lang="json">
"v2.2.1"
</pre>
</td>
		<td></td>
	</tr>
	<tr>
		<td>kubevirtCsi."v1.36".csiSnapshotterVersion</td>
		<td>string</td>
		<td><pre lang="json">
"v8.6.0"
</pre>
</td>
		<td></td>
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
	</tbody>
</table>

