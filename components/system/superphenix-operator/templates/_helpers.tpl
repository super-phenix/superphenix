{{/*
Expand the name of the chart.
*/}}
{{- define "superphenix-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "superphenix-operator.fullname" -}}
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
{{- define "superphenix-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "superphenix-operator.labels" -}}
helm.sh/chart: {{ include "superphenix-operator.chart" . }}
{{ include "superphenix-operator.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "superphenix-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "superphenix-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "superphenix-operator.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "superphenix-operator.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Get the health probe bind address.
*/}}
{{- define "superphenix-operator.healthProbeBindAddress" -}}
{{- if .Values.installOnClusterWithoutCNI -}}
{{- printf ":28765" -}}
{{- else -}}
{{- .Values.health.probeBindAddress -}}
{{- end -}}
{{- end -}}

{{/*
Get the health probe port.
*/}}
{{- define "superphenix-operator.healthProbePort" -}}
{{- $address := include "superphenix-operator.healthProbeBindAddress" . -}}
{{- (split ":" $address)._1 | default "8081" -}}
{{- end -}}
