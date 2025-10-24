# Ascend Device Plugin

## Introduction

This Ascend device plugin is implemented for [HAMi](https://github.com/Project-HAMi/HAMi) scheduling.

Memory slicing is supported based on virtualization template, lease available template is automatically used. For detailed information, check [templeate](./config.yaml)

## Prerequisites

[ascend-docker-runtime](https://gitee.com/ascend/ascend-docker-runtime)

## Compile

```bash
make all
```

### Build

```bash
docker buildx build -t $IMAGE_NAME .
```

## Deployment

Due to dependencies with HAMi, you need to set 

```
devices.ascend.enabled=true
``` 

during HAMi installation. For more details, see 'devices' section in values.yaml.

```yaml
devices:
  ascend:
    enabled: true
    image: "ascend-device-plugin:master"
    imagePullPolicy: IfNotPresent
    extraArgs: []
    nodeSelector:
      ascend: "on"
    tolerations: []
    resources:
      - huawei.com/Ascend910A
      - huawei.com/Ascend910A-memory
      - huawei.com/Ascend910B
      - huawei.com/Ascend910B-memory
      - huawei.com/Ascend310P
      - huawei.com/Ascend310P-memory
```

Note that resources here(hawei.com/Ascend910A,huawei.com/Ascend910B,...) is managed in hami-scheduler-device configMap. It defines three different templates(910A,910B,310P).

label your NPU nodes with 'ascend=on'

```
kubectl label node {ascend-node} ascend=on
```

Deploy ascend-device-plugin by running

```bash
kubectl apply -f ascend-device-plugin.yaml
```


## Usage

You can allocate a slice of NPU by specifying both resource number and resource memory. For more examples, see [examples](./examples/)

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
