{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "chart.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "chart.fullname" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common labels
*/}}
{{- define "chart.labels" -}}
app.kubernetes.io/name: {{ include "chart.name" . }}
helm.sh/chart: {{ include "chart.fullname" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
cluster.x-k8s.io/provider: runtime-extensions-nutanix
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/*
Selector labels
*/}}
{{- define "chart.selectorLabels" -}}
app.kubernetes.io/name: {{ include "chart.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Certificate issuer name
*/}}
{{- define "chart.issuerName" -}}
{{- if .Values.certificates.issuer.selfSigned -}}
{{- if .Values.certificates.issuer.name -}}
{{ .Values.certificates.issuer.name }}
{{- else -}}
{{ template "chart.name" . }}-issuer
{{- end -}}
{{- else -}}
{{ required "A valid .Values.certificates.issuer.name is required!" .Values.certificates.issuer.name }}
{{- end -}}
{{- end -}}

{{/*
  Resolve Helm addon repository URL: override (e.g. oci://harbor...) > internal OCI repo > default HTTPS.
  Input: dict with "addonKey" (ConfigMap key, e.g. nutanix-storage-csi), "defaultURL" (default HTTPS URL), "context" (root .)
*/}}
{{- define "caren.helmAddonRepoURL" -}}
{{- $ctx := .context }}
{{- $overrides := default dict $ctx.Values.helmAddonsOverrides }}
{{- $override := (and (hasKey $overrides .addonKey) (index $overrides .addonKey) (index (index $overrides .addonKey) "repositoryURL")) | default "" }}
{{- if $override -}}
{{ $override }}
{{- else if $ctx.Values.helmRepository.enabled -}}
oci://helm-repository.{{ $ctx.Release.Namespace }}.svc/charts
{{- else -}}
{{ .defaultURL }}
{{- end -}}
{{- end }}
