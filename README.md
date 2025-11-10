# Ascend Device Plugin

## Introduction

This Ascend device plugin is implemented for [HAMi](https://github.com/Project-HAMi/HAMi) and [volcano](https://github.com/volcano-sh/volcano) scheduling.

Memory slicing is supported based on virtualization template, lease available template is automatically used. For detailed information, check [template](./ascend-device-configmap.yaml)

## Prerequisites

[ascend-docker-runtime](https://gitcode.com/Ascend/mind-cluster/tree/master/component/ascend-docker-runtime)

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


```
kubectl label node {ascend-node} ascend=on
``` 

### Deploy ConfigMap

```
kubectl apply -f ascend-device-configmap.yaml
```

### Deploy `ascend-device-plugin`

```bash
kubectl apply -f ascend-device-plugin.yaml
```

If scheduling Ascend devices in HAMi, simply set `devices.ascend.enabled` to true when deploying HAMi, and the ConfigMap and `ascend-device-plugin` will be automatically deployed. refer https://github.com/Project-HAMi/HAMi/blob/master/charts/hami/README.md#huawei-ascend

## Usage

You can allocate a slice of NPU by specifying both resource number and resource memory. If multiple tasks need to share the same NPU, you need to set the corresponding resource request to 1 and configure the appropriate ResourceMemoryName.

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