# Ascend Device Plugin
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2FProject-HAMi%2Fascend-device-plugin.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2FProject-HAMi%2Fascend-device-plugin?ref=badge_shield)


## Introduction

This Ascend device plugin is implemented for [HAMi](https://github.com/Project-HAMi/HAMi) and [volcano](https://github.com/volcano-sh/volcano) scheduling.

#### 1. Template-based Hard Slicing (vNPU)

Memory slicing is supported based on virtualization template, lease available template is automatically used. For detailed information, check [template](./ascend-device-configmap.yaml)

#### 2. Soft Slicing with Runtime Interception (hami-vnpu-core)

This project implements  a soft slicing mechanism based on `libvnpu.so` interception and `limiter` token scheduling, enabling fine-grained resource sharing.  For detailed information, check [hami-vnpu-core](https://github.com/Project-HAMi/hami-vnpu-core)

## Prerequisites

[ascend-docker-runtime](https://gitcode.com/Ascend/mind-cluster/tree/master/component/ascend-docker-runtime)

update submodule:

```bash
git submodule update --init --recursive
```

hami-vnpu-core Soft Slicing Requirements:

- **Ascend Driver Version**: ≥ 25.5
- **Chip Mode**: enable `device-share` mode on Ascend chips for virtualization

## Compile

```bash
make all
```

### Build

```bash
docker buildx build -t $IMAGE_NAME .
```

## Deployment

### Label the Node with `ascend=on`

```bash
kubectl label node {ascend-node} ascend=on
```

### Deploy ConfigMap

```bash
kubectl apply -f ascend-device-configmap.yaml
```

### Deply RuntimeClass

```bash
kubectl apply -f ascend-runtimeclass.yaml
```

### Deploy `ascend-device-plugin`

```bash
kubectl apply -f ascend-device-plugin.yaml
```

If scheduling Ascend devices in HAMi, simply set `devices.ascend.enabled` to true when deploying HAMi, and the ConfigMap and `ascend-device-plugin` will be automatically deployed. refer https://github.com/Project-HAMi/HAMi/blob/master/charts/hami/README.md#huawei-ascend

If you require HAMi to automatically add the `runtimeClassName` configuration to Pods requesting Ascend resources (this is disabled by default), you should set `devices.ascend.runtimeClassName` value to **a non-empty string** in HAMi’s `values.yaml` file, ensuring it matches the name of the `RuntimeClass` resource. For example:

```yaml
devices:
  ascend:
    runtimeClassName: ascend
```

## Usage

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

For more examples, see [examples](./examples/)

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