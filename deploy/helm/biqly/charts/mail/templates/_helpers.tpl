{{- define "mail.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "mail.fullname" -}}
{{- printf "%s-%s" .Release.Name (include "mail.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "mail.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" -}}
{{- end -}}

{{- define "mail.selectorLabels" -}}
app.kubernetes.io/name: {{ include "mail.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: mail
app.kubernetes.io/part-of: biqly
{{- end -}}

{{- define "mail.labels" -}}
helm.sh/chart: {{ include "mail.chart" . }}
{{ include "mail.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
{{- end -}}

{{- define "mail.serviceAccountName" -}}
{{- default "biqly" .Values.global.serviceAccount.name -}}
{{- end -}}

{{- define "mail.image" -}}
{{- $registry := trimSuffix "/" .Values.global.biqlyImageRegistry -}}
{{- if $registry -}}
{{- printf "%s/%s:%s" $registry .Values.image.repository .Values.image.tag -}}
{{- else -}}
{{- printf "%s:%s" .Values.image.repository .Values.image.tag -}}
{{- end -}}
{{- end -}}

{{- define "mail.migrateImage" -}}
{{- $registry := trimSuffix "/" .Values.global.biqlyImageRegistry -}}
{{- $repo := default .Values.image.repository .Values.migrate.image.repository -}}
{{- $tag := default .Values.image.tag .Values.migrate.image.tag -}}
{{- if $registry -}}
{{- printf "%s/%s:%s" $registry $repo $tag -}}
{{- else -}}
{{- printf "%s:%s" $repo $tag -}}
{{- end -}}
{{- end -}}

{{- define "mail.configChecksum" -}}
{{- include (print $.Template.BasePath "/configmap.yaml") . | sha256sum -}}
{{- end -}}

{{- define "mail.secretChecksum" -}}
{{- if .Values.global.secrets.createSecrets -}}
{{- include (print $.Template.BasePath "/secret.yaml") . | sha256sum -}}
{{- else -}}
{{- printf "%s-%s" .Values.global.secretNames.mailDB .Values.global.secretNames.mailSecret | sha256sum -}}
{{- end -}}
{{- end -}}
