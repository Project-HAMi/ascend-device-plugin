# Ascend Device Plugin

## 说明

Ascend device plugin 是用来支持在 [HAMi](https://github.com/Project-HAMi/HAMi) 和 [volcano](https://github.com/volcano-sh/volcano) 中调度昇腾NPU设备.

昇腾NPU虚拟化切分是通过模板来配置的，在调度时会找到满足显存需求的最小模板来作为容器的显存。各芯片的模板配置信息参考[这里](./ascend-device-configmap.yaml)

## 环境要求

部署 [ascend-docker-runtime](https://gitcode.com/Ascend/mind-cluster/tree/master/component/ascend-docker-runtime)

克隆子模块 mind-cluster
```bash
git submodule add https://gitcode.com/Ascend/mind-cluster.git
```

## 编译

```bash
make all
```

### 编译镜像

```bash
docker buildx build -t $IMAGE_NAME .
```

## 部署

### 给 Node 打 ascend 标签


```
kubectl label node {ascend-node} ascend=on
```

### 部署 ConfigMap

```
kubectl apply -f ascend-device-configmap.yaml
```

### 部署 `ascend-device-plugin`

```bash
kubectl apply -f ascend-device-plugin.yaml
```

如果要在HAMi中使用升腾NPU, 在部署HAMi时设置 `devices.ascend.enabled` 为 true 会自动部署 ConfigMap 和 `ascend-device-plugin`。 参考 https://github.com/Project-HAMi/HAMi/blob/master/charts/hami/README.md#huawei-ascend

## 使用

如果要独占整卡或者申请多张卡只需要设置对应的 resourceName 即可。如果多个任务要共享同一张卡，需要将 resourceName 设置为1，并且设置对应的 ResourceMemoryName。

### 在 HAMi 中使用

```yaml
...
    containers:
    - name: npu_pod
      ...
      resources:
        limits:
          huawei.com/Ascend910B: "1"
          # 如果不指定显存大小, 就会使用整张卡
          huawei.com/Ascend910B-memory: "4096"
```
 For more examples, see [examples](./examples/)

 ### 在 volcano 中使用

 在 volcano 中使用时需要提前部署好 volcano, 更多信息请[参考这里](https://github.com/volcano-sh/volcano/tree/master/docs/user-guide/how_to_use_vnpu.md)

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