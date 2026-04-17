# Ascend Device Plugin

## 说明

Ascend device plugin 是用来支持在 [HAMi](https://github.com/Project-HAMi/HAMi) 和 [volcano](https://github.com/volcano-sh/volcano) 中调度昇腾NPU设备.

**基于模板的硬切分 (vNPU)** 

支持基于虚拟化模板的显存切分，系统会自动使用最小可用模板。详细信息请参阅 [template](https://www.google.com/search?q=链接)。

**基于运行时拦截的软切分 (hami-vnpu-core)** 

实现了基于 `libvnpu.so` 拦截和limiter令牌调度的软切分机制，能够实现精细化的资源共享。详细信息请参阅 [hami-vnpu-core](https://www.google.com/search?q=链接)。

## 环境要求

部署 [ascend-docker-runtime](https://gitcode.com/Ascend/mind-cluster/tree/master/component/ascend-docker-runtime)

更新子模块 

```bash
git submodule update --init --recursive
```

**hami-vnpu-core 软切分要求：**

- Ascend 驱动版本：≥ 25.5
- 芯片模式：在昇腾芯片上开启 `device-share` 模式以支持虚拟化。

## 编译

```bash
make all
```

### 编译镜像

```bash
docker buildx build -t $IMAGE_NAME .
```

### 宿主机环境准备

在启动任何容器之前，必须在宿主机上初始化 **全局共享内存 (SHM) 区域**，以便进行 Pod 间的协同。

1. **创建共享目录**

   ```
   sudo mkdir -p /tmp/hami-shared-region
   sudo chmod 777 /tmp/hami-shared-region
   ```

2. **部署 hami-vnpu-core 组件** 

   将以下文件放置在固定的宿主机路径（`/usr/local/hami-vnpu-core/`）中，以便挂载到容器内： 

   ```
   /usr/local/hami-vnpu-core/
   ├── limiter              # Manager daemon binary (compiled from hami-vnpu-core)
   ├── libvnpu.so           # Interception library for LD_PRELOAD
   └── ld.so.preload        # Global preload config 
   ```

## 部署

### 给 Node 打 ascend 标签

```bash
kubectl label node {ascend-node} ascend=on
```

### 部署 ConfigMap

```bash
kubectl apply -f ascend-device-configmap.yaml
```

### 部署 RuntimeClass

```bash
kubectl apply -f ascend-runtimeclass.yaml
```

### 部署 `ascend-device-plugin`

```bash
kubectl apply -f ascend-device-plugin.yaml
```

如果要在HAMi中使用升腾NPU, 在部署HAMi时设置 `devices.ascend.enabled` 为 true 会自动部署 ConfigMap 和 `ascend-device-plugin`。 参考 <https://github.com/Project-HAMi/HAMi/blob/master/charts/hami/README.md#huawei-ascend>

如果需要 HAMi 为申请 ascend 资源的 Pod 自动添加 runtimeClassName 配置（默认关闭），则应该在 HAMi 的 values.yaml 文件中配置 `deivces.ascend.runtimeClassName` 为**一个非空字符串**，并且与 RuntimeClass 资源名称保持一致。 例如：

```yaml
devices:
  ascend:
    runtimeClassName: ascend
```

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

#### 软切分配置 (HAMi)

YAML

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: ascend-soft-slice-pod
  annotations:
    huawei.com/vnpu-mode: 'hami-core' # 添加该注解的走hami-vnpu-core软切分
spec:
  containers:
    - name: npu_pod
      ...
      resources:
        limits:
          huawei.com/Ascend910B3: "1"          # 请求 1 块物理 NPU
          huawei.com/Ascend910B3-memory: "28672" # 请求 28Gi 显存
          huawei.com/Ascend910B3-core: "40"      # 请求 40% 的算力
```



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
