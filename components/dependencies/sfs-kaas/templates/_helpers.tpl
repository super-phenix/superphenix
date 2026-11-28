{{/*
Expand the name of the chart.
*/}}
{{- define "sfs-kaas.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "sfs-kaas.fullname" -}}
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
{{- define "sfs-kaas.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "sfs-kaas.labels" -}}
helm.sh/chart: {{ include "sfs-kaas.chart" . }}
{{ include "sfs-kaas.selectorLabels" . }}
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
{{- define "sfs-kaas.selectorLabels" -}}
app.kubernetes.io/name: {{ include "sfs-kaas.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Returns the FQDN of a cluster
*/}}
{{/*
Validate a user-provided control plane FQDN (DNS name, lowercase).
*/}}
{{- define "sfs-kaas.validFqdn" -}}
{{- $v := toString . -}}
{{- if not (regexMatch `^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*$` $v) -}}
{{- fail (printf "invalid controlPlane.network.fqdn %q: must be a lowercase DNS name" $v) -}}
{{- end -}}
{{- $v -}}
{{- end }}

{{/*
Validate a cluster name (letters, digits, ".", "_" and "-", up to 63 characters).
*/}}
{{- define "sfs-kaas.validName" -}}
{{- $v := toString . -}}
{{- if not (regexMatch "^[a-zA-Z0-9]([a-zA-Z0-9._-]{0,61}[a-zA-Z0-9])?$" $v) -}}
{{- fail (printf "invalid cluster name %q: only letters, digits, '.', '_' and '-' are allowed" $v) -}}
{{- end -}}
{{- $v -}}
{{- end }}

{{/*
Validate a chart or Kubernetes version (semantic version, optional "v" prefix).
*/}}
{{- define "sfs-kaas.validVersion" -}}
{{- $v := toString . -}}
{{- if not (regexMatch `^v?[0-9]+\.[0-9]+\.[0-9]+([-+][0-9A-Za-z.-]+)?$` $v) -}}
{{- fail (printf "invalid version %q: must be a semantic version" $v) -}}
{{- end -}}
{{- $v -}}
{{- end }}

{{/*
Control plane FQDN of a cluster: the validated user-provided one, or the AZ default.
Expects a dict with "root", "cluster" and "clusterID".
*/}}
{{- define "sfs-kaas.controlPlaneFqdn" -}}
{{- $userFqdn := ((.cluster.controlPlane).network).fqdn -}}
{{- if $userFqdn -}}
{{- include "sfs-kaas.validFqdn" $userFqdn -}}
{{- else -}}
{{- include "sfs-kaas.fqdn" (dict "root" .root "name" .clusterID) -}}
{{- end -}}
{{- end }}

{{/*
Validate a chart reference used as a helm argument: an oci:// or https:// URL.
A value starting with "-" would otherwise be read by helm as an option.
*/}}
{{- define "sfs-kaas.validChartUrl" -}}
{{- $v := toString . -}}
{{- if not (regexMatch `^(oci|https)://[A-Za-z0-9.-]+(:[0-9]{1,5})?(/[A-Za-z0-9._~/-]*)?$` $v) -}}
{{- fail (printf "invalid kaas-essentials chart URL %q: must be an oci:// or https:// URL" $v) -}}
{{- end -}}
{{- $v -}}
{{- end }}

{{- define "sfs-kaas.fqdn" -}}
{{- $ := .root }}
{{- $baseUrl := (get $.Values.azDomains $.Values.location | required "Missing value for this AZ under `.azDomains`").external | required "Missing `external` key for this AZ under `.azDomains.<AZ>`" }}
{{- printf "%s" (regexReplaceAll "%s" $baseUrl .name) }}
{{- end }}

{{/*
Returns the SPX effective ID of a resource
*/}}
{{- define "sfs-kaas.spxEID" -}}
{{- printf "spx-%s" (include "sfs-kaas.getUUIDv5" (dict "NS" .project "NAME" .localID)) }}
{{- end }}

{{/*
Generate UUIDv5 through external templating
*/}}
{{- define "sfs-kaas.getUUIDv5" -}}
{{- printf "<spx-uuidv5 %s %s>" .NS .NAME }}
{{- end }}
