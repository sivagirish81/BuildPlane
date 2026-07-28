{{- define "buildplane.labels" -}}
app.kubernetes.io/part-of: buildplane
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
{{- end -}}

{{- define "buildplane.image" -}}
{{- printf "%s:%s" .repository .tag -}}
{{- end -}}

{{- define "buildplane.databaseSecretName" -}}
{{- required "database.existingSecret is required when database.createSecret=false" .Values.database.existingSecret -}}
{{- end -}}

{{- define "buildplane.databaseSecretKey" -}}
{{- required "database.urlKey is required" .Values.database.urlKey -}}
{{- end -}}

{{- define "buildplane.postgresName" -}}
buildplane-postgres
{{- end -}}

{{- define "buildplane.databaseURL" -}}
{{- if .Values.database.url -}}
{{- .Values.database.url -}}
{{- else if .Values.postgres.enabled -}}
{{- printf "postgres://%s:%s@%s:5432/%s?sslmode=disable" .Values.postgres.auth.username .Values.postgres.auth.password (include "buildplane.postgresName" .) .Values.postgres.auth.database -}}
{{- else -}}
{{- required "database.url is required when database.createSecret=true and postgres.enabled=false" .Values.database.url -}}
{{- end -}}
{{- end -}}

{{- define "buildplane.databaseURLEnv" -}}
- name: BUILDPLANE_DATABASE_URL
  valueFrom:
    secretKeyRef:
      name: {{ include "buildplane.databaseSecretName" . }}
      key: {{ include "buildplane.databaseSecretKey" . }}
{{- end -}}
