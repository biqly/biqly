{{/*
  OTLP export env for workloads that call SetupTracing (auth, mail; api/worker
  when deployed). Headers come from a Secret — never from ConfigMap.
*/}}
{{- define "biqly.otelEnv" -}}
{{- $tracing := .Values.global.observability.tracing -}}
{{- if $tracing.enabled }}
- name: OTEL_SERVICE_NAME
  value: {{ $tracing.serviceName | default "biqly" | quote }}
{{- if $tracing.useInClusterCollector }}
- name: OTEL_EXPORTER_OTLP_ENDPOINT
  value: {{ $tracing.collectorEndpoint | required "tracing.collectorEndpoint is required when useInClusterCollector is true" | quote }}
{{- else }}
- name: OTEL_EXPORTER_OTLP_ENDPOINT
  value: {{ $tracing.otlpEndpoint | required "global.observability.tracing.otlpEndpoint is required when tracing.enabled" | quote }}
{{- end }}
- name: OTEL_TRACES_SAMPLER
  value: {{ $tracing.sampler | default "parentbased_traceidratio" | quote }}
- name: OTEL_TRACES_SAMPLER_ARG
  value: {{ $tracing.samplerArg | default "0.25" | quote }}
{{- /* When the gateway is on, the bearer token lives only on the collector. */}}
{{- if and (not $tracing.useInClusterCollector) $tracing.otlpHeadersSecret }}
{{- with $tracing.otlpHeadersSecret }}
- name: OTEL_EXPORTER_OTLP_HEADERS
  valueFrom:
    secretKeyRef:
      name: {{ .name | required "otlpHeadersSecret.name" | quote }}
      key: {{ .key | default "OTEL_EXPORTER_OTLP_HEADERS" | quote }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}

{{/* Headers only — for pods that already load OTEL_* from biqly-config. */}}
{{- define "biqly.otelHeadersEnv" -}}
{{- $tracing := .Values.global.observability.tracing -}}
{{- /* Skipped when useInClusterCollector: apps export to the in-cluster
       collector unauthenticated; the token stays on the collector only. */}}
{{- if and $tracing.enabled (not $tracing.useInClusterCollector) $tracing.otlpHeadersSecret }}
- name: OTEL_EXPORTER_OTLP_HEADERS
  valueFrom:
    secretKeyRef:
      name: {{ $tracing.otlpHeadersSecret.name | quote }}
      key: {{ $tracing.otlpHeadersSecret.key | default "OTEL_EXPORTER_OTLP_HEADERS" | quote }}
{{- end }}
{{- end }}
