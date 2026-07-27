# SFS-IAAS

![Version: 1.4.0](https://img.shields.io/badge/Version-1.4.0-informational?style=flat-square)  ![Version: 1.4.0](https://img.shields.io/badge/Version-1.4.0-informational?style=flat-square)

This Helm Chart is used by the self-service ArgoCDs of Superphenix to generate IaaS resources

## Creating new resources

For each type of resource present in this chart (*VPC*, *Subnet*, *Disk*, ...), there's a dictionnary in which they can be created.
 
The key used to create a new resource within that dictionnary is the ID of the resource.

> [!caution]
> This ID **must** be unique across resources of the same type within the project. 
> If two resources of the same type have identical IDs, the one defined last in the dictionnary will override all others. 

For example, with a VPC:

```yaml
vpcs:
  new-vpc: # This is the unique ID of the resource, no other VPC can have it
    name: "vpc"
    location: "aq01-test01"
  new-vpc: # WARNING: this key will override the one above
     name: "vpc2"
     location: "aq01-test01"
```

> [!note]
> IDs **must** begin with a letter or number, and may contain letters, numbers, hyphens, dots, and underscores, up to 63 characters each.

### Resource names

Resources have a unique ID (see [this](#creating-a-new-resource)) and a name.

Names do not have to be unique across resources and can be identical. They are simply used as a tag to be filtered and found more easily.

To set a name on a resource, use the `.name` value.

> [!note]
> Names **must** begin with a letter or number, and may contain letters, numbers, hyphens, dots, and underscores, up to 63 characters each.

### Setting a resource location

Resources are located within a specific AZ (or *availability zone*). AZs are represented by a unique code.

To deploy your resource in a specific AZ, simply use the code in the `.location` value of the resource.

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
			<td>bgpSpeaker</td>
			<td>string</td>
			<td><pre lang="json">
""
</pre>
</td>
			<td>BGP speaker configuration for this AZ. The value is passed as an escape JSON and must be decoded in the template This parameter cannot be overriden by the user.</td>
		</tr>
		<tr>
			<td>gitops</td>
			<td>bool</td>
			<td><pre lang="json">
false
</pre>
</td>
			<td>Wether this chart is deployed through GitOps or not. This parameter cannot be overriden by the user.</td>
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
<h3>Storage</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>buckets</td>
			<td>object</td>
			<td><pre lang="">
""
</pre>
</td>
			<td>S3 buckets that can be created in the project through Ceph/Rook OBCs</td>
		</tr>
	</tbody>
</table>
<h3>Disk Snapshot Schedules</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>diskSnapshotSchedules</td>
			<td>string</td>
			<td><pre lang="">
""
</pre>
</td>
			<td>Schedules for snapshots of Disks</td>
		</tr>
	</tbody>
</table>
<h3>Disk Snapshots</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>diskSnapshots</td>
			<td>string</td>
			<td><pre lang="">
""
</pre>
</td>
			<td>Snapshots of Disks</td>
		</tr>
	</tbody>
</table>
<h3>Disks</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>disks</td>
			<td>object</td>
			<td><pre lang="">
""
</pre>
</td>
			<td>Disks that can be attached to VMs</td>
		</tr>
	</tbody>
</table>
<h3>Elastic IPs</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>eips</td>
			<td>string</td>
			<td><pre lang="">
"{}"
</pre>
</td>
			<td>Elastic IPs available in this project. See values.yaml for the complete syntax.</td>
		</tr>
	</tbody>
</table>
<h3>Global values</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>global.labels</td>
			<td>object</td>
			<td><pre lang="json">
{}
</pre>
</td>
			<td>Labels added to every single object templated by this chart. This parameter cannot be overriden by the user.</td>
		</tr>
	</tbody>
</table>
<h3>Load balancers</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>loadbalancers</td>
			<td>string</td>
			<td><pre lang="json">
null
</pre>
</td>
			<td>Load balancers to VMs, supports TCP/UDP and automatic healthchecks</td>
		</tr>
	</tbody>
</table>
<h3>Network Policies</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>networkPolicies</td>
			<td>string</td>
			<td><pre lang="json">
null
</pre>
</td>
			<td>Network policies (firewalling rules) to block/allow traffic in and out of the project</td>
		</tr>
	</tbody>
</table>
<h3>Replication</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>replication</td>
			<td>object</td>
			<td><pre lang="json">
{
  "availabilityZones": {},
  "schedule": ""
}
</pre>
</td>
			<td>Replication settings for disaster recovery of the project.</td>
		</tr>
		<tr>
			<td>replication.availabilityZones</td>
			<td>object</td>
			<td><pre lang="json">
{}
</pre>
</td>
			<td>Replication strategy per AZ (overrides .schedule) This setting can be used to enable replication on specific AZs. It can also be used to disable replication for a specific AZ if a global schedule has been configured.</td>
		</tr>
		<tr>
			<td>replication.schedule</td>
			<td>string</td>
			<td><pre lang="json">
""
</pre>
</td>
			<td>Replication schedule applied to every AZ. If specified, the project is set to be replicated in every AZ.</td>
		</tr>
	</tbody>
</table>
<h3>SSH key store</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>ssh</td>
			<td>object</td>
			<td><pre lang="">
""
</pre>
</td>
			<td>SSH access to the VirtualMachines</td>
		</tr>
		<tr>
			<td>ssh.publicKeys</td>
			<td>object</td>
			<td><pre lang="">
"{}"
</pre>
</td>
			<td>Public keys to inject inside the VMs. See value.yaml for the complete syntax, each key should be unique.</td>
		</tr>
	</tbody>
</table>
<h3>Subnets</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>subnets</td>
			<td>object</td>
			<td><pre lang="">
See the default Subnet for the syntax to define a new Subnet
</pre>
</td>
			<td>Each key defines a new Subnet, each key should be unique</td>
		</tr>
	</tbody>
</table>
<h3>VM Snapshots</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>vmSnapshots</td>
			<td>string</td>
			<td><pre lang="">
""
</pre>
</td>
			<td>Snapshots of Virtual Machines</td>
		</tr>
	</tbody>
</table>
<h3>Virtual Machines</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>vms</td>
			<td>string</td>
			<td><pre lang="">
"{}"
</pre>
</td>
			<td>Each key defines a new VM, each key should be unique. See values.yaml for the complete syntax.</td>
		</tr>
	</tbody>
</table>
<h3>VPCs</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>vpcs</td>
			<td>string</td>
			<td><pre lang="">
"{}"
</pre>
</td>
			<td>Each key defines a new VPC, each key should be unique. See the default VPC for the syntax to define a new VPC.</td>
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
		<td>vmRestores</td>
		<td>string</td>
		<td><pre lang="json">
null
</pre>
</td>
		<td></td>
	</tr>
	</tbody>
</table>

