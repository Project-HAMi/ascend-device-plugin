# 在 HAMi 中部署与使用

[English](hami.md) | **中文**

本文档介绍如何在 [HAMi](https://github.com/Project-HAMi/HAMi) 调度器下部署和使用 `ascend-device-plugin`。

## 环境要求

部署 [ascend-docker-runtime](https://gitcode.com/Ascend/mind-cluster/tree/master/component/ascend-docker-runtime)

- Ascend 驱动版本：≥ 25.5
- **`npu-smi` 必须在宿主机上可访问**，按以下顺序查找：
  1. `/usr/local/Ascend/driver/tools/npu-smi`（由已有的 driver hostPath 挂载提供）
  2. `/usr/local/sbin/npu-smi`
  3. `/usr/local/bin/npu-smi`

  若宿主机的 `npu-smi` 在其他位置，可在 `ascend-device-plugin.yaml` 中为其增加 hostPath 挂载。

- **HAMi 版本**：
  - 基于模板的硬切分 (vNPU) 最低版本：≥ 2.7.0
  - `hami-core` 软切分 (hami-vnpu-core) 最低版本：≥ 2.9.0

  两种模式都需要在部署 HAMi 时设置 `devices.ascend.enabled: true`。

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

* **HAMi 和 `ascend-device-plugin` 在同一命名空间**（推荐，默认 `kube-system`）：跳过这一步，HAMi 现有的 `hami-scheduler-device` 已经包含 Ascend 配置。
* **不同命名空间**：
  1. 把 `ascend-device-configmap.yaml` 部署到 `ascend-device-plugin` 自己的命名空间下。
  2. 手动把其中的 `vnpus:` 部分合并进 HAMi 现有的 `hami-scheduler-device`，不要动 HAMi 其他设备的配置。
  3. 以后修改模板、resourceName 或 `hamiVnpuCore` 时，两边同步更新。

**注意：** `vnpus.hamiVnpuCore` 决定了**所有节点**的切分方式（可被 `hami-device-node-config` 按节点覆盖）：`true` 为基于 `hami-core` 的**软切分**；`false` 为基于模板的**硬切分**。

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

如上所述，若 HAMi 和 `ascend-device-plugin` 部署在同一个命名空间，在部署 HAMi 时设置 `devices.ascend.enabled` 为 true，除了会自动部署 ConfigMap 之外，还会一并自动部署 `ascend-device-plugin`。参考 <https://github.com/Project-HAMi/HAMi/blob/master/charts/hami/README.md#huawei-ascend>

如果需要 HAMi 为申请 ascend 资源的 Pod 自动添加 runtimeClassName 配置（默认关闭），则应该在 HAMi 的 values.yaml 文件中配置 `devices.ascend.runtimeClassName` 为**一个非空字符串**，并且与 RuntimeClass 资源名称保持一致。例如：

```yaml
devices:
  ascend:
    runtimeClassName: ascend
```

## 使用

**注意：** 每种 Ascend 芯片型号都有各自对应的 `resourceName`、`resourceMemoryName`、`resourceCoreName`，完整对应关系请参考 `hami-scheduler-device` ConfigMap。

**注意：** 如果要独占整卡或者申请多张卡只需要设置对应的 resourceName 即可。如果多个任务要共享同一张卡，需要将 resourceName 设置为 1，并且设置对应的 ResourceMemoryName。

**注意：** 只有为 Pod 配置了注解 `huawei.com/vnpu-mode: hami-core` 时，设备插件才会按 **软切分**（`libvnpu` / `hami-vnpu-core` 的挂载与环境变量）处理。**未添加**该注解的任务仍走 **原有 vNPU** 方案（虚拟化模板与 `ASCEND_VNPU_SPECS` 等），因此在只暴露 `hami-vnpu-core` 软切分能力的节点上，这类任务可能会一直处于 **Pending**。

```yaml
...
metadata:
  name: ascend-soft-slice-pod
  annotations:
    huawei.com/vnpu-mode: 'hami-core' # 添加该注解的走 hami-vnpu-core 软切分
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

### 软切分配置 (hami-vnpu-core)

需要 **软切分** 时请显式加上下文中的注解；不加则仍为 **模板硬切分 vNPU**（与上一节说明一致）。

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

## 监控

当节点运行在 **hami-vnpu-core(软切)模式**时,设备插件会在 **`:9395/metrics`** 启动内置 **Prometheus exporter**,上报物理设备级和每容器的 vNPU 使用指标。传统的模板 vNPU(或整卡)模式**不会**启动它——那种模式没有软切数据可导出。

在集群内部快速验证：

```bash
POD_IP=$(kubectl -n kube-system get pod -l app.kubernetes.io/component=hami-ascend-device-plugin -o jsonpath='{.items[0].status.podIP}')
curl -s $POD_IP:9395/metrics | grep hami_
```

### 暴露的指标

| 指标 | 标签 | 说明 |
| :--- | :--- | :--- |
| `hami_host_gpu_memory_used_bytes` | `device_index`, `device_uuid`, `device_type` | 物理 NPU 已用显存(字节) |
| `hami_host_gpu_utilization_ratio` | `device_index`, `device_uuid`, `device_type` | 物理 NPU AICore 利用率(0–100) |
| `hami_vgpu_memory_used_bytes` | `namespace`, `pod`, `container`, `vdevice_index`, `device_uuid` | 每容器 vNPU 已用显存(字节) |
| `hami_vgpu_memory_limit_bytes` | `namespace`, `pod`, `container`, `vdevice_index`, `device_uuid` | 每容器 vNPU 显存上限(字节) |
| `hami_container_device_utilization_ratio` | `namespace`, `pod`, `container`, `vdevice_index`, `device_uuid` | 容器所在设备的 AICore 利用率(0–100) |
