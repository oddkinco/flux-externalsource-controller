{{/*
Expand the name of the chart.
*/}}
{{- define "flux-external-controller.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "flux-external-controller.fullname" -}}
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
{{- define "flux-external-controller.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "flux-external-controller.labels" -}}
helm.sh/chart: {{ include "flux-external-controller.chart" . }}
{{ include "flux-external-controller.selectorLabels" . }}
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
{{- define "flux-external-controller.selectorLabels" -}}
app.kubernetes.io/name: {{ include "flux-external-controller.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "flux-external-controller.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (printf "%s-manager" (include "flux-external-controller.fullname" .)) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the cluster role to use
*/}}
{{- define "flux-external-controller.clusterRoleName" -}}
{{- printf "%s-manager-role" (include "flux-external-controller.fullname" .) }}
{{- end }}

{{/*
Create the name of the cluster role binding to use
*/}}
{{- define "flux-external-controller.clusterRoleBindingName" -}}
{{- printf "%s-manager-rolebinding" (include "flux-external-controller.fullname" .) }}
{{- end }}

{{/*
Create the name of the leader election role to use
*/}}
{{- define "flux-external-controller.leaderElectionRoleName" -}}
{{- printf "%s-leader-election-role" (include "flux-external-controller.fullname" .) }}
{{- end }}

{{/*
Create the name of the leader election role binding to use
*/}}
{{- define "flux-external-controller.leaderElectionRoleBindingName" -}}
{{- printf "%s-leader-election-rolebinding" (include "flux-external-controller.fullname" .) }}
{{- end }}

{{/*
Create the name of the metrics service to use
*/}}
{{- define "flux-external-controller.metricsServiceName" -}}
{{- printf "%s-metrics" (include "flux-external-controller.fullname" .) }}
{{- end }}

{{/*
Create the name of the webhook service to use
*/}}
{{- define "flux-external-controller.webhookServiceName" -}}
{{- printf "%s-webhook-service" (include "flux-external-controller.fullname" .) }}
{{- end }}

{{/*
Create the name of the webhook certificate to use
*/}}
{{- define "flux-external-controller.webhookCertificateName" -}}
{{- printf "%s-serving-cert" (include "flux-external-controller.fullname" .) }}
{{- end }}

{{/*
Create the name of the webhook issuer to use
*/}}
{{- define "flux-external-controller.webhookIssuerName" -}}
{{- printf "%s-selfsigned-issuer" (include "flux-external-controller.fullname" .) }}
{{- end }}