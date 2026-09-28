# ascend-device-plugin

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square)  ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square)  ![AppVersion: v1.4.0](https://img.shields.io/badge/AppVersion-v1.4.0-informational?style=flat-square)

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

## Existing Device Configuration

If another chart, such as the HAMi chart, already owns the shared `hami-scheduler-device` ConfigMap, reuse it instead of creating another one:

```bash
helm install ascend-device-plugin ./charts/ascend-device-plugin \
  --namespace kube-system \
  --set config.create=false \
  --set config.existingDeviceConfigMapName=hami-scheduler-device
```

With this mode, the chart mounts the existing device config and still manages `hami-device-node-config` by default.

## hami-vnpu-core

Enable the global `vnpus.hamiVnpuCore` switch in the generated device config:

```bash
helm install ascend-device-plugin ./charts/ascend-device-plugin \
  --namespace kube-system \
  --set hamiVnpuCore.enabled=true
```

### Compute Oversell

`hamiVnpuCore.deviceCoreScaling` sets how much compute the plugin advertises for
hami-core. The plugin registers `Devcore = round(100 * deviceCoreScaling)` and HAMi
admits `-core` requests against that budget:

```bash
helm install ascend-device-plugin ./charts/ascend-device-plugin \
  --namespace kube-system \
  --set hamiVnpuCore.enabled=true \
  --set-json hamiVnpuCore.deviceCoreScaling=1.5
```

Use `--set-json` or a values file for fractional ratios. Plain `--set` parses `1.5` as a
string, which the chart schema rejects.

With `1.5` the advertised budget is 150, so three pods requesting `-core: "30"` plus one
requesting `-core: "20"` fit on the same card. The default `1` keeps the 100-point budget
and preserves current behavior. Ratios below `1` are rejected by the chart schema, and the
plugin falls back to `1` if the device config sets one directly.

Only compute is oversold. Device memory is never scaled, and pod density per card is
still capped by `vDeviceCount`, so raising the ratio alone may not admit more pods.

## ubs-virt-enpu (vCANN-RT)

Prepare hardware and runtime prerequisites using the [official vCANN-RT guide](https://docs.openeuler.org/zh/docs/24.03_LTS_SP3/unifiedbus/unifiedbus/ubs-virt/ubs-virt-enpu/vcann-rt/README.html). For per-node enablement that preserves other backends, see the [ENPU guide](../../examples/enpu/README.md), [ordinary soft-slicing Pod](../../examples/enpu/soft-slicing.yaml), and [mem-swap Pod](../../examples/enpu/mem-swap.yaml), with their accompanying values overlays. A2/910B does not require single-DIE mode; the current A3/910C integration does. Mem-swap requires a compatible manager/runtime version and is not enabled by annotations alone.

Enable ENPU mode when the node has the ubs-virt/vCANN-RT host assets installed:

```bash
helm install ascend-device-plugin ./charts/ascend-device-plugin \
  --namespace kube-system \
  --set enpu.enabled=true
```

ENPU uses the same Ascend resource names as the device configuration, and selects
the runtime with a pod annotation. A container must request exactly one physical
NPU; `-memory` is the memory quota in MiB and `-core` is the AICore quota from 1
to 100 percent:

```yaml
metadata:
  annotations:
    huawei.com/vnpu-mode: enpu
spec:
  runtimeClassName: ascend
  containers:
    - name: workload
      resources:
        limits:
          huawei.com/Ascend910C: "1"
          huawei.com/Ascend910C-memory: "16384"
          huawei.com/Ascend910C-core: "20"
```

The plugin writes one `npu_info.config` per container and mounts it at
`/etc/enpu/vcann-rt/npu_info.config`, together with the official Kubernetes
mount layout: `/usr/local/sbin`, `/usr/local/Ascend/driver`, `/dev/shm`, the
configured vCANN-RT library, monitor, and preload file. The workload image
must provide `/usr/bin/systemd-detect-virt`; set `enpu.systemdDetectVirtPath`
only when a compatible host binary must be mounted. By default the host runtime
assets are expected under `/usr/local/enpu/vcann-rt`; change the `enpu.*Path` values when
the installation uses another location. The default scheduling policy is
`elastic`; use `huawei.com/enpu-policy` with `fixed-share`, `elastic`, or
`best-effort` to select a policy. Workloads sharing one physical NPU must use the
same policy.

The plugin sets `ASCEND_VISIBLE_DEVICES` for the die selected by HAMi and keeps
`enpu.exposeAllDevices=false` so a workload does not receive every `/dev/davinci*`
node. `/dev/shm` is shared with the node for mem-swap so Pods can see the
enpu-manager `shm-id` object.

Set `enpu.defaultPolicy` to the same default used by the HAMi scheduler chart
(`devices.ascend.enpuPolicy`) when the two charts are installed separately.
For mem-swap manager integration, set `enpu.managerURL` to the node-local
enpu-manager REST endpoint and set `enpu.managerConfigRoot` to the manager's
`config_dir` (for example `/etc/enpu`) or its already-expanded
`/etc/enpu/vcann-rt` directory; both forms are accepted. When `managerURL` is empty, the plugin keeps its local
config-file path and the legacy ENPU behavior.
For mem-swap, set `huawei.com/enpu-memory-limit` in MiB. The optional
`huawei.com/enpu-memory-request` defaults to the requested `-memory` resource;
keep an explicit request equal to `-memory`, and the
limit may be larger than the request when the manager has oversubscription
enabled. A Pod must set `huawei.com/vnpu-mode: enpu`; an unannotated Pod is
rejected on an ENPU-only node.
ENPU and hami-vnpu-core can be enabled in the same installation. The pod's
`huawei.com/vnpu-mode` annotation selects the backend. ENPU supports one physical
NPU per container and does not use hami-core compute oversell.

## Node Configuration

Override `nodeConfig` to enable or customize `hami-vnpu-core` per node. Each node may
also override `deviceCoreScaling`:

```yaml
nodeConfig: |-
  nodes:
    - name: "ascend-node-1"
      hami-vnpu-core: true
      vDeviceCount: 8
      deviceCoreScaling: 1.5
```

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| config.create | bool | `true` | Create the device configuration ConfigMap. |
| config.deviceConfigMapName | string | `"hami-scheduler-device"` | Name of the chart-managed device configuration ConfigMap. |
| config.existingDeviceConfigMapName | string | `""` | Existing device configuration ConfigMap to mount instead of the chart-managed ConfigMap. |
| daemonSet.args[0] | string | `"--config_file"` |  |
| daemonSet.args[1] | string | `"/device-config.yaml"` |  |
| daemonSet.args[2] | string | `"--node_config_file"` |  |
| daemonSet.args[3] | string | `"/node-config.yaml"` |  |
| daemonSet.args[4] | string | `"--v=4"` |  |
| daemonSet.name | string | `"hami-ascend-device-plugin"` | Device plugin DaemonSet name. |
| enpu.configRoot | string | `"/var/lib/hami-enpu"` | Host directory for per-container ENPU configs. |
| enpu.defaultPolicy | string | `"elastic"` | Default ENPU scheduling policy. |
| enpu.enabled | bool | `false` | Enable ENPU in the generated device config. |
| enpu.exposeAllDevices | bool | `false` | Expose all davinci devices; keep false for device isolation. |
| enpu.managerConfigRoot | string | `""` | Manager config_dir or its vcann-rt subdirectory; empty uses configRoot. |
| enpu.managerURL | string | `""` | Optional node-local enpu-manager REST endpoint. |
| enpu.monitorPath | string | `"/usr/local/enpu/vcann-rt/tools/enpu-monitor"` | Host path to enpu-monitor. |
| enpu.preloadPath | string | `"/usr/local/enpu/vcann-rt/ld.so.preload"` | Host path to ld.so.preload. |
| enpu.runtimePath | string | `"/usr/local/enpu/vcann-rt/lib/libvruntime.so"` | Host path to libvruntime.so. |
| enpu.systemdDetectVirtPath | string | `""` | Optional host binary; prefer providing it in the workload image. |
| fullnameOverride | string | `""` | Override the fully qualified resource name. |
| hamiVnpuCore.deviceCoreScaling | float | `1` | hami-core compute oversell ratio. The plugin advertises `Devcore = round(100 * deviceCoreScaling)` so HAMi can admit more than 100% of `-core` on one card. Values below 1 are not supported. |
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
| resources.limits.cpu | string | `"500m"` |  |
| resources.limits.memory | string | `"500Mi"` |  |
| resources.requests.cpu | string | `"500m"` |  |
| resources.requests.memory | string | `"500Mi"` |  |
| runtimeClass.create | bool | `true` | Create the Ascend RuntimeClass. |
| runtimeClass.handler | string | `"ascend"` | Container runtime handler used by the RuntimeClass. |
| runtimeClass.name | string | `"ascend"` | RuntimeClass resource name. |
| serviceAccount.create | bool | `true` | Create a ServiceAccount for the device plugin. |
| serviceAccount.name | string | `"hami-ascend"` | ServiceAccount name. Defaults to the chart fullname when empty and creation is enabled. |
