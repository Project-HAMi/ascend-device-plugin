# Ascend Device Plugin

## 说明

基于[HAMi](https://github.com/Project-HAMi/HAMi)调度机制的ascend device plugin。

支持基于显存调度，显存是基于昇腾的虚拟化模板来切分的，会找到满足显存需求的最小模板来作为容器的显存。

启动容器依赖[ascend-docker-runtime](https://gitee.com/ascend/ascend-docker-runtime)。

## 编译

### 编译二进制文件

```bash
make all
```

### 编译镜像

```bash
docker buildx build -t $IMAGE_NAME .
```

## 部署

由于和HAMi的一些依赖关系，部署集成在HAMi的部署中，修改HAMi chart values中的以下部分即可。

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

## 使用

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
