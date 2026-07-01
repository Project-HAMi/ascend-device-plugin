# Ascend Device Plugin
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2FProject-HAMi%2Fascend-device-plugin.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2FProject-HAMi%2Fascend-device-plugin?ref=badge_shield)


## Introduction

This Ascend device plugin is implemented for NPU-Slicing for [HAMi](https://github.com/Project-HAMi/HAMi) and [volcano](https://github.com/volcano-sh/volcano). It supports two modes:

#### 1. Template-based Hard Slicing (vNPU)

Memory slicing is supported based on virtualization template, least available template is automatically used. For detailed information, check [template](https://github.com/Project-HAMi/ascend-device-plugin/blob/main/ascend-device-configmap.yaml)

#### 2. Soft Slicing with Runtime Interception (hami-vnpu-core)

This project implements  a soft slicing mechanism based on `libvnpu.so` interception and `limiter` token scheduling, enabling fine-grained resource sharing.  For detailed information, check [hami-vnpu-core](https://github.com/Project-HAMi/hami-vnpu-core)

**Note 1:** `hami-vnpu-core` currently only supports ARM platforms.
**Note 2:** `hami-vnpu-core` currently only supports HAMi scheduler.

## Prerequisites

[ascend-docker-runtime](https://gitcode.com/Ascend/mind-cluster/tree/master/component/ascend-docker-runtime)

hami-vnpu-core Soft Slicing Requirements:

- **Ascend Driver Version**: ≥ 25.5
- **Chip Mode**: enable `device-share` mode on Ascend chips for virtualization
Below is the English translation of the instructions for enabling `device-share` mode:

**Enabling `device-share` Mode**

**npu-smi set -t device-share -i** *id* **-d** *value* This command is used to set the container sharing mode for all chips on a specified device.

**Parameter Description**

| Type | Description |
| :--- | :--- |
| *id* | **Device ID**. The NPU ID found by running the **npu-smi info -l** command is the device ID. |
| *value* | **Container Enable Status**: Options are disabled or enabled. The default is disabled.<br>0: Disabled<br>1: Enabled |

## Compile

update submodule:

```bash
git submodule update --init --recursive
```

```bash
make all
```

## Deployment

### Label the Node with `ascend=on`

```bash
kubectl label node {ascend-node} ascend=on
```

### Deply RuntimeClass

```bash
kubectl apply -f https://raw.githubusercontent.com/Project-HAMi/ascend-device-plugin/main/ascend-runtimeclass.yaml
```

### Deploy ConfigMap

This configMap is used for global configurations, like resourceName, mode, templates.
* Under `vnpus`, set `hamiVnpuCore: true` so **all nodes** advertise soft-partitioning based on `hami-vnpu-core` to the scheduler (unless overridden per node in `hami-device-node-config`).

```bash
kubectl apply -f https://raw.githubusercontent.com/Project-HAMi/ascend-device-plugin/main/ascend-device-configmap.yaml
```

**Note:** You can choose to ignore this step if this configMap already exists.


#### (Optional) **Node Custom Configuration Description**

The `hami-device-node-config` is used to enable or override hami-vnpu-core for specific nodes within the cluster. Node-level settings take higher priority than the global `vnpus.hamiVnpuCore` switch.

```bash
kubectl apply -f https://raw.githubusercontent.com/Project-HAMi/ascend-device-plugin/main/ascend-device-node-configmap.yaml
```

### Deploy `ascend-device-plugin`

```bash
kubectl apply -f https://raw.githubusercontent.com/Project-HAMi/ascend-device-plugin/main/ascend-device-plugin.yaml
```

## Usage

If you require HAMi to automatically add the `runtimeClassName` configuration to Pods requesting Ascend resources (this is disabled by default), you should set `devices.ascend.runtimeClassName` value to **a non-empty string** in HAMi’s `values.yaml` file, ensuring it matches the name of the `RuntimeClass` resource. For example:

```yaml
devices:
  ascend:
    runtimeClassName: ascend
```

To exclusively use an entire card or request multiple cards, you only need to set the corresponding resourceName. If multiple tasks need to share the same NPU, you need to set the corresponding resource request to 1 and configure the appropriate ResourceMemoryName.

### Usage in HAMi

**How HAMi chooses soft vs legacy vNPU:** The device plugin applies **soft slicing** (`libvnpu` / `hami-vnpu-core` mounts and environment) **only** when the Pod sets `huawei.com/vnpu-mode: hami-core`. Pods **without** this annotation still follow the **original vNPU** path (virtualization templates and `ASCEND_VNPU_SPECS`). These two paths are different. If your cluster effectively has **only** soft-slicing–oriented Ascend capacity (for example every node is configured for `hami-vnpu-core` and workloads are expected to use soft slicing), Pods that **omit** `vnpu-mode=hami-core` may remain **Pending** because they still request the legacy vNPU allocation model, which may not match what those nodes expose or how the scheduler pairs Pods to nodes.

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

### Soft Slicing Configuration (HAMi)

Use the annotation below whenever you intend **soft** slicing; omitting it keeps **template-based vNPU** behavior (see the note under [Usage in HAMi](#usage-in-hami)).

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


### Usage in volcano

Volcano must be installed prior to usage, for more information see [here](https://github.com/volcano-sh/volcano/tree/master/docs/user-guide/how_to_use_vnpu.md)

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: ascend-pod
spec:
  schedulerName: volcano
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

## Monitoring

When a node runs in **hami-vnpu-core (soft slicing) mode**, the device plugin starts an **embedded Prometheus exporter** on **`:9395/metrics`** that reports physical-device and per-container vNPU usage. It is **not** started for the legacy template-based vNPU (or whole-card) path, which has no soft-slice data to export. The DaemonSet declares the `monitorport` (9395) container port; the metrics `Service` and `ServiceMonitor` ship in `ascend-vnpu-monitor-integration.yaml`.

### Exposed metrics

| Metric | Labels | Description |
| :--- | :--- | :--- |
| `hami_host_gpu_memory_used_bytes` | `device_index`, `device_uuid`, `device_type` | Physical NPU memory used (bytes) |
| `hami_host_gpu_utilization_ratio` | `device_index`, `device_uuid`, `device_type` | Physical NPU AICore utilization (0–100) |
| `hami_vgpu_memory_used_bytes` | `namespace`, `pod`, `container`, `vdevice_index`, `device_uuid` | Per-container vNPU memory used (bytes) |
| `hami_vgpu_memory_limit_bytes` | `namespace`, `pod`, `container`, `vdevice_index`, `device_uuid` | Per-container vNPU memory limit (bytes) |
| `hami_container_device_utilization_ratio` | `namespace`, `pod`, `container`, `vdevice_index`, `device_uuid` | AICore utilization of the device the container runs on (0–100) |

Per-container metrics come from the `hami-vnpu-core` soft-slice shmem and require the Pod to carry the device-UUID annotation the plugin writes (`huawei.com/Ascend<type>`), i.e. workloads soft-sliced through this plugin. In soft-slice mode multiple containers share one physical card, so they report that card's AICore utilization.

### Scrape with Prometheus

Quick check via `port-forward` to a plugin Pod:

```bash
POD=$(kubectl -n kube-system get pod -l app.kubernetes.io/component=hami-ascend-device-plugin -o jsonpath='{.items[0].metadata.name}')
kubectl -n kube-system port-forward "$POD" 9395:9395
curl -s localhost:9395/metrics | grep hami_
```

With the Prometheus Operator (kube-prometheus-stack) installed, apply the metrics `Service`, `ServiceMonitor` and recording rules:

```bash
kubectl apply -f https://raw.githubusercontent.com/Project-HAMi/ascend-device-plugin/main/ascend-vnpu-monitor-integration.yaml
```

## License

[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2FProject-HAMi%2Fascend-device-plugin.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2FProject-HAMi%2Fascend-device-plugin?ref=badge_large)