# Ascend Device Plugin

## 说明

Ascend device plugin 是用来支持在 [HAMi](https://github.com/Project-HAMi/HAMi) 和 [volcano](https://github.com/volcano-sh/volcano) 中调度昇腾 NPU 设备，支持以下两种模式：

#### 1. 基于模板的硬切分 (vNPU)

支持基于虚拟化模板的显存切分，系统会自动使用最小可用模板。详细信息请参阅 [template](https://github.com/Project-HAMi/ascend-device-plugin/blob/main/ascend-device-configmap.yaml)。

#### 2. 基于运行时拦截的软切分 (hami-vnpu-core)

实现了基于 `libvnpu.so` 拦截和 limiter 令牌调度的软切分机制，能够实现精细化的资源共享。详细信息请参阅 [hami-vnpu-core](https://github.com/Project-HAMi/hami-vnpu-core)。

**注意 1：** `hami-vnpu-core` 目前只支持 ARM 平台。
**注意 2：** `hami-vnpu-core` 目前只支持 HAMi 调度器。

## 环境要求

部署 [ascend-docker-runtime](https://gitcode.com/Ascend/mind-cluster/tree/master/component/ascend-docker-runtime)

**hami-vnpu-core 软切分要求：**

- Ascend 驱动版本：≥ 25.5
- 芯片模式：在昇腾芯片上开启 `device-share` 模式以支持虚拟化。

**开启 `device-share` 模式**

**npu-smi set -t device-share -i** *id* **-d** *value*  用于设置指定设备的所有芯片的容器共享模式。

**参数说明**

| 类型    | 描述                                                        |
| ------- | ----------------------------------------------------------- |
| *id*    | 设备 ID。通过 **npu-smi info -l** 命令查出的 NPU ID 即为设备 ID。 |
| *value* | 容器使能状态：分为禁用、使能。默认禁用。<br>0：禁用<br>1：使能 |

## 编译

更新子模块：

```bash
git submodule update --init --recursive
```

```bash
make all
```

### 编译镜像

```bash
docker buildx build -t $IMAGE_NAME .
```

## 部署

### 给 Node 打 ascend 标签

```bash
kubectl label node {ascend-node} ascend=on
```

### 部署 RuntimeClass

```bash
kubectl apply -f https://raw.githubusercontent.com/Project-HAMi/ascend-device-plugin/main/ascend-runtimeclass.yaml
```

### 部署 ConfigMap

该 ConfigMap 用于全局配置，包括 resourceName、模式、模板等。
* 在 `vnpus` 下设置 `hamiVnpuCore: true`，**所有节点**会向调度器声明基于 `hami-vnpu-core` 的软切分能力（可被 `hami-device-node-config` 按节点覆盖）。

```bash
kubectl apply -f https://raw.githubusercontent.com/Project-HAMi/ascend-device-plugin/main/ascend-device-configmap.yaml
```

**注意：** 如果该 ConfigMap 已存在，可跳过此步骤。

#### （可选）节点自定义配置说明

`hami-device-node-config` 用于对集群中特定节点的 hami-vnpu-core 进行启用或覆盖。节点级配置的优先级高于全局 `vnpus.hamiVnpuCore` 开关。

同时支持 `filterDevices`，用于配置某个节点上 HAMi 需要忽略的设备。默认情况下 `filterDevices` 为空，表示不忽略任何设备。当设备 UUID 在 `uuid` 列表中，或设备索引在 `index` 列表中时，该设备会被 HAMi 忽略，例如：`filterDevices: {index: [0, 1], uuid: []}`。

```bash
kubectl apply -f https://raw.githubusercontent.com/Project-HAMi/ascend-device-plugin/main/ascend-device-node-configmap.yaml
```

### 部署 `ascend-device-plugin`

```bash
kubectl apply -f https://raw.githubusercontent.com/Project-HAMi/ascend-device-plugin/main/ascend-device-plugin.yaml
```

如果要在 HAMi 中使用昇腾 NPU，在部署 HAMi 时设置 `devices.ascend.enabled` 为 true 会自动部署 ConfigMap 和 `ascend-device-plugin`。参考 <https://github.com/Project-HAMi/HAMi/blob/master/charts/hami/README.md#huawei-ascend>

如果需要 HAMi 为申请 ascend 资源的 Pod 自动添加 runtimeClassName 配置（默认关闭），则应该在 HAMi 的 values.yaml 文件中配置 `devices.ascend.runtimeClassName` 为**一个非空字符串**，并且与 RuntimeClass 资源名称保持一致。例如：

```yaml
devices:
  ascend:
    runtimeClassName: ascend
```

## 使用

如果要独占整卡或者申请多张卡只需要设置对应的 resourceName 即可。如果多个任务要共享同一张卡，需要将 resourceName 设置为 1，并且设置对应的 ResourceMemoryName。

### 在 HAMi 中使用

**HAMi 与 vNPU 模式说明：** 只有为 Pod 配置了注解 `huawei.com/vnpu-mode: hami-core` 时，设备插件才会按 **软切分**（`libvnpu` / `hami-vnpu-core` 的挂载与环境变量）处理。**未添加**该注解的任务仍走 **原有 vNPU** 方案（虚拟化模板与 `ASCEND_VNPU_SPECS` 等）。两种路径不同。当集群里 Ascend 节点 **只有** 面向软切分的部署或调度预期（例如节点均按 `hami-vnpu-core` 配置、工作负载预期都使用软切分）时，**未**设置 `vnpu-mode=hami-core` 的任务可能一直处于 **Pending**，因为其仍按旧版 vNPU 申请与分配逻辑，可能与当前节点暴露的资源或调度匹配方式不一致。

```yaml
...
    containers:
    - name: npu_pod
      ...
      resources:
        limits:
          huawei.com/Ascend910B: "1"
          # 如果不指定显存大小，就会使用整张卡
          huawei.com/Ascend910B-memory: "4096"
```

更多示例请参阅 [examples](https://github.com/Project-HAMi/ascend-device-plugin/tree/main/examples)

### 软切分配置 (HAMi)

需要 **软切分** 时请显式加上下文中的注解；不加则仍为 **模板硬切分 vNPU**（与上一节「在 HAMi 中使用」中的说明一致）。

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: ascend-soft-slice-pod
  annotations:
    huawei.com/vnpu-mode: 'hami-core' # 添加该注解的走 hami-vnpu-core 软切分
spec:
  containers:
    - name: npu_pod
      ...
      resources:
        limits:
          huawei.com/Ascend910B3: "1"           # 请求 1 块物理 NPU
          huawei.com/Ascend910B3-memory: "28672" # 请求 28Gi 显存
          huawei.com/Ascend910B3-core: "40"      # 请求 40% 的算力
```

软切分机制支持在单个 Pod 中申请多个虚拟设备。在进行多卡并行推理（如使用 vLLM）时，`--gpu-memory-utilization` 的值不能大于"容器总显存上限"占"所选卡物理显存总和"的比例。

**示例：使用 vLLM 开启 2 卡张量并行 (TP=2)**

假设单块物理卡显存为 **64Gi**，计划在 2 块卡上各使用 **32Gi**（总计 64Gi）：

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: vllm-npu-2card
  annotations:
    huawei.com/vnpu-mode: 'hami-core' # 启用 hami-vnpu-core 软切分
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
          --gpu-memory-utilization 0.5   # 关键参数：总申请显存 64Gi / 总物理显存 128Gi = 0.5
      resources:
        limits:
          huawei.com/Ascend910B3: "2"           # 申请 2 块虚拟设备进行并行计算
          huawei.com/Ascend910B3-memory: "65536" # 容器可用的总显存上限（2 卡合计 64GiB）
          huawei.com/Ascend910B3-core: "50"
```

### 在 volcano 中使用

在 volcano 中使用时需要提前部署好 volcano，更多信息请[参考这里](https://github.com/volcano-sh/volcano/tree/master/docs/user-guide/how_to_use_vnpu.md)

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

## 监控

当节点运行在 **hami-vnpu-core(软切)模式**时,设备插件会在 **`:9395/metrics`** 启动内置 **Prometheus exporter**,上报物理设备级和每容器的 vNPU 使用指标。传统的模板 vNPU(或整卡)模式**不会**启动它——那种模式没有软切数据可导出。DaemonSet 声明了 `monitorport`(9395)容器端口;metrics `Service` 和 `ServiceMonitor` 放在 `ascend-vnpu-monitor-integration.yaml` 里。

### 暴露的指标

| 指标 | 标签 | 说明 |
| :--- | :--- | :--- |
| `hami_host_gpu_memory_used_bytes` | `device_index`, `device_uuid`, `device_type` | 物理 NPU 已用显存(字节) |
| `hami_host_gpu_utilization_ratio` | `device_index`, `device_uuid`, `device_type` | 物理 NPU AICore 利用率(0–100) |
| `hami_vgpu_memory_used_bytes` | `namespace`, `pod`, `container`, `vdevice_index`, `device_uuid` | 每容器 vNPU 已用显存(字节) |
| `hami_vgpu_memory_limit_bytes` | `namespace`, `pod`, `container`, `vdevice_index`, `device_uuid` | 每容器 vNPU 显存上限(字节) |
| `hami_container_device_utilization_ratio` | `namespace`, `pod`, `container`, `vdevice_index`, `device_uuid` | 容器所在设备的 AICore 利用率(0–100) |

每容器指标来自 `hami-vnpu-core` 软切的共享内存,依赖插件写入的设备 UUID 注解(`huawei.com/Ascend<型号>`),即通过本插件软切的负载。软切模式下多个容器共享同一张物理卡,因此它们上报的是该卡的 AICore 利用率。

### 用 Prometheus 抓取

通过 `port-forward` 到插件 Pod 快速验证:

```bash
POD=$(kubectl -n kube-system get pod -l app.kubernetes.io/component=hami-ascend-device-plugin -o jsonpath='{.items[0].metadata.name}')
kubectl -n kube-system port-forward "$POD" 9395:9395
curl -s localhost:9395/metrics | grep hami_
```

已安装 Prometheus Operator(kube-prometheus-stack)时,应用 metrics `Service`、`ServiceMonitor` 和录制规则:

```bash
kubectl apply -f https://raw.githubusercontent.com/Project-HAMi/ascend-device-plugin/main/ascend-vnpu-monitor-integration.yaml
```
