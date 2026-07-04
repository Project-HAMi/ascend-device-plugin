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
