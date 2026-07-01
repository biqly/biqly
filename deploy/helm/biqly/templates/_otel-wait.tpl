{{/*
  Blocks app startup until the in-cluster OTLP collector accepts TCP connections,
  avoiding the deploy-time race where apps export before Service endpoints exist.
*/}}
{{- define "biqly.waitForOtelCollectorInitContainer" -}}
{{- $tracing := .Values.global.observability.tracing -}}
{{- if and $tracing.enabled $tracing.useInClusterCollector (not $tracing.skipCollectorWait) }}
{{- $ep := required "global.observability.tracing.collectorEndpoint" $tracing.collectorEndpoint -}}
{{- $hostPort := trimPrefix "http://" (trimPrefix "https://" $ep) -}}
{{- $parts := splitList ":" $hostPort -}}
- name: wait-for-otel-collector
  image: busybox:1.37
  command: ["sh", "-c"]
  args:
    - |
      until nc -z {{ index $parts 0 }} {{ default "4318" (index $parts 1) }} 2>/dev/null; do
        echo "waiting for OTLP collector at {{ index $parts 0 }}:{{ default "4318" (index $parts 1) }}..."
        sleep 1
      done
  resources:
    requests:
      cpu: 10m
      memory: 16Mi
    limits:
      cpu: 50m
      memory: 32Mi
  securityContext:
    allowPrivilegeEscalation: false
    readOnlyRootFilesystem: true
    runAsNonRoot: true
    runAsUser: 65532
    runAsGroup: 65532
    capabilities:
      drop:
        - ALL
    seccompProfile:
      type: RuntimeDefault
{{- end }}
{{- end }}
