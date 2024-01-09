{{/*
Expand the name of the chart.
*/}}
{{- define "excat.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "excat.fullname" -}}
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
{{- define "excat.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "excat.labels" -}}
helm.sh/chart: {{ include "excat.chart" . }}
{{ include "excat.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Template labels
*/}}
{{- define "excat.templateLabels" -}}
app.kubernetes.io/name: {{ include "excat.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- if .Values.selectorLabelsOverride }}
{{ toYaml .Values.selectorLabelsOverride }}
{{- end }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "excat.selectorLabels" -}}
app.kubernetes.io/name: {{ include "excat.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "excat-admission.serviceAccountName" -}}
{{- if .Values.admission.serviceAccount.create }}
{{- default (include "excat.fullname" .) .Values.admission.serviceAccount.name }}-admission
{{- else }}
{{- default "default" .Values.admission.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "excat.devicePlugin.serviceAccountName" -}}
{{- if .Values.devicePlugin.serviceAccount.create }}
{{- default (include "excat.fullname" .) .Values.devicePlugin.serviceAccount.name }}-deviceplugin
{{- else }}
{{- default "default" .Values.devicePlugin.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the tls secret to use
*/}}
{{- define "excatadmission.tlsSecretName" -}}
{{- if .Values.admission.tlsSecret.create }}
{{- default (include "excat.fullname" .) .Values.admission.tlsSecret.name }}-admission
{{- else }}
{{- default "default" .Values.admission.tlsSecret.name }}
{{- end }}
{{- end }}