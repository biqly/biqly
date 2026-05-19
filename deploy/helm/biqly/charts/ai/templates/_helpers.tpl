{{- define "ai.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "ai.fullname" -}}
{{- printf "%s-%s" .Release.Name (include "ai.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "ai.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" -}}
{{- end -}}

{{- define "ai.selectorLabels" -}}
app.kubernetes.io/name: {{ include "ai.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: ai
app.kubernetes.io/part-of: biqly
{{- end -}}

{{- define "ai.labels" -}}
helm.sh/chart: {{ include "ai.chart" . }}
{{ include "ai.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
{{- end -}}

{{- define "ai.serviceAccountName" -}}
{{- default "biqly" .Values.global.serviceAccount.name -}}
{{- end -}}

{{- define "ai.image" -}}
{{- $registry := trimSuffix "/" .Values.global.biqlyImageRegistry -}}
{{- if $registry -}}
{{- printf "%s/%s:%s" $registry .Values.image.repository .Values.image.tag -}}
{{- else -}}
{{- printf "%s:%s" .Values.image.repository .Values.image.tag -}}
{{- end -}}
{{- end -}}

{{- define "ai.configChecksum" -}}
{{- include (print $.Template.BasePath "/configmap.yaml") . | sha256sum -}}
{{- end -}}

{{- define "ai.secretChecksum" -}}
{{- if .Values.global.secrets.createSecrets -}}
{{- include (print $.Template.BasePath "/secret.yaml") . | sha256sum -}}
{{- else -}}
{{- printf "%s-%s-%s" .Values.global.secretNames.db .Values.global.secretNames.security .Values.global.secretNames.ai | sha256sum -}}
{{- end -}}
{{- end -}}
