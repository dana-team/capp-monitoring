{{- define "benchmarks.labels" -}}
app.kubernetes.io/name: benchmarks
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "benchmarks.envVars" -}}
- name: TARGET_URL
  value: {{ .Values.targetUrl | quote }}
- name: KSVC_NAME
  value: {{ .Values.knativeService | quote }}
- name: KSVC_NAMESPACE
  value: {{ .Values.knativeNamespace | quote }}
{{- if .Values.prometheus.remoteWriteUrl }}
- name: K6_PROMETHEUS_RW_SERVER_URL
  value: {{ .Values.prometheus.remoteWriteUrl | quote }}
{{- end }}
{{- if .Values.pushgatewayUrl }}
- name: PUSHGATEWAY_URL
  value: {{ .Values.pushgatewayUrl | quote }}
{{- end }}
{{- end }}
