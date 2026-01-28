{{/*
Expand the name of the chart.
*/}}
{{- define "recac.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "recac.fullname" -}}
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
{{- define "recac.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "recac.labels" -}}
helm.sh/chart: {{ include "recac.chart" . }}
{{ include "recac.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "recac.selectorLabels" -}}
app.kubernetes.io/name: {{ include "recac.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "recac.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "recac.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Determine default model based on provider if not explicitly set
*/}}
{{- define "recac.defaultModel" -}}
{{- if .Values.config.model }}
{{- .Values.config.model }}
{{- else }}
{{- if eq .Values.config.provider "openrouter" }}
{{- "mistralai/devstral-2512" }}
{{- else if eq .Values.config.provider "gemini" }}
{{- "gemini-pro" }}
{{- else if eq .Values.config.provider "openai" }}
{{- "gpt-4" }}
{{- else if eq .Values.config.provider "opencode" }}
{{- "claude-3-5-sonnet" }}
{{- else if eq .Values.config.provider "cursor" }}
{{- "claude-3-5-sonnet" }}
{{- else }}
{{- "gemini-pro" }}
{{- end }}
{{- end }}
{{- end }}
