{{- define "catalog.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "catalog.fullname" -}}
{{- printf "%s-%s" .Release.Name (include "catalog.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "catalog.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" -}}
{{- end -}}

{{- define "catalog.selectorLabels" -}}
app.kubernetes.io/name: {{ include "catalog.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: catalog
app.kubernetes.io/part-of: biqly
{{- end -}}

{{- define "catalog.labels" -}}
helm.sh/chart: {{ include "catalog.chart" . }}
{{ include "catalog.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
{{- end -}}

{{- define "catalog.serviceAccountName" -}}
{{- default "biqly" .Values.global.serviceAccount.name -}}
{{- end -}}

{{- define "catalog.image" -}}
{{- $registry := trimSuffix "/" .Values.global.biqlyImageRegistry -}}
{{- if or (contains "/" .Values.image.repository) (not $registry) -}}
{{- printf "%s:%s" .Values.image.repository .Values.image.tag -}}
{{- else -}}
{{- printf "%s/%s:%s" $registry .Values.image.repository .Values.image.tag -}}
{{- end -}}
{{- end -}}

{{- define "catalog.configChecksum" -}}
{{- include (print $.Template.BasePath "/configmap.yaml") . | sha256sum -}}
{{- end -}}

{{- define "catalog.secretChecksum" -}}
{{- if .Values.global.secrets.createSecrets -}}
{{- include (print $.Template.BasePath "/secret.yaml") . | sha256sum -}}
{{- else -}}
{{- printf "%s-%s" .Values.global.secretNames.db .Values.global.secretNames.security | sha256sum -}}
{{- end -}}
{{- end -}}
