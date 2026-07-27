{{/*
Expand the name of the chart.
*/}}
{{- define "policies.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "policies.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "policies.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "policies.labels" -}}
helm.sh/chart: {{ include "policies.chart" . }}
{{ include "policies.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.Version | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "policies.selectorLabels" -}}
app.kubernetes.io/name: {{ include "policies.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "policies.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "policies.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
------------------------------------------
-----------------Labels-------------------
------------------------------------------
*/}}

{{/*
Generated label
*/}}
{{- define "policies.generatedLabel" -}}
"superphenix.net/generated"
{{- end }}

{{/*
GitOps label
*/}}
{{- define "policies.gitopsLabel" -}}
"superphenix.net/gitops"
{{- end }}

{{/*
OrganizationID label
*/}}
{{- define "policies.organizationIDLabel" -}}
"superphenix.net/organizationID"
{{- end }}

{{/*
ProjectID (namespace) label
*/}}
{{- define "policies.projectIDLabel" -}}
"superphenix.net/projectID"
{{- end }}

{{/*
Resource EffectiveID label
*/}}
{{- define "policies.resEffectiveIDLabel" -}}
"superphenix.net/resourceEffectiveID"
{{- end }}

{{/*
Resource LocalID label
*/}}
{{- define "policies.resLocalIDLabel" -}}
"superphenix.net/resourceLocalID"
{{- end }}

{{/*
Organizational labels
*/}}
{{- define "policies.orgLabels" -}}
{{- if eq . "spxid" -}}
["superphenix.net/organizationID", "superphenix.net/projectID"]
{{- else if eq . "name" -}}
["superphenix.net/organizationName", "superphenix.net/projectName"]
{{- end }}
{{- end }}

{{/*
IaaS labels
*/}}
{{- define "policies.iaasLabels" -}}
{{- if eq . "spxid" -}}
["superphenix.net/organizationID", "superphenix.net/projectID", "superphenix.net/resourceEffectiveID"]
{{- else if eq . "localid" -}}
"superphenix.net/resourceLocalID"
{{- else if eq . "name" -}}
["superphenix.net/organizationName", "superphenix.net/projectName", "superphenix.net/resourceName"]
{{- end }}
{{- end }}

{{/*
Nabok label
*/}}
{{- define "policies.nabokLabel" -}}
"plan-name"
{{- end }}

{{/*
Label for test resources missing a required label on purpose
*/}}
{{- define "policies.testMissingLabel" -}}
"testMissingLabel"
{{- end }}

{{/*
Regex for label values and other fields
*/}}
{{- define "policies.regex" -}}
{{- if eq . "spxid" -}}
"^spx-[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$"
{{- else if eq . "name" -}}
"^([a-zA-Z0-9]([a-zA-Z0-9-_.]{0,61}[a-zA-Z0-9])?)?$"
{{- else if eq . "interfaceName" -}}
"^interface-[0-9]+$"
{{- else if eq . "snapshot" -}}
"^snapshot-[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$"
{{- else if eq . "bootdisk" -}}
"^spx-[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}(\-[a-zA-Z0-9]+)*-boot-disk$"
{{- else if eq . "vmsnapshotVolume" -}}
"^vmsnapshot-[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}-volume-spx-[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$"
{{- else if eq . "etcddisk" -}}
"^data(-etcd)?-spx-(([a-z0-9-]*-datastore[0-9][0-9]-([a-z0-9]{5}|[0-9]))|[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}-[a-z0-9]{5})$"
{{- else if eq . "nabokMigration" -}}
"^migration-[a-zA-Z0-9/-]{0,53}$"

{{- end }}
{{- end }}

{{/*
------------------------------------------
----Ignored labels (VM/VMI template)------
------------------------------------------
*/}}

{{/*
Label prefixes that are ignored when checking for same labels in VM and VMI template
*/}}
{{- define "policies.ignoredLabels" -}}
["velero.io/"]
{{- end }}

{{/*
------------------------------------------
------------Match conditions--------------
------------------------------------------
*/}}

{{/*
Do not match resources with a "generated" label
*/}}
{{- define "policies.mcNotGenerated" -}}
{{- $generatedLabel := include "policies.generatedLabel" . -}}
# Do not match resources with a "generated" label
- name: matchNotGenerated
  expression: |
    has(object.metadata.labels) &&
    !({{ $generatedLabel }} in object.metadata.labels)
{{- end }}

{{/*
Do not match Subnets with a "generated" label
*/}}
{{- define "policies.mcNotGeneratedSubnet" -}}
{{- $generatedLabel := include "policies.generatedLabel" . -}}
# Do not match Subnets with a "generated" label
- name: matchNotGeneratedSubnet
  expression: |
    object.kind != "Subnet" ||
    has(object.metadata.labels) &&
    !({{ $generatedLabel }} in object.metadata.labels)
{{- end }}

{{/*
Do not match Volume Snapshots with labels starting with "snapshot.kubevirt.io/source-vm"
*/}}
{{- define "policies.mcNotSourcedVolSnap" -}}
# Do not match Volume Snapshots with labels starting with "snapshot.kubevirt.io/source-vm"
- name: matchNotSourcedVolSnap
  expression: |
    object.kind != "VolumeSnapshot" ||
    has(object.metadata.labels) &&
    !object.metadata.labels.exists(entry,
      entry.startsWith("snapshot.kubevirt.io/source-vm")
    )
{{- end }}

{{/*
Do not match Volume Snapshots with a "csi-driver/cluster" label
*/}}
{{- define "policies.mcNotCsiDriver" -}}
# Do not match Volume Snapshots with a "csi-driver/cluster" label
- name: matchNotCsiDriver
  expression: |
    object.kind != "VolumeSnapshot" ||
    has(object.metadata.labels) &&
    !("csi-driver/cluster" in object.metadata.labels)
{{- end }}

{{/*
Do not match Volume Snapshots with a name starting with "tmp-snapshot-*"
*/}}
{{- define "policies.mcNotTmpSnapshot" -}}
# Do not match Volume Snapshots with a name starting with "tmp-snapshot-*"
- name: matchNotTmpSnapshot
  expression: |
    object.kind != "VolumeSnapshot" ||
    !object.metadata.name.startsWith("tmp-snapshot-")
{{- end }}

{{/*
Do not match Volume Snapshots with a name starting with "velero-*"
*/}}
{{- define "policies.mcNotVeleroVolsnap" -}}
# Do not match Volume Snapshots with a name starting with "velero-*"
- name: matchNotVeleroVolsnap
  expression: |
    object.kind != "VolumeSnapshot" ||
    !object.metadata.name.startsWith("velero-")
{{- end }}

{{/*
Do not match resources related to Nabok
*/}}
{{- define "policies.mcNotNabok" -}}
{{- $nabokLabel := include "policies.nabokLabel" . -}}
{{- $nabokMigration := include "policies.regex" "nabokMigration" -}}
# Do not match resources related to Nabok
- name: matchNotNabok
  expression: |
    has(object.metadata.labels) &&
    !(
      {{ $nabokLabel }} in object.metadata.labels &&
      object.metadata.labels[{{ $nabokLabel }}].matches({{ $nabokMigration }})
    )
{{- end }}

{{/*
Match resources whose name starts with "virt-launcher"
*/}}
{{- define "policies.mcVirtLauncher" -}}
# Match resources whose name starts with "virt-launcher"
- name: matchVirtLauncher
  expression: |
    object.metadata.name
      .startsWith("virt-launcher")
{{- end }}

{{/*
Match system workloads in customer namespaces
*/}}
{{- define "policies.mcSystemWorkloads" -}}
# Match system workloads in customer namespaces
- name: matchSystemWorkloads
  expression: |
    has(object.metadata.labels) &&
    "superphenix.net/workloadClass" in object.metadata.labels &&
    object.metadata.labels["superphenix.net/workloadClass"] in ["kaas-tenant-api-server", "kaas-kubevirt-csi", "kaas-essentials-job", "datavolume-importer"]
{{- end }}

{{/*
Match resources with SPXID labels/namespaces/names
*/}}
{{- define "policies.mcSpxid" -}}
{{- $spxid := include "policies.regex" "spxid" -}}
{{- $orgIDLabel := include "policies.organizationIDLabel" . -}}
{{- $projectIDLabel := include "policies.projectIDLabel" . -}}
{{- $resEffectiveIDLabel := include "policies.resEffectiveIDLabel" . -}}
# Match resources that have (either) an SPXID organizationID label, projectID label, resourceEffectiveID label, namespace, or name.
- name: matchSpxid
  expression: |
    (
      has(object.metadata.labels) &&
      (
        (
          ({{ $orgIDLabel }} in object.metadata.labels) &&
          object.metadata.labels[{{ $orgIDLabel }}]
          .matches({{ $spxid }})
        ) ||
        (
          ({{ $projectIDLabel }} in object.metadata.labels) &&
          object.metadata.labels[{{ $projectIDLabel }}]
          .matches({{ $spxid }})
        ) ||
        (
          ({{ $resEffectiveIDLabel }} in object.metadata.labels) &&
          object.metadata.labels[{{ $resEffectiveIDLabel }}]
          .matches({{ $spxid }})
        )
      )
    ) ||
    (
      has(object.metadata.namespace) &&
      object.metadata.namespace
      .matches({{ $spxid }})
    ) ||
    object.metadata.name
    .matches({{ $spxid }})
{{- end }}

{{/*
Combine match conditions
*/}}
{{- define "policies.matchConditions" -}}
{{- $mc := (.matchConditions | fromYamlArray ) -}}
matchConditions:
  {{- if has "notGenerated" $mc -}}
  {{- include "policies.mcNotGenerated" . | nindent 2 -}}
  {{- end }}
  {{- if has "notGeneratedSubnet" $mc -}}
  {{- include "policies.mcNotGeneratedSubnet" . | nindent 2 -}}
  {{- end }}
  {{- if has "notSourcedVolSnap" $mc -}}
  {{- include "policies.mcNotSourcedVolSnap" . | nindent 2 -}}
  {{- end }}
  {{- if has "notCsiDriver" $mc -}}
  {{- include "policies.mcNotCsiDriver" . | nindent 2 -}}
  {{- end }}
  {{- if has "notTmpSnapshot" $mc -}}
  {{- include "policies.mcNotTmpSnapshot" . | nindent 2 -}}
  {{- end }}
  {{- if has "notVeleroVolsnap" $mc -}}
  {{- include "policies.mcNotVeleroVolsnap" . | nindent 2 -}}
  {{- end }}
  {{- if has "notNabok" $mc -}}
  {{- include "policies.mcNotNabok" . | nindent 2 -}}
  {{- end }}
  {{- if has "virtLauncher" $mc -}}
  {{- include "policies.mcVirtLauncher" . | nindent 2 -}}
  {{- end }}
  {{- if has "systemWorkloads" $mc -}}
  {{- include "policies.mcSystemWorkloads" . | nindent 2 -}}
  {{- end }}
  {{- if has "spxid" $mc -}}
  {{- include "policies.mcSpxid" . | nindent 2 -}}
  {{- end }}
{{- end }}

{{/*
------------------------------------------
------------------Debug-------------------
------------------------------------------
*/}}

{{/*
Debug matching
*/}}
{{- define "policies.debug" -}}
{{- if .debug }}
matchResources:
  matchPolicy: Equivalent
  namespaceSelector: {}
  objectSelector:
    matchExpressions:
      - key: test-janna
        operator: Exists
{{- end }}
{{- end }}

{{/*
------------------------------------------
---------------Annotations----------------
------------------------------------------
*/}}

{{- define "policies.annotations" -}}

{{/*
Required annotations for VMs
*/}}
{{- if eq . "required" -}}
["kubevirt.io/allow-pod-bridge-network-live-migration", "ovn.kubernetes.io/allow_live_migration"]

{{/*
Annotations with key : subnetID.namespace.annotation
*/}}
{{- else if eq . "keySubnetIDNamespace" -}}
["ovn.kubernetes.io/allow_live_migration", "ovn.kubernetes.io/ip_address"]

{{/*
Annotations with value : true
*/}}
{{- else if eq . "valueTrue" -}}
["kubevirt.io/allow-pod-bridge-network-live-migration", "ovn.kubernetes.io/allow_live_migration"]
{{- end }}
{{- end }}

{{/*
------------------------------------------
--------------Allowed CIDRs---------------
------------------------------------------
*/}}

{{/*
List of CIDRs allowed within customer-owned subnets
*/}}
{{- define "policies.allowedCidrs" -}}
["10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "224.0.0.0/4", "240.0.0.0/4", "fc00::/7", "64:ff9b:1::/48"]
{{- end }}

{{/*
List of reserved (unavailable) CIDRs
*/}}
{{- define "policies.reservedCidrs" -}}
["198.18.0.0/16"]
{{- end }}
