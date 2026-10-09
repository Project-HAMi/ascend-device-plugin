{{- define "ascend-device-plugin.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "ascend-device-plugin.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- include "ascend-device-plugin.name" . | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "ascend-device-plugin.daemonSetName" -}}
{{- default (include "ascend-device-plugin.fullname" .) .Values.daemonSet.name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "ascend-device-plugin.rbacName" -}}
{{- default (include "ascend-device-plugin.fullname" .) .Values.rbac.name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "ascend-device-plugin.image" -}}
{{- printf "%s:%s" .Values.image.repository (default .Chart.AppVersion .Values.image.tag) -}}
{{- end -}}

{{- define "ascend-device-plugin.selectorLabels" -}}
app.kubernetes.io/component: hami-ascend-device-plugin
app.kubernetes.io/name: {{ include "ascend-device-plugin.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "ascend-device-plugin.labels" -}}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
{{ include "ascend-device-plugin.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "ascend-device-plugin.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "ascend-device-plugin.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{- define "ascend-device-plugin.deviceConfigMapName" -}}
{{- default .Values.config.deviceConfigMapName .Values.config.existingDeviceConfigMapName -}}
{{- end -}}

{{- define "ascend-device-plugin.nodeConfigMapName" -}}
{{- default "hami-device-node-config" .Values.nodeConfigMap.name -}}
{{- end -}}

{{/* Keep the default mode and legacy precedence aligned with HAMi. */}}
{{- define "ascend-device-plugin.vnpuMode" -}}
{{- if hasKey .Values.enpu "enabled" -}}
  {{- fail "enpu.enabled has been removed; use hamiVnpuMode: enpu" -}}
{{- end -}}
{{- if not (kindIs "bool" .Values.hamiVnpuCore.enabled) -}}
  {{- fail "hamiVnpuCore.enabled must be a boolean; use hamiVnpuMode instead" -}}
{{- end -}}
{{- $mode := "" -}}
{{- if ne .Values.hamiVnpuMode nil -}}
  {{- if not (kindIs "string" .Values.hamiVnpuMode) -}}
    {{- fail "hamiVnpuMode must be a string: template, hami-core (or hamiCore), or enpu" -}}
  {{- end -}}
  {{- $mode = lower (trim .Values.hamiVnpuMode) -}}
{{- end -}}
{{- if eq $mode "" -}}
  {{- $mode = ternary "hami-core" "template" .Values.hamiVnpuCore.enabled -}}
{{- else if eq $mode "hamicore" -}}
  {{- $mode = "hami-core" -}}
{{- end -}}
{{- if not (has $mode (list "template" "hami-core" "enpu")) -}}
  {{- fail (printf "hamiVnpuMode must be template, hami-core (or hamiCore), or enpu, got %q" .Values.hamiVnpuMode) -}}
{{- end -}}
{{- $mode -}}
{{- end -}}
