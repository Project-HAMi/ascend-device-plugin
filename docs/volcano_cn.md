# 在 Volcano 中部署与使用

[English](volcano.md) | **中文**

本文档介绍如何通过 Volcano 的 `HAMi mode`（`deviceshare` 插件）部署和使用 `ascend-device-plugin`。背景信息请参阅 Volcano 官方的 [Ascend vNPU 使用指南](https://github.com/volcano-sh/volcano/blob/master/docs/user-guide/how_to_use_vnpu.md)。

## 环境要求

- **Kubernetes**：≥ 1.20
- **Volcano**：≥ 1.14（`hami-core` 软切分需要 ≥ 1.16）
- [ascend-docker-runtime](https://gitcode.com/Ascend/mind-cluster/tree/master/component/ascend-docker-runtime)
- Ascend 驱动版本：≥ 25.5
- **`npu-smi` 必须在宿主机上可访问**（仅 `hami-core` 软切分需要），按以下顺序查找：
  1. `/usr/local/Ascend/driver/tools/npu-smi`（由已有的 driver hostPath 挂载提供）
  2. `/usr/local/sbin/npu-smi` (默认挂载路径)
  3. `/usr/local/bin/npu-smi`

  若宿主机的 `npu-smi` 在 `/usr/local/bin/npu-smi`，需要在 `ascend-device-plugin.yaml` 中为其增加 hostPath 挂载。

**注意：** `hami-core` 软切分目前仅支持 ARM 平台；基于模板的硬切分没有此限制。

## 部署

### 安装 Volcano

若集群中尚未安装 Volcano，请参考 [Volcano 安装指南](https://github.com/volcano-sh/volcano?tab=readme-ov-file#quick-start-guide)。

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
* （可选）在 `vnpus` 下设置 `hamiVnpuCore: true`，即可在**所有节点**上启用 `hami-vnpu-core` 软切分（可被 `hami-device-node-config` 按节点覆盖）。

```bash
kubectl apply -f https://raw.githubusercontent.com/Project-HAMi/ascend-device-plugin/main/ascend-device-configmap.yaml
```

#### （可选）节点自定义配置

`hami-device-node-config` 用于对集群中特定节点的 hami-vnpu-core 进行启用或覆盖，节点 `hamiVnpuMode` 优先于全局 `vnpus.hamiVnpuMode`，未设置时继承全局，同时支持 `filterDevices` 忽略节点上的特定设备，例如 `filterDevices: {index: [0, 1], uuid: []}`。

```bash
kubectl apply -f https://raw.githubusercontent.com/Project-HAMi/ascend-device-plugin/main/ascend-device-node-configmap.yaml
```

### 部署 `ascend-device-plugin`

```bash
kubectl apply -f https://raw.githubusercontent.com/Project-HAMi/ascend-device-plugin/main/ascend-device-plugin.yaml
```

### 更新 Volcano 调度器配置

在 `volcano-scheduler-configmap` 中为 `deviceshare` 插件开启 Ascend HAMi vNPU 支持：

```yaml
kind: ConfigMap
apiVersion: v1
metadata:
  name: volcano-scheduler-configmap
  namespace: volcano-system
data:
  volcano-scheduler.conf: |
    actions: "enqueue, allocate, backfill"
    tiers:
    - plugins:
      - name: predicates
      - name: deviceshare
        arguments:
          deviceshare.AscendHAMiVNPUEnable: true   # 开启 ascend vnpu
          deviceshare.SchedulePolicy: binpack       # 调度策略：binpack / spread
          deviceshare.KnownGeometriesCMNamespace: kube-system
          deviceshare.KnownGeometriesCMName: hami-scheduler-device
```

**注意：** `volcano-vgpu` 有自己的 `KnownGeometriesCMName`/`KnownGeometriesCMNamespace`。如果同一个 Volcano 集群里既要用 vNPU 又要用 vGPU，需要把两边的 ConfigMap 合并后在此引用合并后的 ConfigMap。

## 使用

**注意：** 每种 Ascend 芯片型号都有各自对应的 `resourceName`、`resourceMemoryName`、`resourceCoreName`，完整对应关系请参考 `hami-scheduler-device` ConfigMap。

在 Pod 上设置 `schedulerName: volcano` 以便由 Volcano 调度。

### 模板硬切分 vNPU 模式

不加 `hami-core` 注解时，Pod 走原有的模板 vNPU 分配方式：

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: ascend-pod
spec:
  schedulerName: volcano
  runtimeClassName: ascend
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

### `hami-core` 软切分模式

无论使用哪种切分方式，都需要加上 `runtimeClassName: ascend`（对应上文[部署](#部署)中部署的 RuntimeClass）。如果还想使用 `hami-vnpu-core` 软切分，再额外添加 `huawei.com/vnpu-mode: hami-core` 注解：

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: ascend-pod
  annotations:
    huawei.com/vnpu-mode: hami-core
spec:
  schedulerName: volcano
  runtimeClassName: ascend
  containers:
    - name: ubuntu-container
      image: quay.io/ascend/vllm-ascend:v0.18.0-310p
      command: ["sleep"]
      args: ["100000"]
      resources:
        limits:
          huawei.com/Ascend310P: "1"
          huawei.com/Ascend310P-memory: "4096"
          huawei.com/Ascend310P-core: "90"
```

**注意：** 如果 Pod 没有携带 `hami-core` 注解，即使 ConfigMap 里已经启用了 `hami-vnpu-core`，设备仍会按模板 vNPU 模式分配。

支持的芯片型号、`ResourceName`/`ResourceMemoryName`/`ResourceCoreName` 完整列表，以及显存分配的取整规则，请参阅 Volcano 官方的 [Ascend vNPU 使用指南](https://github.com/volcano-sh/volcano/blob/master/docs/user-guide/how_to_use_vnpu.md#hami-mode)。

逐卡独立记账需要同时部署本插件及其锁定的 hami-vnpu-core `9f7be4c` 运行库。多卡分配要求所选各卡的显存和算力配额分别相同。例如，申请 4 张 Ascend910B3，每张卡 20 GiB 显存、50% 算力：

```yaml
resources:
  limits:
    huawei.com/Ascend910B3: "4"
    huawei.com/Ascend910B3-memory: "20480" # 每张卡 20 GiB
    huawei.com/Ascend910B3-core: "50"      # 每张卡 50% 算力
```

插件注入 `NPU_MEMORY_LIMIT_MB=20480` 和 `NPU_COMPUTE_LIMIT=50`，每张卡分别应用这两个配额；`-memory` 不应填写 `"81920"`。默认算力策略为 `spatial`。策略选择和升级要求参阅 [HAMi 配置说明](hami_cn.md#软切分配置-hami-vnpu-core)。

`NPU_GLOBAL_SHM_PATH=/hami-shared-region/global_registry` 和 `NPU_LOCAL_SHM_PATH=/hami-vnpu-shmem/vnpu_local_shmem` 是不绑定首卡的基础路径。应用首次使用某张卡时，core 在两个路径后追加 `.device-<physicalID>`。本地域按容器、物理卡分别记账，全局域使用跨容器共享的相同基础路径。业务仍使用容器内的逻辑设备号。

## 监控

当节点运行在 **hami-vnpu-core(软切)模式**时，设备插件会在 **`:9395/metrics`** 启动内置 **Prometheus exporter**，上报物理设备级和每容器的 vNPU 使用指标。传统的模板 vNPU(或整卡)模式**不会**启动它——那种模式没有软切数据可导出。

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
| `hami_vgpu_memory_used_bytes` | `namespace`, `pod`, `container`, `vdevice_index`, `device_uuid` | 该容器在该 NPU 上由 hook 记账的显存(字节) |
| `hami_vgpu_memory_limit_bytes` | `namespace`, `pod`, `container`, `vdevice_index`, `device_uuid` | 该容器在该 NPU 上的显存上限(字节) |
| `hami_container_device_utilization_ratio` | `namespace`, `pod`, `container`, `vdevice_index`, `device_uuid` | 容器所在设备的 AICore 利用率(0–100) |

对于逐卡 SHM 域，`vdevice_index` 为宿主机物理设备 ID，`device_uuid` 标识对应物理卡；旧版无后缀 SHM 保留注解/数组的顺序索引。仅导出已初始化的域。容器显存指标表示 hook 记账的分配量，宿主机显存指标保留 DCMI 上报的物理 HBM 使用量。容器利用率指标取所在整张卡的利用率，同卡容器共享这个数值。
