{{- define "mcp.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "mcp.fullname" -}}
{{- printf "%s-%s" .Release.Name (include "mcp.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "mcp.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" -}}
{{- end -}}

{{- define "mcp.selectorLabels" -}}
app.kubernetes.io/name: {{ include "mcp.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: mcp
app.kubernetes.io/part-of: biqly
{{- end -}}

{{- define "mcp.labels" -}}
helm.sh/chart: {{ include "mcp.chart" . }}
{{ include "mcp.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
{{- end -}}

{{- define "mcp.serviceAccountName" -}}
{{- default "biqly" .Values.global.serviceAccount.name -}}
{{- end -}}

{{- define "mcp.image" -}}
{{- $registry := trimSuffix "/" .Values.global.biqlyImageRegistry -}}
{{- if or (contains "/" .Values.image.repository) (not $registry) -}}
{{- printf "%s:%s" .Values.image.repository .Values.image.tag -}}
{{- else -}}
{{- printf "%s/%s:%s" $registry .Values.image.repository .Values.image.tag -}}
{{- end -}}
{{- end -}}

{{- define "mcp.configChecksum" -}}
{{- include (print $.Template.BasePath "/configmap.yaml") . | sha256sum -}}
{{- end -}}
