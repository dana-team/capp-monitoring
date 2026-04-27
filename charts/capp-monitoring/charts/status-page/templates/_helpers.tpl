{{- define "status-page.fullname" -}}
{{- printf "%s-status-page" .Release.Name }}
{{- end }}

{{- define "status-page.labels" -}}
app.kubernetes.io/name: status-page
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "status-page.selectorLabels" -}}
app.kubernetes.io/name: status-page
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
