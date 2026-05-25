{{- define "auth.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "auth.fullname" -}}
{{- printf "%s-%s" .Release.Name (include "auth.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "auth.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" -}}
{{- end -}}

{{- define "auth.selectorLabels" -}}
app.kubernetes.io/name: {{ include "auth.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: auth
app.kubernetes.io/part-of: biqly
{{- end -}}

{{- define "auth.labels" -}}
helm.sh/chart: {{ include "auth.chart" . }}
{{ include "auth.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
{{- end -}}

{{- define "auth.serviceAccountName" -}}
{{- default "biqly" .Values.global.serviceAccount.name -}}
{{- end -}}

{{- define "auth.image" -}}
{{- $registry := trimSuffix "/" .Values.global.biqlyImageRegistry -}}
{{- if $registry -}}
{{- printf "%s/%s:%s" $registry .Values.image.repository .Values.image.tag -}}
{{- else -}}
{{- printf "%s:%s" .Values.image.repository .Values.image.tag -}}
{{- end -}}
{{- end -}}

{{- define "auth.configChecksum" -}}
{{- include (print $.Template.BasePath "/configmap.yaml") . | sha256sum -}}
{{- end -}}

{{- define "auth.secretChecksum" -}}
{{- if .Values.global.secrets.createSecrets -}}
{{- include (print $.Template.BasePath "/secret.yaml") . | sha256sum -}}
{{- else -}}
{{- printf "%s-%s" .Values.global.secretNames.authDB .Values.global.secretNames.authSecret | sha256sum -}}
{{- end -}}
{{- end -}}
