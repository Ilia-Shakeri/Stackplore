{{- define "stackplore.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "stackplore.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name (include "stackplore.name" .) | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}

{{- define "stackplore.apiName" -}}
{{- $full := include "stackplore.fullname" . -}}
{{- if not (regexMatch "^[a-z]" $full) -}}
{{- $full = printf "sp-%s" $full -}}
{{- end -}}
{{- if le (len $full) 59 -}}
{{- printf "%s-api" $full -}}
{{- else -}}
{{- printf "%s-%s-api" ($full | trunc 50 | trimSuffix "-") ($full | sha256sum | trunc 8) -}}
{{- end -}}
{{- end -}}

{{- define "stackplore.labels" -}}
app.kubernetes.io/name: {{ include "stackplore.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" }}
{{- end }}
{{- define "stackplore.image" -}}
{{- if .Values.api.image.digest -}}
{{- printf "%s@%s" .Values.api.image.repository .Values.api.image.digest -}}
{{- else -}}
{{- printf "%s:%s" .Values.api.image.repository .Values.api.image.tag -}}
{{- end -}}
{{- end -}}
