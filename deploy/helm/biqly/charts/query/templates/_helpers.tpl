{{- define "query.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "query.fullname" -}}
{{- printf "%s-%s" .Release.Name (include "query.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "query.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" -}}
{{- end -}}

{{- define "query.selectorLabels" -}}
app.kubernetes.io/name: {{ include "query.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: query
app.kubernetes.io/part-of: biqly
{{- end -}}

{{- define "query.labels" -}}
helm.sh/chart: {{ include "query.chart" . }}
{{ include "query.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
{{- end -}}

{{- define "query.serviceAccountName" -}}
{{- default "biqly" .Values.global.serviceAccount.name -}}
{{- end -}}

{{- define "query.image" -}}
{{- $registry := trimSuffix "/" .Values.global.biqlyImageRegistry -}}
{{- if $registry -}}
{{- printf "%s/%s:%s" $registry .Values.image.repository .Values.image.tag -}}
{{- else -}}
{{- printf "%s:%s" .Values.image.repository .Values.image.tag -}}
{{- end -}}
{{- end -}}

{{- define "query.configChecksum" -}}
{{- include (print $.Template.BasePath "/configmap.yaml") . | sha256sum -}}
{{- end -}}

{{- define "query.secretChecksum" -}}
{{- include (print $.Template.BasePath "/secret.yaml") . | sha256sum -}}
{{- end -}}
