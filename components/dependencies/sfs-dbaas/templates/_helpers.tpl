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

{{/*
CloudNative-PG services exposed for a database, comma-separated: rw, ro with more than one
instance, and r
*/}}
{{- define "sfs-dbaas.roles" -}}
{{- $roles := list "rw" }}
{{- if gt (int (.instances | default 1)) 1 }}
{{- $roles = append $roles "ro" }}
{{- end }}
{{- $roles = append $roles "r" }}
{{- join "," $roles }}
{{- end }}

{{/*
Memory of an instance in MiB, from a Gi or Mi quantity
*/}}
{{- define "sfs-dbaas.memoryMi" -}}
{{- $memory := toString . }}
{{- if hasSuffix "Gi" $memory }}
{{- mul (trimSuffix "Gi" $memory | int) 1024 }}
{{- else if hasSuffix "Mi" $memory }}
{{- trimSuffix "Mi" $memory | int }}
{{- else }}
{{- fail (printf "Database memory %s must be in Gi or Mi" $memory) }}
{{- end }}
{{- end }}

{{/*
HAProxy configuration forwarding each proxy port to its CloudNative-PG service
*/}}
{{- define "sfs-dbaas.proxyConfig" -}}
{{- $ := .root }}
{{- $dbID := .dbID }}
{{- $defaults := $.Values.databaseDefaults -}}
global
  log stdout format raw local0
  # Unset, HAProxy derives it from the fd limit and preallocates for it
  maxconn {{ mul (int .maxConnections) (len .roles) }}
defaults
  mode tcp
  log global
  timeout connect 5s
  timeout client 1h
  timeout server 1h
resolvers k8s
  parse-resolv-conf
  hold valid 10s
{{- range $role := .roles }}
frontend fe_{{ $role }}
  bind :{{ get $defaults.proxy.ports $role }}
  default_backend be_{{ $role }}
backend be_{{ $role }}
  server {{ $role }} {{ printf "%s-%s.%s.svc.%s" $dbID $role $.Release.Namespace $defaults.clusterDomain }}:{{ $defaults.port }} resolvers k8s resolve-prefer ipv6 init-addr none check
{{- end }}
{{- end }}
