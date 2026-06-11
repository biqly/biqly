{{- define "worker.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "worker.fullname" -}}
{{- printf "%s-%s" .Release.Name (include "worker.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "worker.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" -}}
{{- end -}}

{{- define "worker.selectorLabels" -}}
app.kubernetes.io/name: {{ include "worker.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: worker
app.kubernetes.io/part-of: biqly
{{- end -}}

{{- define "worker.labels" -}}
helm.sh/chart: {{ include "worker.chart" . }}
{{ include "worker.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
{{- end -}}

{{- define "worker.serviceAccountName" -}}
{{- default "biqly" .Values.global.serviceAccount.name -}}
{{- end -}}

{{- define "worker.image" -}}
{{- $registry := trimSuffix "/" .Values.global.biqlyImageRegistry -}}
{{- if or (contains "/" .Values.image.repository) (not $registry) -}}
{{- printf "%s:%s" .Values.image.repository .Values.image.tag -}}
{{- else -}}
{{- printf "%s/%s:%s" $registry .Values.image.repository .Values.image.tag -}}
{{- end -}}
{{- end -}}

{{- define "worker.configChecksum" -}}
{{- include (print $.Template.BasePath "/configmap.yaml") . | sha256sum -}}
{{- end -}}

{{- define "worker.secretChecksum" -}}
{{- printf "%s-%s-%s-%s-%s-%s" .Values.global.secretNames.db .Values.global.secretNames.security .Values.global.secretNames.authSecret (default "" .Values.global.secrets.BI_METADATA_DB_DSN) (default "" .Values.global.secrets.BI_ENCRYPTION_KEY) (default "" .Values.global.secrets.BI_AUTH_INTERNAL_TOKEN) | sha256sum -}}
{{- end -}}
