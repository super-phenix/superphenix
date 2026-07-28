# spx-policies
![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square)
A Helm chart for templating Validating Admission Policies
## Values
<h3>DataVolume Labels and Annotations</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>policies.dv-metadata</td>
			<td>object</td>
			<td><pre lang="">
"{}"
</pre>
</td>
			<td>Check that all DataVolumes have the correct labels and annotations</td>
		</tr>
		<tr>
			<td>policies.dv-metadata.debug</td>
			<td>bool</td>
			<td><pre lang="json">
false
</pre>
</td>
			<td>If debug is True, the VAP only matches resources having the "test-janna" label.</td>
		</tr>
		<tr>
			<td>policies.dv-metadata.defaultNetwork</td>
			<td>string</td>
			<td><pre lang="json">
"kube-system/system-isolated-egress"
</pre>
</td>
			<td>Value of the "default-network" annotation.</td>
		</tr>
		<tr>
			<td>policies.dv-metadata.matchConditions</td>
			<td>string</td>
			<td><pre lang="json">
"[spxid, notNabok]"
</pre>
</td>
			<td>Match conditions to target only specific resources. Possible values are: "spxid" (either orgID, projectID, resEffID, namespace or name are SPXIDs), "notGenerated" (doesn't have a "generated" label), "notSourcedVolSnap" (volume snapshot without labels starting with "snapshot.kubevirt.io/source-vm"), "notCsiDriver" (volume snapshot without a "csi-driver/cluster" label), "notTmpSnapshot" (volume snapshot without a name starting with "tmp-snapshot-*"), "virtLauncher" (pod whose name starts with "virt-launcher"), "systemWorkloads" (pod with certain "superphenix.net/workloadClass" label values). If multiple conditions are provided, ALL must be true for a policy to be evaluated. If ANY is false, there is no match.</td>
		</tr>
		<tr>
			<td>policies.dv-metadata.validationActions</td>
			<td>string</td>
			<td><pre lang="json">
"[Deny, Audit]"
</pre>
</td>
			<td>Actions to take if the validation fails. Possible values are: Deny, Warn, Audit.</td>
		</tr>
	</tbody>
</table>
<h3>DataVolume Sources</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>

		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>policies.dv-sources</td>
			<td>object</td>
			<td><pre lang="">
"{}"
</pre>
</td>
			<td>Check that all DataVolumes have valid sources and references to other resources</td>
		</tr>
		<tr>
			<td>policies.dv-sources.debug</td>
			<td>bool</td>
			<td><pre lang="json">
false
</pre>
</td>
			<td>If debug is True, the VAP only matches resources having the "test-janna" label.</td>
		</tr>
		<tr>
			<td>policies.dv-sources.matchConditions</td>
			<td>string</td>
			<td><pre lang="json">
"[spxid, notNabok]"
</pre>
</td>
			<td>Match conditions to target only specific resources. Possible values are: "spxid" (either orgID, projectID, resEffID, namespace or name are SPXIDs), "notGenerated" (doesn't have a "generated" label), "notSourcedVolSnap" (volume snapshot without labels starting with "snapshot.kubevirt.io/source-vm"), "notCsiDriver" (volume snapshot without a "csi-driver/cluster" label), "notTmpSnapshot" (volume snapshot without a name starting with "tmp-snapshot-*"), "virtLauncher" (pod whose name starts with "virt-launcher"), "systemWorkloads" (pod with certain "superphenix.net/workloadClass" label values). If multiple conditions are provided, ALL must be true for a policy to be evaluated. If ANY is false, there is no match.</td>
		</tr>
		<tr>
			<td>policies.dv-sources.validationActions</td>
			<td>string</td>
			<td><pre lang="json">
"[Deny, Audit]"
</pre>
</td>
			<td>Actions to take if the validation fails. Possible values are: Deny, Warn, Audit.</td>
		</tr>
	</tbody>
</table>
<h3>Labels (IaaS)</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>policies.iaas-labels</td>
			<td>object</td>
			<td><pre lang="">
"{}"
</pre>
</td>
			<td>Check that all IaaS resources have the necessary labels with correct naming</td>
		</tr>
		<tr>
			<td>policies.iaas-labels.debug</td>
			<td>bool</td>
			<td><pre lang="json">
false
</pre>
</td>
			<td>If debug is True, the VAP only matches resources having the "test-janna" label.</td>
		</tr>
		<tr>
			<td>policies.iaas-labels.matchConditions</td>

			<td>string</td>
			<td><pre lang="json">
"[spxid, notSourcedVolSnap, notTmpSnapshot, notNabok]"
</pre>
</td>
			<td>Match conditions to target only specific resources. Possible values are: "spxid" (either orgID, projectID, resEffID, namespace or name are SPXIDs), "notGenerated" (doesn't have a "generated" label), "notSourcedVolSnap" (volume snapshot without labels starting with "snapshot.kubevirt.io/source-vm"), "notCsiDriver" (volume snapshot without a "csi-driver/cluster" label), "notTmpSnapshot" (volume snapshot without a name starting with "tmp-snapshot-*"), "virtLauncher" (pod whose name starts with "virt-launcher"), "systemWorkloads" (pod with certain "superphenix.net/workloadClass" label values). If multiple conditions are provided, ALL must be true for a policy to be evaluated. If ANY is false, there is no match.</td>
		</tr>
		<tr>
			<td>policies.iaas-labels.validationActions</td>
			<td>string</td>
			<td><pre lang="json">
"[Deny, Audit]"
</pre>
</td>
			<td>Actions to take if the validation fails. Possible values are: Deny, Warn, Audit.</td>
		</tr>
	</tbody>
</table>
<h3>Name is EffectiveID</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>policies.iaas-name-is-effective-id</td>
			<td>object</td>
			<td><pre lang="">
"{}"
</pre>
</td>
			<td>Check that all IaaS resources' names match their effective IDs</td>
		</tr>
		<tr>
			<td>policies.iaas-name-is-effective-id.debug</td>
			<td>bool</td>
			<td><pre lang="json">
false
</pre>
</td>
			<td>If debug is True, the VAP only matches resources having the "test-janna" label.</td>
		</tr>
		<tr>
			<td>policies.iaas-name-is-effective-id.matchConditions</td>
			<td>string</td>
			<td><pre lang="json">
"[spxid, notGenerated, notSourcedVolSnap, notTmpSnapshot, notVeleroVolsnap, notNabok]"
</pre>
</td>
			<td>Match conditions to target only specific resources. Possible values are: "spxid" (either orgID, projectID, resEffID, namespace or name are SPXIDs), "notGenerated" (doesn't have a "generated" label), "notSourcedVolSnap" (volume snapshot without labels starting with "snapshot.kubevirt.io/source-vm"), "notCsiDriver" (volume snapshot without a "csi-driver/cluster" label), "notTmpSnapshot" (volume snapshot without a name starting with "tmp-snapshot-*"), "virtLauncher" (pod whose name starts with "virt-launcher"), "systemWorkloads" (pod with certain "superphenix.net/workloadClass" label values). If multiple conditions are provided, ALL must be true for a policy to be evaluated. If ANY is false, there is no match.</td>
		</tr>
		<tr>
			<td>policies.iaas-name-is-effective-id.validationActions</td>
			<td>string</td>
			<td><pre lang="json">
"[Deny, Audit]"
</pre>
</td>
			<td>Actions to take if the validation fails. Possible values are: Deny, Warn, Audit.</td>
		</tr>
	</tbody>
</table>
<h3>Network-Attachment-Definitions Config</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>

		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>policies.netattachdef-config</td>
			<td>object</td>
			<td><pre lang="">
"{}"
</pre>
</td>
			<td>Check that all Network Attachment Definitions have the correct config</td>
		</tr>
		<tr>
			<td>policies.netattachdef-config.debug</td>
			<td>bool</td>
			<td><pre lang="json">
false
</pre>
</td>
			<td>If debug is True, the VAP only matches resources having the "test-janna" label.</td>
		</tr>
		<tr>
			<td>policies.netattachdef-config.matchConditions</td>
			<td>string</td>
			<td><pre lang="json">
"[spxid, notGenerated]"
</pre>
</td>
			<td>Match conditions to target only specific resources. Possible values are: "spxid" (either orgID, projectID, resEffID, namespace or name are SPXIDs), "notGenerated" (doesn't have a "generated" label), "notSourcedVolSnap" (volume snapshot without labels starting with "snapshot.kubevirt.io/source-vm"), "notCsiDriver" (volume snapshot without a "csi-driver/cluster" label), "notTmpSnapshot" (volume snapshot without a name starting with "tmp-snapshot-*"), "virtLauncher" (pod whose name starts with "virt-launcher"), "systemWorkloads" (pod with certain "superphenix.net/workloadClass" label values). If multiple conditions are provided, ALL must be true for a policy to be evaluated. If ANY is false, there is no match.</td>
		</tr>
		<tr>
			<td>policies.netattachdef-config.validationActions</td>
			<td>string</td>
			<td><pre lang="json">
"[Deny, Audit]"
</pre>
</td>
			<td>Actions to take if the validation fails. Possible values are: Deny, Warn, Audit.</td>
		</tr>
	</tbody>
</table>
<h3>Network Policy MatchLabels</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>policies.netpol</td>
			<td>object</td>
			<td><pre lang="">
"{}"
</pre>
</td>
			<td>Check that all Network Policies have the correct fields</td>
		</tr>
		<tr>
			<td>policies.netpol.debug</td>
			<td>bool</td>
			<td><pre lang="json">
false
</pre>
</td>
			<td>If debug is True, the VAP only matches resources having the "test-janna" label.</td>
		</tr>
		<tr>

			<td>policies.netpol.matchConditions</td>
			<td>string</td>
			<td><pre lang="json">
"[spxid]"
</pre>
</td>
			<td>Match conditions to target only specific resources. Possible values are: "spxid" (either orgID, projectID, resEffID, namespace or name are SPXIDs), "notGenerated" (doesn't have a "generated" label), "notSourcedVolSnap" (volume snapshot without labels starting with "snapshot.kubevirt.io/source-vm"), "notCsiDriver" (volume snapshot without a "csi-driver/cluster" label), "notTmpSnapshot" (volume snapshot without a name starting with "tmp-snapshot-*"), "virtLauncher" (pod whose name starts with "virt-launcher"), "systemWorkloads" (pod with certain "superphenix.net/workloadClass" label values). If multiple conditions are provided, ALL must be true for a policy to be evaluated. If ANY is false, there is no match.</td>
		</tr>
		<tr>
			<td>policies.netpol.validationActions</td>
			<td>string</td>
			<td><pre lang="json">
"[Deny, Audit]"
</pre>
</td>
			<td>Actions to take if the validation fails. Possible values are: Deny, Warn, Audit.</td>
		</tr>
	</tbody>
</table>
<h3>Allowed/reserved CIDRs</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>policies.network-cidr</td>
			<td>object</td>
			<td><pre lang="">
"{}"
</pre>
</td>
			<td>Check that CIDRs used by network resources are allowed</td>
		</tr>
		<tr>
			<td>policies.network-cidr.debug</td>
			<td>bool</td>
			<td><pre lang="json">
false
</pre>
</td>
			<td>If debug is True, the VAP only matches resources having the "test-janna" label.</td>
		</tr>
		<tr>
			<td>policies.network-cidr.matchConditions</td>
			<td>string</td>
			<td><pre lang="json">
"[spxid]"
</pre>
</td>
			<td>Match conditions to target only specific resources. Possible values are: "spxid" (either orgID, projectID, resEffID, namespace or name are SPXIDs), "notGenerated" (doesn't have a "generated" label), "notSourcedVolSnap" (volume snapshot without labels starting with "snapshot.kubevirt.io/source-vm"), "notCsiDriver" (volume snapshot without a "csi-driver/cluster" label), "notTmpSnapshot" (volume snapshot without a name starting with "tmp-snapshot-*"), "virtLauncher" (pod whose name starts with "virt-launcher"), "systemWorkloads" (pod with certain "superphenix.net/workloadClass" label values). If multiple conditions are provided, ALL must be true for a policy to be evaluated. If ANY is false, there is no match.</td>
		</tr>
		<tr>
			<td>policies.network-cidr.validationActions</td>
			<td>string</td>
			<td><pre lang="json">
"[Deny, Audit]"
</pre>
</td>
			<td>Actions to take if the validation fails. Possible values are: Deny, Warn, Audit.</td>
		</tr>
		<tr>
			<td>policies.system-workloads</td>
			<td>object</td>
			<td><pre lang="">
"{}"
</pre>

</td>
			<td>Check that system workloads in customer namespaces have the correct fields</td>
		</tr>
	</tbody>
</table>
<h3>Labels (Org)</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>policies.org-labels</td>
			<td>object</td>
			<td><pre lang="">
"{}"
</pre>
</td>
			<td>Check that all organizational resources have the necessary labels with correct naming</td>
		</tr>
		<tr>
			<td>policies.org-labels.debug</td>
			<td>bool</td>
			<td><pre lang="json">
false
</pre>
</td>
			<td>If debug is True, the VAP only matches resources having the "test-janna" label.</td>
		</tr>
		<tr>
			<td>policies.org-labels.matchConditions</td>
			<td>string</td>
			<td><pre lang="json">
"[spxid]"
</pre>
</td>
			<td>Match conditions to target only specific resources. Possible values are: "spxid" (either orgID, projectID, resEffID, namespace or name are SPXIDs), "notGenerated" (doesn't have a "generated" label), "notSourcedVolSnap" (volume snapshot without labels starting with "snapshot.kubevirt.io/source-vm"), "notCsiDriver" (volume snapshot without a "csi-driver/cluster" label), "notTmpSnapshot" (volume snapshot without a name starting with "tmp-snapshot-*"), "virtLauncher" (pod whose name starts with "virt-launcher"), "systemWorkloads" (pod with certain "superphenix.net/workloadClass" label values). If multiple conditions are provided, ALL must be true for a policy to be evaluated. If ANY is false, there is no match.</td>
		</tr>
		<tr>
			<td>policies.org-labels.validationActions</td>
			<td>string</td>
			<td><pre lang="json">
"[Deny, Audit]"
</pre>
</td>
			<td>Actions to take if the validation fails. Possible values are: Deny, Warn, Audit.</td>
		</tr>
	</tbody>
</table>
<h3>Pod Annotations</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>policies.pod-annotations</td>
			<td>object</td>
			<td><pre lang="">
"{}"
</pre>
</td>
			<td>Check that all virt-launcher pods have the correct network annotations</td>
		</tr>

		<tr>
			<td>policies.pod-annotations.debug</td>
			<td>bool</td>
			<td><pre lang="json">
false
</pre>
</td>
			<td>If debug is True, the VAP only matches resources having the "test-janna" label.</td>
		</tr>
		<tr>
			<td>policies.pod-annotations.matchConditions</td>
			<td>string</td>
			<td><pre lang="json">
"[virtLauncher]"
</pre>
</td>
			<td>Match conditions to target only specific resources. Possible values are: "spxid" (either orgID, projectID, resEffID, namespace or name are SPXIDs), "notGenerated" (doesn't have a "generated" label), "notSourcedVolSnap" (volume snapshot without labels starting with "snapshot.kubevirt.io/source-vm"), "notCsiDriver" (volume snapshot without a "csi-driver/cluster" label), "notTmpSnapshot" (volume snapshot without a name starting with "tmp-snapshot-*"), "virtLauncher" (pod whose name starts with "virt-launcher"), "systemWorkloads" (pod with certain "superphenix.net/workloadClass" label values). If multiple conditions are provided, ALL must be true for a policy to be evaluated. If ANY is false, there is no match.</td>
		</tr>
		<tr>
			<td>policies.pod-annotations.validationActions</td>
			<td>string</td>
			<td><pre lang="json">
"[Deny, Audit]"
</pre>
</td>
			<td>Actions to take if the validation fails. Possible values are: Deny, Warn, Audit.</td>
		</tr>
	</tbody>
</table>
<h3>Reference is SPXID</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>policies.ref-is-spxid</td>
			<td>object</td>
			<td><pre lang="">
"{}"
</pre>
</td>
			<td>Check that all references to other resources are SPXIDs</td>
		</tr>
		<tr>
			<td>policies.ref-is-spxid.debug</td>
			<td>bool</td>
			<td><pre lang="json">
false
</pre>
</td>
			<td>If debug is True, the VAP only matches resources having the "test-janna" label.</td>
		</tr>
		<tr>
			<td>policies.ref-is-spxid.matchConditions</td>
			<td>string</td>
			<td><pre lang="json">
"[spxid, notGeneratedSubnet]"
</pre>
</td>
			<td>Match conditions to target only specific resources. Possible values are: "spxid" (either orgID, projectID, resEffID, namespace or name are SPXIDs), "notGenerated" (doesn't have a "generated" label), "notSourcedVolSnap" (volume snapshot without labels starting with "snapshot.kubevirt.io/source-vm"), "notCsiDriver" (volume snapshot without a "csi-driver/cluster" label), "notTmpSnapshot" (volume snapshot without a name starting with "tmp-snapshot-*"), "virtLauncher" (pod whose name starts with "virt-launcher"), "systemWorkloads" (pod with certain "superphenix.net/workloadClass" label values). If multiple conditions are provided, ALL must be true for a policy to be evaluated. If ANY is false, there is no match.</td>
		</tr>
		<tr>
			<td>policies.ref-is-spxid.validationActions</td>
			<td>string</td>
			<td><pre lang="json">
"[Deny, Audit]"

</pre>
</td>
			<td>Actions to take if the validation fails. Possible values are: Deny, Warn, Audit.</td>
		</tr>
	</tbody>
</table>
<h3>Restores (VM)</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>policies.restore</td>
			<td>object</td>
			<td><pre lang="">
"{}"
</pre>
</td>
			<td>Check that all VM Restores have correct fields</td>
		</tr>
		<tr>
			<td>policies.restore.debug</td>
			<td>bool</td>
			<td><pre lang="json">
false
</pre>
</td>
			<td>If debug is True, the VAP only matches resources having the "test-janna" label.</td>
		</tr>
		<tr>
			<td>policies.restore.matchConditions</td>
			<td>string</td>
			<td><pre lang="json">
"[spxid]"
</pre>
</td>
			<td>Match conditions to target only specific resources. Possible values are: "spxid" (either orgID, projectID, resEffID, namespace or name are SPXIDs), "notGenerated" (doesn't have a "generated" label), "notSourcedVolSnap" (volume snapshot without labels starting with "snapshot.kubevirt.io/source-vm"), "notCsiDriver" (volume snapshot without a "csi-driver/cluster" label), "notTmpSnapshot" (volume snapshot without a name starting with "tmp-snapshot-*"), "virtLauncher" (pod whose name starts with "virt-launcher"), "systemWorkloads" (pod with certain "superphenix.net/workloadClass" label values). If multiple conditions are provided, ALL must be true for a policy to be evaluated. If ANY is false, there is no match.</td>
		</tr>
		<tr>
			<td>policies.restore.validationActions</td>
			<td>string</td>
			<td><pre lang="json">
"[Deny, Audit]"
</pre>
</td>
			<td>Actions to take if the validation fails. Possible values are: Deny, Warn, Audit.</td>
		</tr>
	</tbody>
</table>
<h3>Snapshots (VM/Volume)</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>policies.snapshot</td>
			<td>object</td>
			<td><pre lang="">
"{}"
</pre>
</td>
			<td>Check that all VM/Volume Snapshots source names are SPXIDs</td>

		</tr>
		<tr>
			<td>policies.snapshot.debug</td>
			<td>bool</td>
			<td><pre lang="json">
false
</pre>
</td>
			<td>If debug is True, the VAP only matches resources having the "test-janna" label.</td>
		</tr>
		<tr>
			<td>policies.snapshot.matchConditions</td>
			<td>string</td>
			<td><pre lang="json">
"[spxid, notCsiDriver]"
</pre>
</td>
			<td>Match conditions to target only specific resources. Possible values are: "spxid" (either orgID, projectID, resEffID, namespace or name are SPXIDs), "notGenerated" (doesn't have a "generated" label), "notSourcedVolSnap" (volume snapshot without labels starting with "snapshot.kubevirt.io/source-vm"), "notCsiDriver" (volume snapshot without a "csi-driver/cluster" label), "notTmpSnapshot" (volume snapshot without a name starting with "tmp-snapshot-*"), "virtLauncher" (pod whose name starts with "virt-launcher"), "systemWorkloads" (pod with certain "superphenix.net/workloadClass" label values). If multiple conditions are provided, ALL must be true for a policy to be evaluated. If ANY is false, there is no match.</td>
		</tr>
		<tr>
			<td>policies.snapshot.validationActions</td>
			<td>string</td>
			<td><pre lang="json">
"[Deny, Audit]"
</pre>
</td>
			<td>Actions to take if the validation fails. Possible values are: Deny, Warn, Audit.</td>
		</tr>
	</tbody>
</table>
<h3>Subnet</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>policies.subnet</td>
			<td>object</td>
			<td><pre lang="">
"{}"
</pre>
</td>
			<td>Check that all Subnets have the correct fields</td>
		</tr>
		<tr>
			<td>policies.subnet.debug</td>
			<td>bool</td>
			<td><pre lang="json">
false
</pre>
</td>
			<td>If debug is True, the VAP only matches resources having the "test-janna" label.</td>
		</tr>
		<tr>
			<td>policies.subnet.matchConditions</td>
			<td>string</td>
			<td><pre lang="json">
"[spxid, notGenerated]"
</pre>
</td>
			<td>Match conditions to target only specific resources. Possible values are: "spxid" (either orgID, projectID, resEffID, namespace or name are SPXIDs), "notGenerated" (doesn't have a "generated" label), "notSourcedVolSnap" (volume snapshot without labels starting with "snapshot.kubevirt.io/source-vm"), "notCsiDriver" (volume snapshot without a "csi-driver/cluster" label), "notTmpSnapshot" (volume snapshot without a name starting with "tmp-snapshot-*"), "virtLauncher" (pod whose name starts with "virt-launcher"), "systemWorkloads" (pod with certain "superphenix.net/workloadClass" label values). If multiple conditions are provided, ALL must be true for a policy to be evaluated. If ANY is false, there is no match.</td>
		</tr>
		<tr>
			<td>policies.subnet.validationActions</td>
			<td>string</td>
			<td><pre lang="json">

"[Deny, Audit]"
</pre>
</td>
			<td>Actions to take if the validation fails. Possible values are: Deny, Warn, Audit.</td>
		</tr>
	</tbody>
</table>
<h3>System workloads</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>policies.system-workloads.debug</td>
			<td>bool</td>
			<td><pre lang="json">
false
</pre>
</td>
			<td>If debug is True, the VAP only matches resources having the "test-janna" label.</td>
		</tr>
		<tr>
			<td>policies.system-workloads.matchConditions</td>
			<td>string</td>
			<td><pre lang="json">
"[spxid, systemWorkloads]"
</pre>
</td>
			<td>Match conditions to target only specific resources. Possible values are: "spxid" (either orgID, projectID, resEffID, namespace or name are SPXIDs), "notGenerated" (doesn't have a "generated" label), "notSourcedVolSnap" (volume snapshot without labels starting with "snapshot.kubevirt.io/source-vm"), "notCsiDriver" (volume snapshot without a "csi-driver/cluster" label), "notTmpSnapshot" (volume snapshot without a name starting with "tmp-snapshot-*"), "virtLauncher" (pod whose name starts with "virt-launcher"), "systemWorkloads" (pod with certain "superphenix.net/workloadClass" label values). If multiple conditions are provided, ALL must be true for a policy to be evaluated. If ANY is false, there is no match.</td>
		</tr>
		<tr>
			<td>policies.system-workloads.validationActions</td>
			<td>string</td>
			<td><pre lang="json">
"[Deny, Audit]"
</pre>
</td>
			<td>Actions to take if the validation fails. Possible values are: Deny, Warn, Audit.</td>
		</tr>
	</tbody>
</table>
<h3>VM Credentials</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>policies.vm-credentials</td>
			<td>object</td>
			<td><pre lang="">
"{}"
</pre>
</td>
			<td>Check that all VirtualMachines' and VirtualMachineInstances' access credentials secret names are SPXIDs</td>
		</tr>
		<tr>
			<td>policies.vm-credentials.debug</td>
			<td>bool</td>
			<td><pre lang="json">
true
</pre>
</td>

			<td>If debug is True, the VAP only matches resources having the "test-janna" label.</td>
		</tr>
		<tr>
			<td>policies.vm-credentials.matchConditions</td>
			<td>string</td>
			<td><pre lang="json">
"[spxid]"
</pre>
</td>
			<td>Match conditions to target only specific resources. Possible values are: "spxid" (either orgID, projectID, resEffID, namespace or name are SPXIDs), "notGenerated" (doesn't have a "generated" label), "notSourcedVolSnap" (volume snapshot without labels starting with "snapshot.kubevirt.io/source-vm"), "notCsiDriver" (volume snapshot without a "csi-driver/cluster" label), "notTmpSnapshot" (volume snapshot without a name starting with "tmp-snapshot-*"), "virtLauncher" (pod whose name starts with "virt-launcher"), "systemWorkloads" (pod with certain "superphenix.net/workloadClass" label values). If multiple conditions are provided, ALL must be true for a policy to be evaluated. If ANY is false, there is no match.</td>
		</tr>
		<tr>
			<td>policies.vm-credentials.validationActions</td>
			<td>string</td>
			<td><pre lang="json">
"[Deny, Audit]"
</pre>
</td>
			<td>Actions to take if the validation fails. Possible values are: Deny, Warn, Audit.</td>
		</tr>
	</tbody>
</table>
<h3>VM Interfaces</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>policies.vm-interfaces</td>
			<td>object</td>
			<td><pre lang="">
"{}"
</pre>
</td>
			<td>Check that all VirtualMachines and VirtualMachineInstances use Bridge or Managedtap interfaces with the correct naming scheme</td>
		</tr>
		<tr>
			<td>policies.vm-interfaces.debug</td>
			<td>bool</td>
			<td><pre lang="json">
false
</pre>
</td>
			<td>If debug is True, the VAP only matches resources having the "test-janna" label.</td>
		</tr>
		<tr>
			<td>policies.vm-interfaces.matchConditions</td>
			<td>string</td>
			<td><pre lang="json">
"[spxid]"
</pre>
</td>
			<td>Match conditions to target only specific resources. Possible values are: "spxid" (either orgID, projectID, resEffID, namespace or name are SPXIDs), "notGenerated" (doesn't have a "generated" label), "notSourcedVolSnap" (volume snapshot without labels starting with "snapshot.kubevirt.io/source-vm"), "notCsiDriver" (volume snapshot without a "csi-driver/cluster" label), "notTmpSnapshot" (volume snapshot without a name starting with "tmp-snapshot-*"), "virtLauncher" (pod whose name starts with "virt-launcher"), "systemWorkloads" (pod with certain "superphenix.net/workloadClass" label values). If multiple conditions are provided, ALL must be true for a policy to be evaluated. If ANY is false, there is no match.</td>
		</tr>
		<tr>
			<td>policies.vm-interfaces.validationActions</td>
			<td>string</td>
			<td><pre lang="json">
"[Deny, Audit]"
</pre>
</td>
			<td>Actions to take if the validation fails. Possible values are: Deny, Warn, Audit.</td>
		</tr>
	</tbody>
</table>
<h3>VM Networks</h3>

<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>policies.vm-networks</td>
			<td>object</td>
			<td><pre lang="">
"{}"
</pre>
</td>
			<td>Check that all VirtualMachines and VirtualMachineInstances use the Multus network</td>
		</tr>
		<tr>
			<td>policies.vm-networks.debug</td>
			<td>bool</td>
			<td><pre lang="json">
true
</pre>
</td>
			<td>If debug is True, the VAP only matches resources having the "test-janna" label.</td>
		</tr>
		<tr>
			<td>policies.vm-networks.matchConditions</td>
			<td>string</td>
			<td><pre lang="json">
"[spxid]"
</pre>
</td>
			<td>Match conditions to target only specific resources. Possible values are: "spxid" (either orgID, projectID, resEffID, namespace or name are SPXIDs), "notGenerated" (doesn't have a "generated" label), "notSourcedVolSnap" (volume snapshot without labels starting with "snapshot.kubevirt.io/source-vm"), "notCsiDriver" (volume snapshot without a "csi-driver/cluster" label), "notTmpSnapshot" (volume snapshot without a name starting with "tmp-snapshot-*"), "virtLauncher" (pod whose name starts with "virt-launcher"), "systemWorkloads" (pod with certain "superphenix.net/workloadClass" label values). If multiple conditions are provided, ALL must be true for a policy to be evaluated. If ANY is false, there is no match.</td>
		</tr>
		<tr>
			<td>policies.vm-networks.validationActions</td>
			<td>string</td>
			<td><pre lang="json">
"[Deny, Audit]"
</pre>
</td>
			<td>Actions to take if the validation fails. Possible values are: Deny, Warn, Audit.</td>
		</tr>
	</tbody>
</table>
<h3>VM-VMI Same Labels</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>policies.vm-vmi-same-labels</td>
			<td>object</td>
			<td><pre lang="">
"{}"
</pre>
</td>
			<td>Check that all VirtualMachines and their templated VirtualMachineInstance share the same labels</td>
		</tr>
		<tr>
			<td>policies.vm-vmi-same-labels.debug</td>
			<td>bool</td>
			<td><pre lang="json">
false
</pre>

</td>
			<td>If debug is True, the VAP only matches resources having the "test-janna" label.</td>
		</tr>
		<tr>
			<td>policies.vm-vmi-same-labels.matchConditions</td>
			<td>string</td>
			<td><pre lang="json">
"[spxid, notNabok]"
</pre>
</td>
			<td>Match conditions to target only specific resources. Possible values are: "spxid" (either orgID, projectID, resEffID, namespace or name are SPXIDs), "notGenerated" (doesn't have a "generated" label), "notSourcedVolSnap" (volume snapshot without labels starting with "snapshot.kubevirt.io/source-vm"), "notCsiDriver" (volume snapshot without a "csi-driver/cluster" label), "notTmpSnapshot" (volume snapshot without a name starting with "tmp-snapshot-*"), "virtLauncher" (pod whose name starts with "virt-launcher"), "systemWorkloads" (pod with certain "superphenix.net/workloadClass" label values). If multiple conditions are provided, ALL must be true for a policy to be evaluated. If ANY is false, there is no match.</td>
		</tr>
		<tr>
			<td>policies.vm-vmi-same-labels.validationActions</td>
			<td>string</td>
			<td><pre lang="json">
"[Deny, Audit]"
</pre>
</td>
			<td>Actions to take if the validation fails. Possible values are: Deny, Warn, Audit.</td>
		</tr>
	</tbody>
</table>
<h3>VM Volumes</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>policies.vm-volumes</td>
			<td>object</td>
			<td><pre lang="">
"{}"
</pre>
</td>
			<td>Check that all VirtualMachines and VirtualMachineInstances use PVC (at least 1) and/or CloudInitNoCloud volumes, and that the PVC name and claimName are SPXIDs</td>
		</tr>
		<tr>
			<td>policies.vm-volumes.debug</td>
			<td>bool</td>
			<td><pre lang="json">
true
</pre>
</td>
			<td>If debug is True, the VAP only matches resources having the "test-janna" label.</td>
		</tr>
		<tr>
			<td>policies.vm-volumes.matchConditions</td>
			<td>string</td>
			<td><pre lang="json">
"[spxid]"
</pre>
</td>
			<td>Match conditions to target only specific resources. Possible values are: "spxid" (either orgID, projectID, resEffID, namespace or name are SPXIDs), "notGenerated" (doesn't have a "generated" label), "notSourcedVolSnap" (volume snapshot without labels starting with "snapshot.kubevirt.io/source-vm"), "notCsiDriver" (volume snapshot without a "csi-driver/cluster" label), "notTmpSnapshot" (volume snapshot without a name starting with "tmp-snapshot-*"), "virtLauncher" (pod whose name starts with "virt-launcher"), "systemWorkloads" (pod with certain "superphenix.net/workloadClass" label values). If multiple conditions are provided, ALL must be true for a policy to be evaluated. If ANY is false, there is no match.</td>
		</tr>
		<tr>
			<td>policies.vm-volumes.validationActions</td>
			<td>string</td>
			<td><pre lang="json">
"[Deny, Audit]"
</pre>
</td>
			<td>Actions to take if the validation fails. Possible values are: Deny, Warn, Audit.</td>
		</tr>
	</tbody>
</table>

<h3>VMI Annotations</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>policies.vmi-annotations</td>
			<td>object</td>
			<td><pre lang="">
"{}"
</pre>
</td>
			<td>Check that all VirtualMachines templates and VirtualMachineInstances have the correct annotations</td>
		</tr>
		<tr>
			<td>policies.vmi-annotations.debug</td>
			<td>bool</td>
			<td><pre lang="json">
false
</pre>
</td>
			<td>If debug is True, the VAP only matches resources having the "test-janna" label.</td>
		</tr>
		<tr>
			<td>policies.vmi-annotations.matchConditions</td>
			<td>string</td>
			<td><pre lang="json">
"[spxid, notNabok]"
</pre>
</td>
			<td>Match conditions to target only specific resources. Possible values are: "spxid" (either orgID, projectID, resEffID, namespace or name are SPXIDs), "notGenerated" (doesn't have a "generated" label), "notSourcedVolSnap" (volume snapshot without labels starting with "snapshot.kubevirt.io/source-vm"), "notCsiDriver" (volume snapshot without a "csi-driver/cluster" label), "notTmpSnapshot" (volume snapshot without a name starting with "tmp-snapshot-*"), "virtLauncher" (pod whose name starts with "virt-launcher"), "systemWorkloads" (pod with certain "superphenix.net/workloadClass" label values). If multiple conditions are provided, ALL must be true for a policy to be evaluated. If ANY is false, there is no match.</td>
		</tr>
		<tr>
			<td>policies.vmi-annotations.validationActions</td>
			<td>string</td>
			<td><pre lang="json">
"[Deny, Audit]"
</pre>
</td>
			<td>Actions to take if the validation fails. Possible values are: Deny, Warn, Audit.</td>
		</tr>
	</tbody>
</table>
<h3>VPC</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>policies.vpc</td>
			<td>object</td>
			<td><pre lang="">
"{}"
</pre>
</td>
			<td>Check that all VPCs have the correct fields</td>
		</tr>
		<tr>
			<td>policies.vpc.debug</td>
			<td>bool</td>
			<td><pre lang="json">
false

</pre>
</td>
			<td>If debug is True, the VAP only matches resources having the "test-janna" label.</td>
		</tr>
		<tr>
			<td>policies.vpc.matchConditions</td>
			<td>string</td>
			<td><pre lang="json">
"[spxid]"
</pre>
</td>
			<td>Match conditions to target only specific resources. Possible values are: "spxid" (either orgID, projectID, resEffID, namespace or name are SPXIDs), "notGenerated" (doesn't have a "generated" label), "notSourcedVolSnap" (volume snapshot without labels starting with "snapshot.kubevirt.io/source-vm"), "notCsiDriver" (volume snapshot without a "csi-driver/cluster" label), "notTmpSnapshot" (volume snapshot without a name starting with "tmp-snapshot-*"), "virtLauncher" (pod whose name starts with "virt-launcher"), "systemWorkloads" (pod with certain "superphenix.net/workloadClass" label values). If multiple conditions are provided, ALL must be true for a policy to be evaluated. If ANY is false, there is no match.</td>
		</tr>
		<tr>
			<td>policies.vpc.validationActions</td>
			<td>string</td>
			<td><pre lang="json">
"[Deny, Audit]"
</pre>
</td>
			<td>Actions to take if the validation fails. Possible values are: Deny, Warn, Audit.</td>
		</tr>
	</tbody>
</table>

