{{/*
Expand the name of the chart.
*/}}
{{- define "sfs-baas.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "sfs-baas.fullname" -}}
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
{{- define "sfs-baas.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "sfs-baas.labels" -}}
helm.sh/chart: {{ include "sfs-baas.chart" . }}
{{ include "sfs-baas.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.Version | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
superphenix.net/gitops: {{ .Values.gitops | quote }}
superphenix.net/organizationID: spx-{{ .Values.organizationID }}
superphenix.net/projectID: spx-{{ .Values.projectID }}
{{- if .Values.gitops }}
superphenix.net/organizationName: {{ .Values.organizationName | quote }}
superphenix.net/projectName: {{ .Values.projectName | quote }}
{{- end }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "sfs-baas.selectorLabels" -}}
app.kubernetes.io/name: {{ include "sfs-baas.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "sfs-baas.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "sfs-baas.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}


{{/*
Returns the SPX effective ID of a resource
*/}}
{{- define "sfs-baas.spxEID" -}}
{{- printf "spx-%s" (include "sfs-baas.getUUIDv5" (dict "NS" .projectID "NAME" .localID)) }}
{{- end }}

{{/*
Generate UUIDv5 through external templating
*/}}
{{- define "sfs-baas.getUUIDv5" -}}
{{- printf "<spx-uuidv5 %s %s>" .NS .NAME }}
{{- end }}

{{/*
Resources to backup, grouped by type of resource
*/}}

{{- define "sfs-baas.storageResourceGlobal" }}
- persistentvolume
{{- end }}

{{- define "sfs-baas.storageResourceNamespaced" }}
- datavolume.cdi.kubevirt.io
- persistentvolumeclaim
{{- end }}

{{- define "sfs-baas.vmResourceNamespaced" }}
- virtualmachine.kubevirt.io
- controllerrevision.apps
- secret
{{- include "sfs-baas.storageResourceNamespaced" . }}
{{- end }}

{{- define "sfs-baas.vmResourceGlobal" }}
{{- include "sfs-baas.storageResourceGlobal" . }}
{{- end }}

{{- define "sfs-baas.networkResourceGlobal" }}
- vpc.kubeovn.io
- vpc-nat-gateway.kubeovn.io
- subnet.kubeovn.io
- iptables-eip.kubeovn.io
- iptables-fip-rule.kubeovn.io
- iptables-snat-rule.kubeovn.io
- iptables-dnat-rule.kubeovn.io
- switch-lb-rule.kubeovn.io
{{- end }}

{{- define "sfs-baas.networkResourceNamespaced" }}
- network-attachment-definition.k8s.cni.cncf.io
- networkpolicy.networking.k8s.io
{{- end }}

{{- define "sfs-baas.kaasResourceGlobal" }}
- datastore.kamaji.clastix.io
- clusterrole.rbac.authorization.k8s.io
- clusterrolebinding.rbac.authorization.k8s.io
- mutatingadmissionpolicy.admissionregistration.k8s.io
- mutatingadmissionpolicybinding.admissionregistration.k8s.io
{{- end }}

{{- define "sfs-baas.kaasResourceNamespaced" }}
- configmap
- deployment.apps
- cluster.cluster.x-k8s.io
- kubevirtcluster.infrastructure.cluster.x-k8s.io
- kamajicontrolplane.controlplane.cluster.x-k8s.io
- gateway.gateway.networking.k8s.io
- tlsroute.gateway.networking.k8s.io
- certificate.cert-manager.io
- etcdcluster.etcd-operator.cozystack.io
- etcdmember.etcd-operator.cozystack.io
- issuer.cert-manager.io
- serviceaccount
- role.rbac.authorization.k8s.io
- rolebinding.rbac.authorization.k8s.io
- kubeadmconfigtemplate.bootstrap.cluster.x-k8s.io
- kubevirtmachinetemplate.infrastructure.cluster.x-k8s.io
- kubeadmconfig.bootstrap.cluster.x-k8s.io
- kubevirtmachine.infrastructure.cluster.x-k8s.io
- machinedeployment.cluster.x-k8s.io
- machineset.cluster.x-k8s.io
- machine.cluster.x-k8s.io
{{- end }}

{{/*
Resources to backup for each backup type
*/}}

{{- define "sfs-baas.clusterResources" -}}
{{- if eq .backupType "all" }}
{{- include "sfs-baas.vmResourceGlobal" . }}
{{- include "sfs-baas.networkResourceGlobal" . }}
{{- include "sfs-baas.storageResourceGlobal" . }}
{{- include "sfs-baas.kaasResourceGlobal" . }}
{{- else if eq .backupType "vm" }}
{{- include "sfs-baas.vmResourceGlobal" . }}
{{- end }}
- namespace
{{- end }}

{{- define "sfs-baas.namespaceResources" -}}
{{- if eq .backupType "all" }}
{{- include "sfs-baas.vmResourceNamespaced" . }}
{{- include "sfs-baas.networkResourceNamespaced" . }}
{{- include "sfs-baas.storageResourceNamespaced" . }}
{{- include "sfs-baas.kaasResourceNamespaced" . }}
{{- else if eq .backupType "vm" }}
{{- include "sfs-baas.vmResourceNamespaced" . }}
{{- end }}
{{- end }}