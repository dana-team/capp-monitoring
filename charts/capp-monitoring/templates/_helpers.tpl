{{/*
Validate extraLabels keys and values for safe use in URLs and CLI args.
Keys must be valid Prometheus label names; values must be alphanumeric (plus _ . -).
*/}}
{{- define "capp-monitoring.validateExtraLabels" -}}
{{- range $k, $v := .Values.extraLabels -}}
{{- if not (regexMatch "^[a-zA-Z_][a-zA-Z0-9_]*$" $k) -}}
{{- fail (printf "extraLabels key %q is invalid: must match [a-zA-Z_][a-zA-Z0-9_]*" $k) -}}
{{- end -}}
{{- if not (regexMatch "^[a-zA-Z0-9_.-]*$" ($v | toString)) -}}
{{- fail (printf "extraLabels value %q for key %q is invalid: must match [a-zA-Z0-9_.-]*" ($v | toString) $k) -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Build VM import extra_label query params from extraLabels.
*/}}
{{- define "capp-monitoring.extraLabelsQuery" -}}
{{- include "capp-monitoring.validateExtraLabels" . -}}
{{- range $k, $v := .Values.extraLabels -}}
&extra_label={{ $k }}={{ $v }}
{{- end -}}
{{- end -}}

{{/*
Build k6 --tag flags from extraLabels.
*/}}
{{- define "capp-monitoring.k6ExtraTags" -}}
{{- include "capp-monitoring.validateExtraLabels" . -}}
{{- range $k, $v := .Values.extraLabels }} --tag {{ $k }}={{ $v }}{{- end -}}
{{- end -}}

{{- define "capp-monitoring.labels" -}}
app.kubernetes.io/name: {{ .Chart.Name }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Env vars for the scale benchmark and other components that target testCapp.
*/}}
{{- define "capp-monitoring.envVars" -}}
{{- $ns := .Values.cappNamespace -}}
{{- $capp := .Values.cappName -}}
{{- $target := .Values.targetUrl -}}
{{- if .Values.testCapp.enabled -}}
  {{- $ns = .Values.testCapp.namespace | default .Release.Namespace -}}
  {{- if not $capp }}{{- $capp = .Values.testCapp.name -}}{{- end -}}
  {{- if not $target }}{{- $target = printf "http://%s.%s.svc.cluster.local" .Values.testCapp.name $ns -}}{{- end -}}
{{- end -}}
- name: TARGET_URL
  value: {{ $target | quote }}
- name: CAPP_NAME
  value: {{ $capp | quote }}
- name: CAPP_NAMESPACE
  value: {{ $ns | quote }}
{{- with .Values.victoriametrics.importUrls }}
- name: VM_IMPORT_URLS
  value: {{ . | join "," | quote }}
{{- end }}
{{- if .Values.pushgatewayUrl }}
- name: PUSHGATEWAY_URL
  value: {{ .Values.pushgatewayUrl | quote }}
{{- end }}
{{- if .Values.extraLabels }}
- name: EXTRA_LABELS_QUERY
  value: {{ include "capp-monitoring.extraLabelsQuery" . | quote }}
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

{{/*
True when the on-demand Job targets latency.testCapp (k6-latency or cold-start).
*/}}
{{- define "capp-monitoring.oneshotUsesLatencyCapp" -}}
{{- if and .Values.oneshot.enabled (or (eq .Values.oneshot.type "k6-latency") (eq .Values.oneshot.type "cold-start")) }}true{{- end -}}
{{- end }}

{{/*
Env vars for the latency benchmark — targets latency.testCapp instead of testCapp.
*/}}
{{- define "capp-monitoring.latencyEnvVars" -}}
{{- $ns := .Values.testCapp.namespace | default .Release.Namespace -}}
{{- $capp := required "latency.testCapp.name is required when the latency CronJob or a latency oneshot is enabled" .Values.latency.testCapp.name -}}
{{- $target := printf "http://%s.%s.svc.cluster.local" $capp $ns -}}
- name: TARGET_URL
  value: {{ $target | quote }}
- name: CAPP_NAME
  value: {{ $capp | quote }}
- name: CAPP_NAMESPACE
  value: {{ $ns | quote }}
{{- with .Values.victoriametrics.importUrls }}
- name: VM_IMPORT_URLS
  value: {{ . | join "," | quote }}
- name: K6_PROMETHEUS_RW_SERVER_URL
  value: {{ printf "%s/api/v1/write" (first .) | quote }}
{{- end }}
{{- if .Values.pushgatewayUrl }}
- name: PUSHGATEWAY_URL
  value: {{ .Values.pushgatewayUrl | quote }}
{{- end }}
{{- if .Values.extraLabels }}
- name: EXTRA_LABELS_QUERY
  value: {{ include "capp-monitoring.extraLabelsQuery" . | quote }}
{{- end }}
- name: LATENCY_PATH
  value: {{ .Values.latency.path | quote }}
{{- end }}
