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

{{- define "e-team.rabbitmqName" -}}
{{- printf "%s-rabbitmq" (include "e-team.fullname" .) -}}
{{- end -}}

{{- define "e-team.redisName" -}}
{{- printf "%s-redis" (include "e-team.fullname" .) -}}
{{- end -}}

{{- define "e-team.rabbitmqSecretName" -}}
{{- printf "%s-rabbitmq-credentials" (include "e-team.fullname" .) -}}
{{- end -}}

{{- define "e-team.workerGitAuthSecretName" -}}
{{- printf "%s-worker-git-auth" (include "e-team.fullname" .) -}}
{{- end -}}

{{- define "e-team.rabbitmqAmqpURL" -}}
{{- $u := urlquery .Values.rabbitmq.auth.username -}}
{{- $p := urlquery .Values.rabbitmq.auth.password -}}
{{- printf "amqp://%s:%s@%s:%v/" $u $p (include "e-team.rabbitmqName" .) .Values.rabbitmq.service.amqpPort -}}
{{- end -}}

{{- define "e-team.validationCacheRedisURL" -}}
{{- if .Values.validationService.cache.redisURL -}}
{{- .Values.validationService.cache.redisURL -}}
{{- else if .Values.redis.enabled -}}
{{- printf "redis://%s:%v/0" (include "e-team.redisName" .) .Values.redis.service.port -}}
{{- else -}}
{{- required "validationService.cache.redisURL is required when validationService.cache.enabled=true and redis.enabled=false" .Values.validationService.cache.redisURL -}}
{{- end -}}
{{- end -}}

{{- define "e-team.mysqlName" -}}
{{- printf "%s-mysql" (include "e-team.fullname" .) -}}
{{- end -}}

{{- define "e-team.mysqlHeadlessName" -}}
{{- printf "%s-mysql-headless" (include "e-team.fullname" .) -}}
{{- end -}}

{{- define "e-team.mysqlSecretName" -}}
{{- printf "%s-mysql-credentials" (include "e-team.fullname" .) -}}
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
{{- if .Values.mysql.enabled -}}
{{- printf "%s:%s@tcp(%s:%v)/%s?parseTime=true&charset=utf8mb4&loc=UTC" .Values.mysql.auth.username .Values.mysql.auth.password (include "e-team.mysqlName" .) .Values.mysql.service.port .Values.mysql.auth.database -}}
{{- else -}}
{{- required "externalDatabase.url is required when mysql.enabled=false" .Values.externalDatabase.url -}}
{{- end -}}
{{- end -}}

{{- define "e-team.databaseHost" -}}
{{- if .Values.mysql.enabled -}}
{{- include "e-team.mysqlName" . -}}
{{- else -}}
{{- required "externalDatabase.host is required when mysql.enabled=false and database wait init containers are enabled" .Values.externalDatabase.host -}}
{{- end -}}
{{- end -}}

{{- define "e-team.databasePort" -}}
{{- if .Values.mysql.enabled -}}
{{- .Values.mysql.service.port -}}
{{- else -}}
{{- .Values.externalDatabase.port -}}
{{- end -}}
{{- end -}}

{{- define "e-team.databaseUser" -}}
{{- if .Values.mysql.enabled -}}
{{- .Values.mysql.auth.username -}}
{{- else -}}
{{- required "externalDatabase.username is required when mysql.enabled=false and database wait init containers are enabled" .Values.externalDatabase.username -}}
{{- end -}}
{{- end -}}

{{- define "e-team.databasePassword" -}}
{{- if .Values.mysql.enabled -}}
{{- .Values.mysql.auth.password -}}
{{- else -}}
{{- required "externalDatabase.password is required when mysql.enabled=false and database wait init containers are enabled" .Values.externalDatabase.password -}}
{{- end -}}
{{- end -}}

{{- define "e-team.databaseName" -}}
{{- if .Values.mysql.enabled -}}
{{- .Values.mysql.auth.database -}}
{{- else -}}
{{- required "externalDatabase.database is required when mysql.enabled=false and database wait init containers are enabled" .Values.externalDatabase.database -}}
{{- end -}}
{{- end -}}
