# ascend-device-plugin

![Version: 0.1.1](https://img.shields.io/badge/Version-0.1.1-informational?style=flat-square)  ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square)  ![AppVersion: v1.4.0](https://img.shields.io/badge/AppVersion-v1.4.0-informational?style=flat-square)

HAMi Ascend device plugin

This chart deploys the standalone HAMi Ascend device plugin manifests:

- RuntimeClass
- ConfigMaps
- RBAC and ServiceAccount
- Device plugin DaemonSet

## Install

Label Ascend nodes before installing:

```bash
kubectl label node <ascend-node> ascend=on --overwrite
```

Install the chart:

```bash
helm install ascend-device-plugin ./charts/ascend-device-plugin \
  --namespace kube-system
```

If the HAMi chart already manages the Ascend device plugin DaemonSet, related ConfigMaps, RBAC, or RuntimeClass, do not deploy this standalone chart at the same time.

## Device Configuration Ownership

By default, this chart creates the global device configuration ConfigMap. Set
`config.create=false` only when another chart, such as the HAMi chart, owns
the configuration. The external ConfigMap must be in the release namespace:

```bash
helm install ascend-device-plugin ./charts/ascend-device-plugin \
  --namespace kube-system \
  --set config.create=false \
  --set config.existingDeviceConfigMapName=hami-scheduler-device
```

With this mode, the chart mounts the existing device config and still manages
`hami-device-node-config` by default. The device configuration uses a
`subPath` mount, so update the DaemonSet manually after changing an external
ConfigMap.

## hami-vnpu-core

When `config.create=true`, enable the global `vnpus.hamiVnpuCore` switch in
the generated device config:

```bash
helm install ascend-device-plugin ./charts/ascend-device-plugin \
  --namespace kube-system \
  --set hamiVnpuCore.enabled=true
```

When `config.create=false`, configure the global switch in the external
ConfigMap instead. A matching node entry in `nodeConfig` overrides the global
setting for that node. A workload still needs the
`huawei.com/vnpu-mode: hami-core` annotation to use the soft-slicing Allocate
path.

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
| config.deviceConfigMapName | string | `"hami-scheduler-device"` | Name of the chart-managed device configuration ConfigMap when `config.create=true`. |
| config.existingDeviceConfigMapName | string | `""` | Existing device configuration ConfigMap to mount when `config.create=false`; it must be in the release namespace. |
| daemonSet.args[0] | string | `"--config_file"` |  |
| daemonSet.args[1] | string | `"/device-config.yaml"` |  |
| daemonSet.args[2] | string | `"--node_config_file"` |  |
| daemonSet.args[3] | string | `"/node-config.yaml"` |  |
| daemonSet.args[4] | string | `"--v=4"` |  |
| daemonSet.name | string | `"hami-ascend-device-plugin"` | Device plugin DaemonSet name. |
| fullnameOverride | string | `""` | Override the fully qualified resource name. |
| hamiVnpuCore.enabled | bool | `false` | Enable hami-vnpu-core in the generated global device configuration. |
| image.digest | string | `""` | Optional immutable OCI manifest digest. When set, it takes precedence over `image.tag`. |
| image.pullPolicy | string | `"IfNotPresent"` | Kubernetes image pull policy. |
| image.repository | string | `"projecthami/ascend-device-plugin"` | Container image repository. |
| image.tag | string | `""` | Container image tag. Defaults to the chart `appVersion` when empty. |
| nameOverride | string | `""` | Override the chart name used in resource names. |
| nodeConfig | string | `"nodes: []"` | Per-node hami-vnpu-core configuration written to the node ConfigMap. |
| nodeConfigMap.create | bool | `true` | Create the per-node configuration ConfigMap. |
| nodeConfigMap.name | string | `"hami-device-node-config"` | Per-node configuration ConfigMap name. |
| nodeSelector.ascend | string | `"on"` | Node label value used to schedule the device plugin. |
| rbac.name | string | `"hami-ascend"` | Name shared by the chart-managed RBAC resources. |
| resources.limits.cpu | string | `"500m"` |  |
| resources.limits.memory | string | `"500Mi"` |  |
| resources.requests.cpu | string | `"500m"` |  |
| resources.requests.memory | string | `"500Mi"` |  |
| runtimeClass.create | bool | `true` | Create the Ascend RuntimeClass. |
| runtimeClass.handler | string | `"ascend"` | Container runtime handler used by the RuntimeClass. |
| runtimeClass.name | string | `"ascend"` | RuntimeClass resource name. |
| serviceAccount.create | bool | `true` | Create a ServiceAccount for the device plugin. |
| serviceAccount.name | string | `"hami-ascend"` | ServiceAccount name. Defaults to the chart fullname when empty and creation is enabled. |
