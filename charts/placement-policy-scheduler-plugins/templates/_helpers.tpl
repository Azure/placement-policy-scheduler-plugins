{{/*
Expand the name of the chart.
*/}}
{{- define "placement-policy-scheduler-plugins.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "placement-policy-scheduler-plugins.fullname" -}}
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
{{- define "placement-policy-scheduler-plugins.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "placement-policy-scheduler-plugins.labels" -}}
helm.sh/chart: {{ include "placement-policy-scheduler-plugins.chart" . }}
{{ include "placement-policy-scheduler-plugins.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "placement-policy-scheduler-plugins.selectorLabels" -}}
app.kubernetes.io/name: {{ include "placement-policy-scheduler-plugins.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
component: scheduler
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "placement-policy-scheduler-plugins.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "placement-policy-scheduler-plugins.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}
