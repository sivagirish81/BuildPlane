{{- define "buildplane.labels" -}}
app.kubernetes.io/part-of: buildplane
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
{{- end -}}

{{- define "buildplane.image" -}}
{{- printf "%s:%s" .repository .tag -}}
{{- end -}}
