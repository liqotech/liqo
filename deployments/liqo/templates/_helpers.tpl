{{/*
Expand the name of the chart.
*/}}
{{- define "liqo.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "liqo.fullname" -}}
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
{{- define "liqo.chart" -}}
{{- printf "%s-%s" .Chart.Name (include "liqo.version" .) | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create version used to select the liqo version to be installed .
*/}}
{{- define "liqo.version" -}}
{{- if .version }}
{{- .version }}
{{- else if .Values.tag }}
{{- .Values.tag }}
{{- else if .Chart.AppVersion }}
{{- .Chart.AppVersion }}
{{- else }}
{{- fail "At least one between .Values.tag and .Chart.AppVersion should be set" }}
{{- end }}
{{- end }}

{{/*
The suffix added to the Liqo images, to identify CI builds.
*/}}
{{- define "liqo.suffix" -}}
{{/* https://semver.org/#is-there-a-suggested-regular-expression-regex-to-check-a-semver-string */}}
{{- $semverregex := "^v(?P<major>0|[1-9]\\d*)\\.(?P<minor>0|[1-9]\\d*)\\.(?P<patch>0|[1-9]\\d*)(?:-(?P<prerelease>(?:0|[1-9]\\d*|\\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\\.(?:0|[1-9]\\d*|\\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\\+(?P<buildmetadata>[0-9a-zA-Z-]+(?:\\.[0-9a-zA-Z-]+)*))?$" }}
{{- if or (eq .Values.tag "") (mustRegexMatch $semverregex .Values.tag) (.version) }}
{{- print "" }}
{{- else }}
{{- print "-ci" }}
{{- end }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "liqo.labels" -}}
{{ include "liqo.selectorLabels" . }}
helm.sh/chart: {{ quote (include "liqo.chart" .) }}
app.kubernetes.io/version: {{ quote (include "liqo.version" .) }}
app.kubernetes.io/managed-by: {{ quote .Release.Service }}
{{- end }}

{{/*
Selector labels, it accepts a dict which contains fields "name" and "module"
*/}}
{{- define "liqo.selectorLabels" -}}
app.kubernetes.io/name: {{ quote .name }}
app.kubernetes.io/instance: {{ quote (printf "%s-%s" .Release.Name .name) }}
app.kubernetes.io/component: {{ quote .module }}
app.kubernetes.io/part-of: {{ quote (include "liqo.name" .) }}
{{- end }}

{{/*
Common metadata for Gateway Templates
*/}}
{{- define "liqo.metadataTemplate" -}}
name: {{ quote "{{ .Name }}" }}
namespace: {{ quote "{{ .Namespace }}" }}
labels:
{{ include "liqo.labelsTemplate" . | indent 2 }}
{{- end }}

{{/*
Common Labels for Gateway Templates
*/}}
{{- define "liqo.labelsTemplate" -}}
{{ include "liqo.selectorLabelsTemplate" . }}
helm.sh/chart: {{ quote (include "liqo.chart" .) }}
app.kubernetes.io/version: {{ quote (include "liqo.version" .) }}
app.kubernetes.io/managed-by: {{ quote .Release.Service }}
networking.liqo.io/component: "gateway"
networking.liqo.io/gateway-name: "{{"{{ .Name }}"}}"
networking.liqo.io/gateway-namespace: "{{"{{ .Namespace }}"}}"
{{- if .isService }}
networking.liqo.io/active: "true"
{{- end }}
{{- end }}

{{/*
Selector labels template, it accepts a dict which contains fields "name" and "module"
*/}}
{{- define "liqo.selectorLabelsTemplate" -}}
app.kubernetes.io/name: {{ quote "{{ .Name }}" }}
app.kubernetes.io/instance: {{ quote (printf "%s-%s" .Release.Name "{{ .Name }}") }}
app.kubernetes.io/component: {{ quote .module }}
app.kubernetes.io/part-of: {{ quote (include "liqo.name" .) }}
{{- end }}

{{/*
Create a name prefixed with the chart name, it accepts a dict which contains the field "name".
*/}}
{{- define "liqo.prefixedName" -}}
{{- printf "%s-%s" (include "liqo.name" .) .name }}
{{- end }}

{{/*
Create the file name of a role starting from a prefix, it accepts a dict which contains the field "prefix".
*/}}
{{- define "liqo.role-filename" -}}
{{- printf "files/%s-%s" .prefix "Role.yaml" }}
{{- end }}

{{/*
Create the file name of a cluster role starting from a prefix, it accepts a dict which contains the field "prefix".
*/}}
{{- define "liqo.cluster-role-filename" -}}
{{- printf "files/%s-%s" .prefix "ClusterRole.yaml" }}
{{- end }}

{{/*
Metrics pod labels
*/}}
{{- define "liqo.metricAgentServiceLabels" -}}
run: metric-agent
{{- end }}

{{/*
Webhook pod labels
*/}}
{{- define "liqo.webhookServiceLabels" -}}
webhook.liqo.io/backend: "liqo-webhook"
{{- end }}

{{/*
Pre-delete hook Annotations
*/}}
{{- define "liqo.preDeleteAnnotations" -}}
"helm.sh/hook": pre-delete
"helm.sh/hook-weight": "-5"
"helm.sh/hook-delete-policy": before-hook-creation,hook-succeeded
{{- end }}

{{/*
Concatenates a values dictionary into a string in the form "--commandName=key1=val1,key2=val2"
*/}}
{{- define "liqo.concatenateMap" -}}
{{- $res := print .commandName "=" -}}
{{- range $key, $val := .dictionary -}}
{{- $res = print $res $key "=" $val "," -}}
{{- end -}}
- {{ trimSuffix "," $res }}
{{- end -}}

{{/*
Concatenates a values list into a string in the form "--commandName=val1,val2"
*/}}
{{- define "liqo.concatenateList" -}}
{{- $res := print .commandName "=" -}}
{{- range $val := .list -}}
{{- $res = print $res $val "," -}}
{{- end -}}
- {{ trimSuffix "," $res }}
{{- end -}}

{{/*
Concatenates a values list into a string in the form "--commandName=val1;default,val2"
*/}}
{{- define "liqo.concatenateListDefault" -}}
{{- $res := print .commandName "=" -}}
{{- range $val := .list -}}
{{- $res = print $res $val.name -}}
{{- if $val.default -}}
{{- $res = print $res ";default" -}}
{{- end -}}
{{- $res = print $res "," -}}
{{- end -}}
- {{ trimSuffix "," $res }}
{{- end -}}


{{/*
Concatenates a values list of groupVersionResources into a string in the form "--commandName=group1/version1/resource1,group2/version2/resource2"
*/}}
{{- define "liqo.concatenateGroupVersionResources" -}}
{{- $res := print .commandName "=" -}}
{{- range $val := .list -}}
{{- $res = print $res $val.apiVersion "/" $val.resource "," -}}
{{- end -}}
- {{ trimSuffix "," $res }}
{{- end -}}

{{/*
Get the liqo clusterID ConfigMap name
*/}}
{{- define "liqo.clusterIdConfig" -}}
{{- $config := (merge (dict "name" "clusterid-configmap" "module" "clusterid-configmap") .) -}}
{{ include "liqo.prefixedName" $config }}
{{- end -}}

{{/*
Get the Pod security context
*/}}
{{- define "liqo.podSecurityContext" -}}
{{- if not .Values.openshiftConfig.enabled }}
runAsNonRoot: true
runAsUser: 1000
runAsGroup: 1000
fsGroup: 1000
{{- end }}
{{- end -}}

{{/*
Get the Container security context
*/}}
{{- define "liqo.containerSecurityContext" -}}
allowPrivilegeEscalation: false
{{- end -}}
