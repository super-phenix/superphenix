{{/*
Expand the name of the chart.
*/}}
{{- define "sfs-gitops.name" -}}
{{- default .Chart.Name .Values.project.name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "sfs-gitops.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "sfs-gitops.labels" -}}
helm.sh/chart: {{ include "sfs-gitops.chart" . }}
{{ include "sfs-gitops.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "sfs-gitops.selectorLabels" -}}
app.kubernetes.io/name: {{ include "sfs-gitops.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Helper to include "self-service-projects.labels" as requested in the snippet.
*/}}
{{- define "self-service-projects.labels" -}}
{{ include "sfs-gitops.labels" . }}
{{- end }}

