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
* By setting `hamiVnpuCore: true` at the top level, **all nodes** will enable soft-partitioning based on `hami-vnpu-core`.

```bash
kubectl apply -f https://raw.githubusercontent.com/Project-HAMi/ascend-device-plugin/main/ascend-device-configmap.yaml
```

**Note:** You can choose to ignore this step if this configMap already exists.


#### (Optional) **Node Custom Configuration Description**

The `hami-device-node-config` is used to enable or override hami-vnpu-core for specific nodes within the cluster. Node-level settings take higher priority than the global `hamiVnpuCore` switch.

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

```yaml
...
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

## License

[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2FProject-HAMi%2Fascend-device-plugin.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2FProject-HAMi%2Fascend-device-plugin?ref=badge_large)