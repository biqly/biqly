{{- define "frontend.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "frontend.fullname" -}}
{{- printf "%s-%s" .Release.Name (include "frontend.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "frontend.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" -}}
{{- end -}}

{{- define "frontend.selectorLabels" -}}
app.kubernetes.io/name: {{ include "frontend.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: frontend
app.kubernetes.io/part-of: biqly
{{- end -}}

{{- define "frontend.labels" -}}
helm.sh/chart: {{ include "frontend.chart" . }}
{{ include "frontend.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
{{- end -}}

{{- define "frontend.serviceAccountName" -}}
{{- default "biqly" .Values.global.serviceAccount.name -}}
{{- end -}}

{{- define "frontend.image" -}}
{{- $registry := trimSuffix "/" .Values.global.biqlyImageRegistry -}}
{{- if or (contains "/" .Values.image.repository) (not $registry) -}}
{{- printf "%s:%s" .Values.image.repository .Values.image.tag -}}
{{- else -}}
{{- printf "%s/%s:%s" $registry .Values.image.repository .Values.image.tag -}}
{{- end -}}
{{- end -}}

{{- define "frontend.configChecksum" -}}
{{- include (print $.Template.BasePath "/configmap.yaml") . | sha256sum -}}
{{- end -}}
