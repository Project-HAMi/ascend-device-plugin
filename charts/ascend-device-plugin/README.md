# ascend-device-plugin

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square)  ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square)  ![AppVersion: v1.3.0](https://img.shields.io/badge/AppVersion-v1.3.0-informational?style=flat-square)

HAMi Ascend device plugin

This chart deploys the standalone HAMi Ascend device plugin manifests:

- RuntimeClass
- ConfigMaps
- RBAC and ServiceAccount
- Device plugin DaemonSet
- (Optional) vNPU monitor integration resources (Service, ServiceMonitor, PrometheusRule)

## Install

Label Ascend nodes before installing:

```bash
kubectl label node <ascend-node> ascend=on --overwrite
```

Install the chart:

```bash
helm install ascend-device-plugin ./charts/ascend-device-plugin \
  --namespace kube-system \
  --set image.tag=v1.3.0
```

If the HAMi chart already manages the Ascend device plugin DaemonSet, related ConfigMaps, RBAC, or RuntimeClass, do not deploy this standalone chart at the same time.

## Existing Device Configuration

If another chart, such as the HAMi chart, already owns the shared `hami-scheduler-device` ConfigMap, reuse it instead of creating another one:

```bash
helm install ascend-device-plugin ./charts/ascend-device-plugin \
  --namespace kube-system \
  --set image.tag=v1.3.0 \
  --set config.create=false \
  --set config.existingDeviceConfigMapName=hami-scheduler-device
```

With this mode, the chart mounts the existing device config and still manages `hami-device-node-config` by default.

## hami-vnpu-core

Enable the global `vnpus.hamiVnpuCore` switch in the generated device config:

```bash
helm install ascend-device-plugin ./charts/ascend-device-plugin \
  --namespace kube-system \
  --set image.tag=v1.3.0 \
  --set hamiVnpuCore.enabled=true
```

## vNPU Monitor Integration

The chart can also create the resources from `ascend-vnpu-monitor-integration.yaml`.

- `vnpuMonitor.enabled=true` creates the metrics `Service`.
- `ServiceMonitor` and `PrometheusRule` are created by default; ensure Prometheus Operator CRDs are installed first.
- You can disable either one with `vnpuMonitor.serviceMonitor.create=false` or `vnpuMonitor.prometheusRule.create=false`.

```bash
helm install ascend-device-plugin ./charts/ascend-device-plugin \
  --namespace kube-system \
  --set image.tag=v1.3.0 \
  --set hamiVnpuCore.enabled=true \
  --set vnpuMonitor.enabled=true
```

## Node Configuration

Override `nodeConfig` to enable or customize `hami-vnpu-core` per node:

```yaml
nodeConfig: |-
  nodes:
    - name: "ascend-node-1"
      hami-vnpu-core: true
      vDeviceCount: 8
```

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| config.create | bool | `true` | Create the device configuration ConfigMap. |
| config.deviceConfigMapName | string | `"hami-scheduler-device"` | Name of the chart-managed device configuration ConfigMap. |
| config.existingDeviceConfigMapName | string | `""` | Existing device configuration ConfigMap to mount instead of the chart-managed ConfigMap. |
| daemonSet.name | string | `"hami-ascend-device-plugin"` | Device plugin DaemonSet name. |
| fullnameOverride | string | `""` | Override the fully qualified resource name. |
| hamiVnpuCore.enabled | bool | `false` | Enable hami-vnpu-core in the generated global device configuration. |
| image.pullPolicy | string | `"IfNotPresent"` | Kubernetes image pull policy. |
| image.repository | string | `"projecthami/ascend-device-plugin"` | Container image repository. |
| image.tag | string | `""` | Container image tag. Defaults to the chart `appVersion` when empty. |
| nameOverride | string | `""` | Override the chart name used in resource names. |
| nodeConfig | string | `"nodes: []"` | Per-node hami-vnpu-core configuration written to the node ConfigMap. |
| nodeConfigMap.create | bool | `true` | Create the per-node configuration ConfigMap. |
| nodeConfigMap.name | string | `"hami-device-node-config"` | Per-node configuration ConfigMap name. |
| nodeSelector.ascend | string | `"on"` | Node label value used to schedule the device plugin. |
| rbac.name | string | `"hami-ascend"` | Name shared by the chart-managed RBAC resources. |
| resources.limits.cpu | string | `"500m"` | CPU limit for the device plugin container. |
| resources.limits.memory | string | `"500Mi"` | Memory limit for the device plugin container. |
| resources.requests.cpu | string | `"500m"` | Requested CPU for the device plugin container. |
| resources.requests.memory | string | `"500Mi"` | Requested memory for the device plugin container. |
| runtimeClass.create | bool | `true` | Create the Ascend RuntimeClass. |
| runtimeClass.handler | string | `"ascend"` | Container runtime handler used by the RuntimeClass. |
| runtimeClass.name | string | `"ascend"` | RuntimeClass resource name. |
| serviceAccount.create | bool | `true` | Create a ServiceAccount for the device plugin. |
| serviceAccount.name | string | `"hami-ascend"` | ServiceAccount name. Defaults to the chart fullname when empty and creation is enabled. |
| vnpuMonitor.enabled | bool | `false` | Create vNPU monitoring integration resources. |
| vnpuMonitor.prometheusRule.create | bool | `true` | Create a Prometheus Operator PrometheusRule. |
| vnpuMonitor.prometheusRule.groupLabels.vendor | string | `"ascend"` | Vendor label applied to the Prometheus rule group. |
| vnpuMonitor.prometheusRule.groupName | string | `"ascend-vnpu"` | Prometheus rule group name. |
| vnpuMonitor.prometheusRule.interval | string | `"15s"` | Prometheus rule evaluation interval. |
| vnpuMonitor.prometheusRule.labels.release | string | `"prometheus"` | Prometheus release label applied to the PrometheusRule. |
| vnpuMonitor.prometheusRule.labels.role | string | `"recording-rules"` | Role label applied to the PrometheusRule. |
| vnpuMonitor.prometheusRule.labels.vendor | string | `"ascend"` | Vendor label applied to the PrometheusRule. |
| vnpuMonitor.prometheusRule.name | string | `"hami-ascend-vnpu-monitor"` | PrometheusRule name. |
| vnpuMonitor.prometheusRule.namespace | string | `"monitoring"` | Namespace in which to create the PrometheusRule. |
| vnpuMonitor.service.name | string | `"hami-ascend-device-plugin-metrics"` | Metrics Service name. |
| vnpuMonitor.serviceMonitor.create | bool | `true` | Create a Prometheus Operator ServiceMonitor. |
| vnpuMonitor.serviceMonitor.interval | string | `"15s"` | Metrics scrape interval. |
| vnpuMonitor.serviceMonitor.labels.release | string | `"prometheus"` | Prometheus release label applied to the ServiceMonitor. |
| vnpuMonitor.serviceMonitor.name | string | `"hami-ascend-vnpu-monitor"` | ServiceMonitor name. |
| vnpuMonitor.serviceMonitor.namespace | string | `"monitoring"` | Namespace in which to create the ServiceMonitor. |
| vnpuMonitor.serviceMonitor.path | string | `"/metrics"` | Metrics HTTP path. |
