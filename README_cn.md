# Ascend Device Plugin

## 说明

基于[HAMi](https://github.com/Project-HAMi/HAMi)调度机制的ascend device plugin。

支持基于显存调度，显存是基于昇腾的虚拟化模板来切分的，会找到满足显存需求的最小模板来作为容器的显存。模版的具体信息参考[配置模版](./config.yaml)

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

由于和HAMi的一些依赖关系，部署集成在HAMi的部署中，指定以下字段：

```
devices.ascend.enabled=true
``` 

相关的每一种NPU设备的资源名，参考values.yaml中的以下字段，目前本组件支持3种型号的NPU切片（310p,910A,910B）若不需要修改的话可以直接使用以下的默认配置：

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

将集群中的NPU节点打上如下标签：

```
kubectl label node {ascend-node} ascend=on
```

最后使用以下指令部署ascend-device-plugin

```bash
kubectl apply -f ascend-device-plugin.yaml
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
