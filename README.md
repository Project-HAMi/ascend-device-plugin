# Ascend Device Plugin

## Introduction

This Ascend device plugin is implemented for [HAMi](https://github.com/Project-HAMi/HAMi) scheduling.

Memory slicing is supported based on virtualization template, lease available template is automatically used. For detailed information, check [templeate](./config.yaml)

## Prequisites

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

Due to dependencies with HAMi, the deployment is integrated into the HAMi deployment, you need to set 'devices.ascend.enabled=true'. The device-plugin is automaticaly deployed. For more details ,see 'devices' section in values.yaml.

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


## Usage

```yaml
...
    containers:
    - name: npu_pod
      ...
      resources:
        limits:
          huawei.com/Ascend910B: "1"
          # 不填写显存默认使用整张卡
          huawei.com/Ascend910B-memory: "4096"
```
