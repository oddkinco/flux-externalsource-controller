{{/*
Expand the name of the chart.
*/}}
{{- define "fx-controller.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "fx-controller.fullname" -}}
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
{{- define "fx-controller.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "fx-controller.labels" -}}
helm.sh/chart: {{ include "fx-controller.chart" . }}
{{ include "fx-controller.selectorLabels" . }}
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
{{- define "fx-controller.selectorLabels" -}}
app.kubernetes.io/name: {{ include "fx-controller.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "fx-controller.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (printf "%s-manager" (include "fx-controller.fullname" .)) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the cluster role to use
*/}}
{{- define "fx-controller.clusterRoleName" -}}
{{- printf "%s-manager-role" (include "fx-controller.fullname" .) }}
{{- end }}

{{/*
Create the name of the cluster role binding to use
*/}}
{{- define "fx-controller.clusterRoleBindingName" -}}
{{- printf "%s-manager-rolebinding" (include "fx-controller.fullname" .) }}
{{- end }}

{{/*
Create the name of the leader election role to use
*/}}
{{- define "fx-controller.leaderElectionRoleName" -}}
{{- printf "%s-leader-election-role" (include "fx-controller.fullname" .) }}
{{- end }}

{{/*
Create the name of the leader election role binding to use
*/}}
{{- define "fx-controller.leaderElectionRoleBindingName" -}}
{{- printf "%s-leader-election-rolebinding" (include "fx-controller.fullname" .) }}
{{- end }}

{{/*
Create the name of the metrics service to use
*/}}
{{- define "fx-controller.metricsServiceName" -}}
{{- printf "%s-metrics" (include "fx-controller.fullname" .) }}
{{- end }}

{{/*
Create the name of the webhook service to use
*/}}
{{- define "fx-controller.webhookServiceName" -}}
{{- printf "%s-webhook-service" (include "fx-controller.fullname" .) }}
{{- end }}

{{/*
Create the name of the webhook certificate to use
*/}}
{{- define "fx-controller.webhookCertificateName" -}}
{{- printf "%s-serving-cert" (include "fx-controller.fullname" .) }}
{{- end }}

{{/*
Create the name of the webhook issuer to use
*/}}
{{- define "fx-controller.webhookIssuerName" -}}
{{- printf "%s-selfsigned-issuer" (include "fx-controller.fullname" .) }}
{{- end }}