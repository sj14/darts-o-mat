{{/*
Expand the name of the chart.
*/}}
{{- define "darts-o-mat.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "darts-o-mat.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "darts-o-mat.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "darts-o-mat.labels" -}}
helm.sh/chart: {{ include "darts-o-mat.chart" . }}
{{ include "darts-o-mat.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "darts-o-mat.selectorLabels" -}}
app.kubernetes.io/name: {{ include "darts-o-mat.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "darts-o-mat.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "darts-o-mat.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
The Secret that holds the PostgreSQL data source name.
*/}}
{{- define "darts-o-mat.dsnSecret" -}}
{{- default (printf "%s-dsn" (include "darts-o-mat.fullname" .)) .Values.config.postgres.existingSecret }}
{{- end }}

{{/*
The Secret with the Turnstile secret key: the chart's or one of the user's.
*/}}
{{- define "darts-o-mat.turnstileSecret" -}}
{{- default (printf "%s-turnstile" (include "darts-o-mat.fullname" .)) .Values.config.turnstile.existingSecret }}
{{- end }}

{{/*
The name of the chart's PostgreSQL: its StatefulSet, Service and Secret.
*/}}
{{- define "darts-o-mat.postgresql" -}}
{{- printf "%s-postgresql" (include "darts-o-mat.fullname" .) }}
{{- end }}

{{/*
The Secret with the password of the chart's PostgreSQL: the chart's or one of the user's.
*/}}
{{- define "darts-o-mat.postgresqlSecret" -}}
{{- default (include "darts-o-mat.postgresql" .) .Values.postgresql.auth.existingSecret }}
{{- end }}

{{/*
The password of the chart's PostgreSQL: the one set, else the one stored at the install
(so an upgrade keeps it), else a new one.
*/}}
{{- define "darts-o-mat.postgresqlPassword" -}}
{{- $secret := lookup "v1" "Secret" .Release.Namespace (include "darts-o-mat.postgresql" .) }}
{{- if .Values.postgresql.auth.password }}
{{- .Values.postgresql.auth.password }}
{{- else if and $secret (index $secret.data .Values.postgresql.auth.existingSecretKey) }}
{{- index $secret.data .Values.postgresql.auth.existingSecretKey | b64dec }}
{{- else }}
{{- randAlphaNum 32 }}
{{- end }}
{{- end }}

{{/*
Refuses settings that contradict each other, so nothing runs half configured.
*/}}
{{- define "darts-o-mat.validate" -}}
{{- $postgres := eq .Values.config.database "postgres" }}
{{- if not (has .Values.config.database (list "sqlite" "postgres")) }}
{{- fail "config.database must be sqlite or postgres" }}
{{- end }}
{{- if and (not $postgres) (gt (int .Values.replicaCount) 1) }}
{{- fail "replicaCount above 1 needs config.database=postgres: SQLite is one file for one process" }}
{{- end }}
{{- if and .Values.postgresql.enabled (not $postgres) }}
{{- fail "postgresql.enabled needs config.database=postgres" }}
{{- end }}
{{- if and .Values.postgresql.enabled (or .Values.config.postgres.dsn .Values.config.postgres.existingSecret) }}
{{- fail "config.postgres is for a PostgreSQL of your own: leave it empty with postgresql.enabled" }}
{{- end }}
{{- if and $postgres (not .Values.postgresql.enabled) (not .Values.config.postgres.dsn) (not .Values.config.postgres.existingSecret) }}
{{- fail "config.database=postgres needs postgresql.enabled, or config.postgres.dsn or config.postgres.existingSecret" }}
{{- end }}
{{- $turnstileSecret := or .Values.config.turnstile.secretKey .Values.config.turnstile.existingSecret }}
{{- if ne (not .Values.config.turnstile.siteKey) (not $turnstileSecret) }}
{{- fail "config.turnstile needs both keys or none: siteKey, and secretKey or existingSecret" }}
{{- end }}
{{- end }}
