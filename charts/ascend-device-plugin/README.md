# Ascend Device Plugin Helm Chart

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
  --set image.tag=v1.4.0
```

If the HAMi chart already manages the Ascend device plugin DaemonSet, related ConfigMaps, RBAC, or RuntimeClass, do not deploy this standalone chart at the same time.

## Existing Device Configuration

If another chart, such as the HAMi chart, already owns the shared `hami-scheduler-device` ConfigMap, reuse it instead of creating another one:

```bash
helm install ascend-device-plugin ./charts/ascend-device-plugin \
  --namespace kube-system \
  --set image.tag=v1.4.0 \
  --set config.create=false \
  --set config.existingDeviceConfigMapName=hami-scheduler-device
```

With this mode, the chart mounts the existing device config and still manages `hami-device-node-config` by default.

## hami-vnpu-core

Enable the global `vnpus.hamiVnpuCore` switch in the generated device config:

```bash
helm install ascend-device-plugin ./charts/ascend-device-plugin \
  --namespace kube-system \
  --set image.tag=v1.4.0 \
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
  --set image.tag=v1.4.0 \
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
