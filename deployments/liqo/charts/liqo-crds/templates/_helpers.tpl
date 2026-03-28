{{/*
Expand the name of the chart.
*/}}
{{- define "liqo-crds.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "liqo-crds.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if or (contains $name .Release.Name) (contains .Release.Name $name) }}
{{- printf "%s-crds" .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "liqo-crds.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "liqo-crds.labels" -}}
helm.sh/chart: {{ include "liqo-crds.chart" . }}
{{ include "liqo-crds.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "liqo-crds.selectorLabels" -}}
app.kubernetes.io/name: {{ include "liqo-crds.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create version used for the CRD upgrade image.
*/}}
{{- define "liqo-crds.version" -}}
{{- if .Values.crdUpgrade.image.version }}
{{- .Values.crdUpgrade.image.version }}
{{- else if .Chart.AppVersion }}
{{- .Chart.AppVersion }}
{{- else }}
{{- fail "At least one between .Values.crdUpgrade.image.version and .Chart.AppVersion should be set" }}
{{- end }}
{{- end }}

{{/*
The suffix added to the CRD upgrade image, to identify CI builds.
*/}}
{{- define "liqo-crds.suffix" -}}
{{/* https://semver.org/#is-there-a-suggested-regular-expression-regex-to-check-a-semver-string */}}
{{- $semverregex := "^v(?P<major>0|[1-9]\\d*)\\.(?P<minor>0|[1-9]\\d*)\\.(?P<patch>0|[1-9]\\d*)(?:-(?P<prerelease>(?:0|[1-9]\\d*|\\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\\.(?:0|[1-9]\\d*|\\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\\+(?P<buildmetadata>[0-9a-zA-Z-]+(?:\\.[0-9a-zA-Z-]+)*))?$" }}
{{- $version := include "liqo-crds.version" . }}
{{- if or (eq $version "") (mustRegexMatch $semverregex $version) }}
{{- print "" }}
{{- else }}
{{- print "-ci" }}
{{- end }}
{{- end }}
