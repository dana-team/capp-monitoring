{{- define "benchmarks.labels" -}}
app.kubernetes.io/name: benchmarks
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "benchmarks.envVars" -}}
{{- $ns := .Values.knativeNamespace -}}
{{- $ksvc := .Values.knativeService -}}
{{- $target := .Values.targetUrl -}}
{{- if .Values.testCapp.enabled -}}
  {{- $ns = .Values.testCapp.namespace | default .Release.Namespace -}}
  {{- if not $ksvc }}{{- $ksvc = .Values.testCapp.name -}}{{- end -}}
  {{- if not $target }}{{- $target = printf "http://%s.%s.svc.cluster.local" .Values.testCapp.name $ns -}}{{- end -}}
{{- end -}}
- name: TARGET_URL
  value: {{ $target | quote }}
- name: KSVC_NAME
  value: {{ $ksvc | quote }}
- name: KSVC_NAMESPACE
  value: {{ $ns | quote }}
{{- $urls := .Values.victoriametrics.importUrls | default (list) }}
{{- if and (empty $urls) .Values.victoriametrics.importUrl }}
{{- $urls = list .Values.victoriametrics.importUrl }}
{{- end }}
{{- if $urls }}
- name: VM_IMPORT_URLS
  value: {{ $urls | join "," | quote }}
{{- end }}
{{- if .Values.prometheus.remoteWriteUrl }}
- name: K6_PROMETHEUS_RW_SERVER_URL
  value: {{ .Values.prometheus.remoteWriteUrl | quote }}
{{- end }}
{{- if .Values.pushgatewayUrl }}
- name: PUSHGATEWAY_URL
  value: {{ .Values.pushgatewayUrl | quote }}
{{- end }}
- name: TARGET_PODS
  value: {{ .Values.scale.targetPods | quote }}
- name: TIMEOUT_SECONDS
  value: {{ .Values.scale.timeoutSeconds | quote }}
- name: LOAD_VUS
  value: {{ .Values.scale.loadVus | quote }}
- name: LOAD_DURATION
  value: {{ .Values.scale.loadDuration | quote }}
- name: LOAD_PATH
  value: {{ .Values.testCapp.loadPath | quote }}
- name: ZERO_WAIT_SECONDS
  value: {{ .Values.scale.zeroWaitSeconds | quote }}
{{- end }}
