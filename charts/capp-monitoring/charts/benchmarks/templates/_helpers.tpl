{{- define "benchmarks.labels" -}}
app.kubernetes.io/name: benchmarks
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "benchmarks.envVars" -}}
{{- $ns := .Values.knativeNamespace -}}
{{- $ksvc := .Values.knativeService -}}
{{- $target := "" -}}
{{- if .Values.testCapp.enabled -}}
  {{- $ns = .Values.testCapp.namespace | default .Release.Namespace -}}
  {{- if not $ksvc }}{{- $ksvc = .Values.testCapp.name -}}{{- end -}}
  {{- $target = printf "http://%s.%s.svc.cluster.local" .Values.testCapp.name $ns -}}
{{- end -}}
- name: TARGET_URL
  value: {{ $target | quote }}
- name: KSVC_NAME
  value: {{ $ksvc | quote }}
- name: KSVC_NAMESPACE
  value: {{ $ns | quote }}
{{- $urls := .Values.victoriametrics.importUrls | default (list) }}
{{- if $urls }}
- name: VM_IMPORT_URLS
  value: {{ $urls | join "," | quote }}
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
