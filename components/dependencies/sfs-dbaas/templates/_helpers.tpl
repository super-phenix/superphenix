{{/*
Expand the name of the chart.
*/}}
{{- define "sfs-dbaas.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "sfs-dbaas.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "sfs-dbaas.labels" -}}
helm.sh/chart: {{ include "sfs-dbaas.chart" . }}
{{ include "sfs-dbaas.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.Version | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{ include "sfs-dbaas.spxLabels" . }}
{{- end }}

{{/*
SPX labels
*/}}
{{- define "sfs-dbaas.spxLabels" -}}
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
{{- define "sfs-dbaas.selectorLabels" -}}
app.kubernetes.io/name: {{ include "sfs-dbaas.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Returns the SPX effective ID of a resource
*/}}
{{- define "sfs-dbaas.spxEID" -}}
{{- printf "spx-%s" (include "sfs-dbaas.getUUIDv5" (dict "NS" .project "NAME" .localID)) }}
{{- end }}

{{/*
Generate UUIDv5 through external templating
*/}}
{{- define "sfs-dbaas.getUUIDv5" -}}
{{- printf "<spx-uuidv5 %s %s>" .NS .NAME }}
{{- end }}
