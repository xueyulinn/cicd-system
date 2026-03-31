{{- define "e-team.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "e-team.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name (include "e-team.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "e-team.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" -}}
{{- end -}}

{{- define "e-team.labels" -}}
helm.sh/chart: {{ include "e-team.chart" . }}
app.kubernetes.io/name: {{ include "e-team.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "e-team.selectorLabels" -}}
app.kubernetes.io/name: {{ include "e-team.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "e-team.componentLabels" -}}
{{ include "e-team.selectorLabels" .context }}
app.kubernetes.io/component: {{ .component }}
{{- end -}}

{{- define "e-team.apiGatewayName" -}}
{{- printf "%s-api-gateway" (include "e-team.fullname" .) -}}
{{- end -}}

{{- define "e-team.validationServiceName" -}}
{{- printf "%s-validation-service" (include "e-team.fullname" .) -}}
{{- end -}}

{{- define "e-team.executionServiceName" -}}
{{- printf "%s-execution-service" (include "e-team.fullname" .) -}}
{{- end -}}

{{- define "e-team.workerServiceName" -}}
{{- printf "%s-worker-service" (include "e-team.fullname" .) -}}
{{- end -}}

{{- define "e-team.reportingServiceName" -}}
{{- printf "%s-reporting-service" (include "e-team.fullname" .) -}}
{{- end -}}

{{- define "e-team.postgresName" -}}
{{- printf "%s-postgres" (include "e-team.fullname" .) -}}
{{- end -}}

{{- define "e-team.postgresHeadlessName" -}}
{{- printf "%s-postgres-headless" (include "e-team.fullname" .) -}}
{{- end -}}

{{- define "e-team.postgresSecretName" -}}
{{- printf "%s-postgres-credentials" (include "e-team.fullname" .) -}}
{{- end -}}

{{- define "e-team.migrationJobName" -}}
{{- printf "%s-report-db-migrate" (include "e-team.fullname" .) -}}
{{- end -}}

{{- define "e-team.prometheusName" -}}
{{- printf "%s-prometheus" (include "e-team.fullname" .) -}}
{{- end -}}

{{- define "e-team.lokiName" -}}
{{- printf "%s-loki" (include "e-team.fullname" .) -}}
{{- end -}}

{{- define "e-team.tempoName" -}}
{{- printf "%s-tempo" (include "e-team.fullname" .) -}}
{{- end -}}

{{- define "e-team.otelCollectorName" -}}
{{- printf "%s-otel-collector" (include "e-team.fullname" .) -}}
{{- end -}}

{{- define "e-team.grafanaName" -}}
{{- printf "%s-grafana" (include "e-team.fullname" .) -}}
{{- end -}}

{{- define "e-team.observabilityPrometheusPVCName" -}}
{{- printf "%s-prometheus-data" (include "e-team.fullname" .) -}}
{{- end -}}

{{- define "e-team.observabilityLokiPVCName" -}}
{{- printf "%s-loki-data" (include "e-team.fullname" .) -}}
{{- end -}}

{{- define "e-team.observabilityTempoPVCName" -}}
{{- printf "%s-tempo-data" (include "e-team.fullname" .) -}}
{{- end -}}

{{- define "e-team.observabilityGrafanaPVCName" -}}
{{- printf "%s-grafana-data" (include "e-team.fullname" .) -}}
{{- end -}}

{{- define "e-team.otelHTTPEndpoint" -}}
{{- printf "http://%s:%v" (include "e-team.otelCollectorName" .) .Values.observability.otelCollector.service.otlpHttpPort -}}
{{- end -}}

{{- define "e-team.databaseURL" -}}
{{- if .Values.postgres.enabled -}}
{{- printf "postgres://%s:%s@%s:%v/%s?sslmode=disable" .Values.postgres.auth.username .Values.postgres.auth.password (include "e-team.postgresName" .) .Values.postgres.service.port .Values.postgres.auth.database -}}
{{- else -}}
{{- required "externalDatabase.url is required when postgres.enabled=false" .Values.externalDatabase.url -}}
{{- end -}}
{{- end -}}

{{- define "e-team.databaseHost" -}}
{{- if .Values.postgres.enabled -}}
{{- include "e-team.postgresName" . -}}
{{- else -}}
{{- required "externalDatabase.host is required when postgres.enabled=false and database wait init containers are enabled" .Values.externalDatabase.host -}}
{{- end -}}
{{- end -}}

{{- define "e-team.databasePort" -}}
{{- if .Values.postgres.enabled -}}
{{- .Values.postgres.service.port -}}
{{- else -}}
{{- .Values.externalDatabase.port -}}
{{- end -}}
{{- end -}}

{{- define "e-team.databaseUser" -}}
{{- if .Values.postgres.enabled -}}
{{- .Values.postgres.auth.username -}}
{{- else -}}
{{- required "externalDatabase.username is required when postgres.enabled=false and database wait init containers are enabled" .Values.externalDatabase.username -}}
{{- end -}}
{{- end -}}

{{- define "e-team.databasePassword" -}}
{{- if .Values.postgres.enabled -}}
{{- .Values.postgres.auth.password -}}
{{- else -}}
{{- required "externalDatabase.password is required when postgres.enabled=false and database wait init containers are enabled" .Values.externalDatabase.password -}}
{{- end -}}
{{- end -}}

{{- define "e-team.databaseName" -}}
{{- if .Values.postgres.enabled -}}
{{- .Values.postgres.auth.database -}}
{{- else -}}
{{- required "externalDatabase.database is required when postgres.enabled=false and database wait init containers are enabled" .Values.externalDatabase.database -}}
{{- end -}}
{{- end -}}
