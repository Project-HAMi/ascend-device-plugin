# Deploy & Use with Volcano

**English** | [中文](volcano_cn.md)

This guide covers deploying `ascend-device-plugin` for use with the [Volcano](https://github.com/volcano-sh/volcano) scheduler, via Volcano's `HAMi mode` `deviceshare` plugin. For background, see Volcano's own [Ascend vNPU user guide](https://github.com/volcano-sh/volcano/blob/master/docs/user-guide/how_to_use_vnpu.md).

## Prerequisites

- **Kubernetes**: ≥ 1.16
- **Volcano**: ≥ 1.14 (≥ 1.16 required for `hami-core` soft slicing)
- [ascend-docker-runtime](https://gitcode.com/Ascend/mind-cluster/tree/master/component/ascend-docker-runtime)
- **Ascend Driver Version**: ≥ 25.5
- **`npu-smi` must be reachable on the host** (only needed for `hami-core` soft slicing), at one of the following paths (checked in order):
  1. `/usr/local/Ascend/driver/tools/npu-smi` (from the existing driver hostPath mount)
  2. `/usr/local/sbin/npu-smi` (mounted by default)
  3. `/usr/local/bin/npu-smi`

  If `npu-smi` is located at `/usr/local/bin/npu-smi`, please add the path mount in `ascend-device-plugin.yaml`.

**Note:** `hami-core` soft slicing currently only supports ARM platforms; template-based hard slicing has no such restriction.

## Deployment

### Install Volcano

Follow the [Volcano Installer Guide](https://github.com/volcano-sh/volcano?tab=readme-ov-file#quick-start-guide) if Volcano isn't already installed in your cluster.

### Label the Node with `ascend=on`

```bash
kubectl label node {ascend-node} ascend=on
```

### Deploy RuntimeClass

```bash
kubectl apply -f https://raw.githubusercontent.com/Project-HAMi/ascend-device-plugin/main/ascend-runtimeclass.yaml
```

### Deploy ConfigMap

This configMap is used for global configurations, like resourceName, mode, and templates.
* (Optional) Under `vnpus`, set `hamiVnpuCore: true` if you want to enable `hami-vnpu-core` soft slicing on **all nodes** (unless overridden per node in `hami-device-node-config`).

```bash
kubectl apply -f https://raw.githubusercontent.com/Project-HAMi/ascend-device-plugin/main/ascend-device-configmap.yaml
```

#### (Optional) Node Custom Configuration

The `hami-device-node-config` is used to enable or override hami-vnpu-core for specific nodes within the cluster. Node-level settings take higher priority than the global `vnpus.hamiVnpuCore` switch. It also supports `filterDevices` to ignore specific devices on a node, e.g. `filterDevices: {index: [0, 1], uuid: []}`.

```bash
kubectl apply -f https://raw.githubusercontent.com/Project-HAMi/ascend-device-plugin/main/ascend-device-node-configmap.yaml
```

### Deploy `ascend-device-plugin`

```bash
kubectl apply -f https://raw.githubusercontent.com/Project-HAMi/ascend-device-plugin/main/ascend-device-plugin.yaml
```

### Update the Volcano scheduler config

Enable the `deviceshare` plugin's Ascend HAMi vNPU support in `volcano-scheduler-configmap`:

```yaml
kind: ConfigMap
apiVersion: v1
metadata:
  name: volcano-scheduler-configmap
  namespace: volcano-system
data:
  volcano-scheduler.conf: |
    actions: "enqueue, allocate, backfill"
    tiers:
    - plugins:
      - name: predicates
      - name: deviceshare
        arguments:
          deviceshare.AscendHAMiVNPUEnable: true   # enable ascend vnpu
          deviceshare.SchedulePolicy: binpack       # scheduling policy: binpack / spread
          deviceshare.KnownGeometriesCMNamespace: kube-system
          deviceshare.KnownGeometriesCMName: hami-scheduler-device
```

**Note:** `volcano-vgpu` has its own `KnownGeometriesCMName`/`KnownGeometriesCMNamespace`. If you need both vNPU and vGPU in the same Volcano cluster, merge the ConfigMaps from both sides and reference the merged one here.

## Usage

**Note:** Each Ascend chip model has its own `resourceName`, `resourceMemoryName`, and `resourceCoreName`; see the `hami-scheduler-device` ConfigMap for the full mapping.

**Note:** To exclusively use an entire card or request multiple cards, you only need to set the corresponding resourceName.

**Note:** The device plugin applies **soft slicing** (`libvnpu` / `hami-vnpu-core` mounts and environment) **only** when the Pod sets `huawei.com/vnpu-mode: hami-core`. Pods **without** this annotation still follow the **original vNPU** path (virtualization templates and `ASCEND_VNPU_SPECS`), so on nodes that only expose `hami-vnpu-core` soft-slicing capacity, such Pods may stay **Pending** indefinitely.

Set `schedulerName: volcano` on your Pods so Volcano schedules them. 

### Template vNPU mode

Without the `hami-core` annotation, Pods use the legacy template-based vNPU allocation:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: ascend-pod
spec:
  schedulerName: volcano
  runtimeClassName: ascend
  containers:
    - name: ubuntu-container
      image: swr.cn-south-1.myhuaweicloud.com/ascendhub/ascend-pytorch:24.0.RC1-A2-1.11.0-ubuntu20.04
      command: ["sleep"]
      args: ["100000"]
      resources:
        limits:
          huawei.com/Ascend310P: "1"
          huawei.com/Ascend310P-memory: "4096"
```

### `hami-core` soft slicing mode

`runtimeClassName: ascend` (deployed in [Deployment](#deployment) above) is required for all Ascend Pods regardless of slicing mode. To additionally use `hami-vnpu-core` soft slicing, also add the `huawei.com/vnpu-mode: hami-core` annotation:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: ascend-pod
  annotations:
    huawei.com/vnpu-mode: hami-core
spec:
  schedulerName: volcano
  runtimeClassName: ascend
  containers:
    - name: ubuntu-container
      image: quay.io/ascend/vllm-ascend:v0.18.0-310p
      command: ["sleep"]
      args: ["100000"]
      resources:
        limits:
          huawei.com/Ascend310P: "1"
          huawei.com/Ascend310P-memory: "4096"
          huawei.com/Ascend310P-core: "90"
```

## Monitoring

When a node runs in **hami-vnpu-core (soft slicing) mode**, the device plugin starts an **embedded Prometheus exporter** on **`:9395/metrics`** that reports physical-device and per-container vNPU usage. It is **not** started for the legacy template-based vNPU (or whole-card) path, which has no soft-slice data to export.

Quick check from inside the cluster:

```bash
POD_IP=$(kubectl -n kube-system get pod -l app.kubernetes.io/component=hami-ascend-device-plugin -o jsonpath='{.items[0].status.podIP}')
curl -s $POD_IP:9395/metrics | grep hami_
```

### Exposed metrics

| Metric | Labels | Description |
| :--- | :--- | :--- |
| `hami_host_gpu_memory_used_bytes` | `device_index`, `device_uuid`, `device_type` | Physical NPU memory used (bytes) |
| `hami_host_gpu_utilization_ratio` | `device_index`, `device_uuid`, `device_type` | Physical NPU AICore utilization (0–100) |
| `hami_vgpu_memory_used_bytes` | `namespace`, `pod`, `container`, `vdevice_index`, `device_uuid` | Per-container vNPU memory used (bytes) |
| `hami_vgpu_memory_limit_bytes` | `namespace`, `pod`, `container`, `vdevice_index`, `device_uuid` | Per-container vNPU memory limit (bytes) |
| `hami_container_device_utilization_ratio` | `namespace`, `pod`, `container`, `vdevice_index`, `device_uuid` | AICore utilization of the device the container runs on (0–100) |
