{{/*
Expand the name of the chart.
*/}}
{{- define "flux-externalsource-controller.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "flux-externalsource-controller.fullname" -}}
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
{{- define "flux-externalsource-controller.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "flux-externalsource-controller.labels" -}}
helm.sh/chart: {{ include "flux-externalsource-controller.chart" . }}
{{ include "flux-externalsource-controller.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- with .Values.commonLabels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "flux-externalsource-controller.selectorLabels" -}}
app.kubernetes.io/name: {{ include "flux-externalsource-controller.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "flux-externalsource-controller.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (printf "%s-manager" (include "flux-externalsource-controller.fullname" .)) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the cluster role to use
*/}}
{{- define "flux-externalsource-controller.clusterRoleName" -}}
{{- printf "%s-manager-role" (include "flux-externalsource-controller.fullname" .) }}
{{- end }}

{{/*
Create the name of the cluster role binding to use
*/}}
{{- define "flux-externalsource-controller.clusterRoleBindingName" -}}
{{- printf "%s-manager-rolebinding" (include "flux-externalsource-controller.fullname" .) }}
{{- end }}

{{/*
Create the name of the leader election role to use
*/}}
{{- define "flux-externalsource-controller.leaderElectionRoleName" -}}
{{- printf "%s-leader-election-role" (include "flux-externalsource-controller.fullname" .) }}
{{- end }}

{{/*
Create the name of the leader election role binding to use
*/}}
{{- define "flux-externalsource-controller.leaderElectionRoleBindingName" -}}
{{- printf "%s-leader-election-rolebinding" (include "flux-externalsource-controller.fullname" .) }}
{{- end }}

{{/*
Create the name of the metrics service to use
*/}}
{{- define "flux-externalsource-controller.metricsServiceName" -}}
{{- printf "%s-metrics" (include "flux-externalsource-controller.fullname" .) }}
{{- end }}

{{/*
Create the name of the webhook service to use
*/}}
{{- define "flux-externalsource-controller.webhookServiceName" -}}
{{- printf "%s-webhook-service" (include "flux-externalsource-controller.fullname" .) }}
{{- end }}

{{/*
Create the name of the webhook certificate to use
*/}}
{{- define "flux-externalsource-controller.webhookCertificateName" -}}
{{- printf "%s-serving-cert" (include "flux-externalsource-controller.fullname" .) }}
{{- end }}

{{/*
Create the name of the webhook issuer to use
*/}}
{{- define "flux-externalsource-controller.webhookIssuerName" -}}
{{- printf "%s-selfsigned-issuer" (include "flux-externalsource-controller.fullname" .) }}
{{- end }}