# Deploy & Use with HAMi

**English** | [中文](hami_cn.md)

This guide covers deploying `ascend-device-plugin` for use with the [HAMi](https://github.com/Project-HAMi/HAMi) scheduler.

## Prerequisites

[ascend-docker-runtime](https://gitcode.com/Ascend/mind-cluster/tree/master/component/ascend-docker-runtime)

- **Ascend Driver Version**: ≥ 25.5
- **`npu-smi` must be reachable on the host**, at one of the following paths (checked in order):
  1. `/usr/local/Ascend/driver/tools/npu-smi` (from the existing driver hostPath mount)
  2. `/usr/local/sbin/npu-smi`
  3. `/usr/local/bin/npu-smi`

  If your host ships `npu-smi` elsewhere, add a hostPath mount for it in `ascend-device-plugin.yaml`.

- **HAMi Version**:
  - ≥ 2.7.0 for template-based hard slicing (vNPU)
  - ≥ 2.9.0 for `hami-core` soft slicing (hami-vnpu-core)

  Both require `devices.ascend.enabled: true` to be set when deploying HAMi.

## Deployment

### Label the Node with `ascend=on`

```bash
kubectl label node {ascend-node} ascend=on
```

### Deploy RuntimeClass

```bash
kubectl apply -f https://raw.githubusercontent.com/Project-HAMi/ascend-device-plugin/main/ascend-runtimeclass.yaml
```

### Deploy ConfigMap

* **HAMi and `ascend-device-plugin` in the same namespace** (recommended): skip this step — HAMi's existing `hami-scheduler-device` ConfigMap already covers Ascend.
* **Different namespaces**:
  1. Deploy `ascend-device-configmap.yaml` into `ascend-device-plugin`'s own namespace.
  2. Manually merge its `vnpus:` section into HAMi's existing `hami-scheduler-device` ConfigMap, without touching HAMi's other device entries.
  3. Keep both copies in sync whenever you change templates, resourceNames, or `hamiVnpuCore`.

```bash
kubectl apply -f https://raw.githubusercontent.com/Project-HAMi/ascend-device-plugin/main/ascend-device-configmap.yaml
```

**Note:** `vnpus.hamiVnpuCore` decides the slicing mode for **all nodes** (unless overridden per node in `hami-device-node-config`): `true` uses `hami-core`-based **soft slicing**; `false` uses template-based **hard slicing**.

#### (Optional) **Node Custom Configuration Description**

The `hami-device-node-config` is used to enable or override hami-vnpu-core for specific nodes within the cluster. Node-level settings take higher priority than the global `vnpus.hamiVnpuCore` switch.

It also supports `filterDevices` to configure devices ignored by HAMi on a specific node. By default, `filterDevices` is empty, which means no devices are ignored. A device is ignored when its UUID is listed in `uuid` or its index is listed in `index`, for example: `filterDevices: {index: [0, 1], uuid: []}`.

```bash
kubectl apply -f https://raw.githubusercontent.com/Project-HAMi/ascend-device-plugin/main/ascend-device-node-configmap.yaml
```

### Deploy `ascend-device-plugin`

```bash
kubectl apply -f https://raw.githubusercontent.com/Project-HAMi/ascend-device-plugin/main/ascend-device-plugin.yaml
```

## Usage

**Note:** Each Ascend chip model has its own `resourceName`, `resourceMemoryName`, and `resourceCoreName`; see the `hami-scheduler-device` ConfigMap for the full mapping.

**Note:** To exclusively use an entire card or request multiple cards, you only need to set the corresponding resourceName.

**Note:** The device plugin applies **soft slicing** (`libvnpu` / `hami-vnpu-core` mounts and environment) **only** when the Pod sets `huawei.com/vnpu-mode: hami-core`. Pods **without** this annotation still follow the **original vNPU** path (virtualization templates and `ASCEND_VNPU_SPECS`), so on nodes that only expose `hami-vnpu-core` soft-slicing capacity, such Pods may stay **Pending** indefinitely.

```yaml
...
metadata:
  name: ascend-soft-slice-pod
  annotations:
    huawei.com/vnpu-mode: 'hami-core' # Enables hami-vnpu-core soft-segmentation for this pod
    containers:
    - name: npu_pod
      ...
      resources:
        limits:
          huawei.com/Ascend910B: "1"
          # if you don't specify Ascend910B-memory, it will use a whole NPU.
          huawei.com/Ascend910B-memory: "4096"
```

For more examples, see [examples](https://github.com/Project-HAMi/ascend-device-plugin/tree/main/examples)

### Soft Slicing Configuration (hami-vnpu-core)

Use the annotation below whenever you intend **soft** slicing; omitting it keeps **template-based vNPU** behavior (see the note above).

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: ascend-soft-slice-pod
  annotations:
    huawei.com/vnpu-mode: 'hami-core' # Enables hami-vnpu-core soft-segmentation for this pod
spec:
  containers:
    - name: npu_pod
      ...
      resources:
        limits:
          huawei.com/Ascend910B3: "1"           # Request 1 physical NPU
          huawei.com/Ascend910B3-memory: "28672"     # Request 28Gi memory
          huawei.com/Ascend910B3-core: "40"     # Request 40% core
```

The soft partitioning mechanism supports requesting multiple virtual devices within a same Pod. When performing multi-card parallel inference (e.g., using vLLM), the value of `--gpu-memory-utilization` must not exceed the ratio of the "container's total memory limit" to the "sum of physical memory of the selected cards".

**Example: Enabling 2-Card Tensor Parallelism (TP=2) with vLLM**

Assume each physical card has **64Gi** of memory, and you plan to use **32Gi** on each of the 2 cards (totaling 64Gi):

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: vllm-npu-2card
  annotations:
    huawei.com/vnpu-mode: 'hami-core' # Enable hami-vnpu-core soft partitioning
spec:
  containers:
    - name: vllm-container
      image: vllm-ascend:latest
      command: ["/bin/sh", "-c"]
      args:
        - |
          vllm serve /model/Qwen3-0.6B \
          --host 0.0.0.0 \
          --port 8002 \
          --enforce-eager \
          --tensor-parallel-size 2 \
          --gpu-memory-utilization 0.5   # Key parameter: Total requested memory 64Gi / Total physical memory 128Gi = 0.5
      resources:
        limits:
          huawei.com/Ascend910B3: "2"           # Request 2 virtual devices for parallel computation
          huawei.com/Ascend910B3-memory: "65536" # Total memory limit for the container (64GiB combined across 2 cards)
          huawei.com/Ascend910B3-core: "50"
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
