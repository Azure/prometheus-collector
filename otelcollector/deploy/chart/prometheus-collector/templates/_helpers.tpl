{{/*
Expand the name of the chart.
*/}}
{{- define "prometheus-collector.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "prometheus-collector.fullname" -}}
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
{{- define "prometheus-collector.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "prometheus-collector.labels" -}}
helm.sh/chart: {{ include "prometheus-collector.chart" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Get node-exporter fullname 
*/}}
{{- define "prometheus-collector.nodeexporterfullname" -}}
{{- $name := "prometheus-node-exporter" -}}
{{- $releasename := .Release.Name | toString }}
{{- if contains $name $releasename -}}
{{- $releasename | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" $releasename $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{/*
Get kube-state-metrics fullname 
*/}}
{{- define "prometheus-collector.kubestatemetricsfullname" -}}
{{- $name := "kube-state-metrics" -}}
{{- $releasename := .Release.Name | toString }}
{{- if contains $name $releasename -}}
{{- $releasename | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" $releasename $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{/*
Validate namespace for MAC mode
*/}}
{{- define "mac-namespace-validate" -}}
  {{ $namespace := .Release.Namespace }}
  {{- if eq $namespace "kube-system" -}}
  namespace: {{ $namespace }}
  {{- end -}}
{{- end -}}
