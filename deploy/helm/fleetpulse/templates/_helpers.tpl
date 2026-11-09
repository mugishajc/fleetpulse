{{- define "fleetpulse.labels" -}}
app.kubernetes.io/part-of: fleetpulse
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}
