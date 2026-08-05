{{/*
Expand the name of the chart.
*/}}
{{- define "superphenix-system.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "superphenix-system.fullname" -}}
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
{{- define "superphenix-system.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "superphenix-system.labels" -}}
helm.sh/chart: {{ include "superphenix-system.chart" . }}
{{ include "superphenix-system.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "superphenix-system.selectorLabels" -}}
app.kubernetes.io/name: {{ include "superphenix-system.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "superphenix-system.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "superphenix-system.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
The effective mode is the concatenation of deploymentTopology and type.
*/}}
{{- define "superphenix-system.effectiveMode" -}}
{{- printf "%s%s" .Values.cluster.deploymentTopology .Values.cluster.type -}}
{{- end -}}

{{/*
Determine if an application should be deployed.
Usage: {{ include "superphenix-system.shouldDeploy" (list $appValues $) }}
*/}}
{{- define "superphenix-system.shouldDeploy" -}}
{{- $app := index . 0 -}}
{{- $root := index . 1 -}}
{{- $effectiveMode := include "superphenix-system.effectiveMode" $root -}}
{{- if and (eq $app.enabled true) (eq $root.Values.disableAll false) -}}
  {{- if or (not $app.modes) (has $effectiveMode (default (list) $app.modes)) -}}
    true
  {{- end -}}
{{- end -}}
{{- end -}}
