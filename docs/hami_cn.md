# 在 HAMi 中部署与使用

[English](hami.md) | **中文**

本文档介绍如何在 [HAMi](https://github.com/Project-HAMi/HAMi) 调度器下部署和使用 `ascend-device-plugin`。

## 环境要求

部署 [ascend-docker-runtime](https://gitcode.com/Ascend/mind-cluster/tree/master/component/ascend-docker-runtime)

- Ascend 驱动版本：≥ 25.5
- **`npu-smi` 必须在宿主机上可访问**，按以下顺序查找：
  1. `/usr/local/Ascend/driver/tools/npu-smi`（由已有的 driver hostPath 挂载提供）
  2. `/usr/local/sbin/npu-smi` (默认挂载路径)
  3. `/usr/local/bin/npu-smi`

  若宿主机的 `npu-smi` 在 `/usr/local/bin/npu-smi`，需要在 `ascend-device-plugin.yaml` 中为其增加 hostPath 挂载。

- **HAMi 版本**：
  - 基于模板的硬切分 (vNPU) 最低版本：≥ 2.7.0
  - `hami-core` 软切分 (hami-vnpu-core) 最低版本：≥ 2.9.0
  - `enpu` 运行时软切分：需要与业务镜像 CANN 版本匹配的 ubs-virt-enpu/vCANN-RT 产物

  这些模式都需要在部署 HAMi 时设置 `devices.ascend.enabled: true`。

  HAMi v2.9.0 把 `hami-scheduler-device` ConfigMap 中的 Ascend 芯片列表从 `vnpus` 挪到了 `vnpus.configs`。插件两种格式都能读，回退到旧格式时会打一条告警日志，因此可以先于 HAMi 升级。

**注意：** `hami-vnpu-core` 软切分目前仅支持 ARM 平台；基于模板的硬切分没有此限制。

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

* **HAMi 和 `ascend-device-plugin` 在同一命名空间(推荐)**：跳过这一步，HAMi 现有的 `hami-scheduler-device` 已经包含 Ascend 配置。
* **不同命名空间**：把 Ascend 的 ConfigMap 部署到 `ascend-device-plugin` 自己的命名空间下，然后手动把其中的 `vnpus:` 部分合并进 HAMi 现有的 `hami-scheduler-device`，不要动 HAMi 其他设备的配置。若 HAMi < v2.9.0，合并时要保留它自己的 `vnpus` 列表写法——那边的调度器读不了 `vnpus.configs`。以后修改模板、resourceName 或 `hamiVnpuCore` 时，两边同步更新。

  ```bash
  kubectl apply -f https://raw.githubusercontent.com/Project-HAMi/ascend-device-plugin/main/ascend-device-configmap.yaml
  ```

**注意：** `vnpus.hamiVnpuCore` 和 `vnpus.enpu` 分别启用两种运行时软切分后端；两者都为 `false` 时使用模板硬切分。节点配置中的 `hami-vnpu-core` 和 `enpu` 会覆盖全局设置。

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
spec:
  runtimeClassName: ascend
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
  runtimeClassName: ascend
  containers:
    - name: npu_pod
      ...
      resources:
        limits:
          huawei.com/Ascend910B3: "1"           # 请求 1 块物理 NPU
          huawei.com/Ascend910B3-memory: "28672" # 请求 28Gi 显存
          huawei.com/Ascend910B3-core: "40"      # 请求 40% 的算力
```

**注意：** `-core` 是可选的。与 `-memory`（省略时默认使用整卡）不同，省略 `-core` 默认值为 **0**，即不预留专属算力。它只有在设置了 `huawei.com/vnpu-mode: hami-core` 时才生效；未加该注解时设置 `-core` 会被拒绝。

软切分机制支持在单个 Pod 中申请多个虚拟设备。在进行多卡并行推理（如使用 vLLM）时，`--gpu-memory-utilization` 的值不能大于"容器总显存上限"占"所选卡物理显存总和"的比例。

### vCANN-RT 运行时软切分（enpu）

硬件与运行时前提请按 [vCANN-RT 官方配置示例](https://docs.openeuler.org/zh/docs/24.03_LTS_SP3/unifiedbus/unifiedbus/ubs-virt/ubs-virt-enpu/vcann-rt/README.html) 准备；节点启用方式、不同型号 ConfigMap 字段和 manager 接入见 [HAMi ENPU 使用说明](../examples/enpu/README_cn.md)。完整 Pod YAML：[普通软切分](../examples/enpu/soft-slicing.yaml)、[mem-swap 显存超分](../examples/enpu/mem-swap.yaml)。A2/910B 使用同一套适配，不需要 A3 的独立 DIE 模式设置；该要求仅适用于 A3/910C。910A 暂不列入 ENPU 支持范围。

`enpu` 使用 ubs-virt-enpu/vCANN-RT 的 runtime hook 实现算力和显存配额。为 Pod 添加 `huawei.com/vnpu-mode: enpu`，并使用现有 HAMi 资源：例如 `huawei.com/Ascend910C: "1"`、对应的 `-memory`（MB）和 `-core`（1–100%）。ENPU 每个容器只支持一个物理 DIE 上的共享份额；省略 `-core` 时按 100% 配置。必须设置 `runtimeClassName: ascend`。管理员负责在节点安装 Ascend 驱动，并在业务镜像中安装兼容版本的 CANN。

启用 ENPU 后，插件从自身镜像把 `libvruntime.so`、`enpu-monitor` 和 `ld.so.preload` 安装到宿主机 `/usr/local/enpu/vcann-rt`。已有文件内容相同时直接复用；内容不同时拒绝覆盖，插件启动失败。更换版本时遵循[运行库构建、升级与回滚说明](../enpu-runtime-assets/README.md)，先停止 ENPU 业务并暂停插件，再替换运行库文件。保留原有 hami-vnpu-core 文件。

```yaml
metadata:
  annotations:
    huawei.com/vnpu-mode: enpu
    huawei.com/enpu-policy: elastic # fixed-share、elastic 或 best-effort
spec:
  runtimeClassName: ascend
  containers:
    - name: npu
      resources:
        limits:
          huawei.com/Ascend910C: "1"
          huawei.com/Ascend910C-memory: "16384"
          huawei.com/Ascend910C-core: "20"
```

设备插件把每个容器的 vCANN-RT 配置写入 `ENPU_CONFIG_ROOT`（默认 `/var/lib/hami-enpu`），再挂载到容器的 `/etc/enpu/vcann-rt/npu_info.config`。挂载布局与官方 Kubernetes 示例一致：将节点的 `/usr/local/sbin`、`/usr/local/Ascend/driver`、运行库、监控器、`/etc/ld.so.preload` 和 `/dev/shm` 注入业务容器。业务镜像必须在 `/usr/bin/systemd-detect-virt` 提供该命令；如果镜像没有，可设置 `enpu.systemdDetectVirtPath`，插件会把兼容的主机二进制挂载到该路径。这是 vCANN-RT 调用 DCMI 的前置条件。仍需使用 `ascend` RuntimeClass。插件只设置 HAMi 选中的 DIE 对应的 `ASCEND_VISIBLE_DEVICES`，默认保持 `enpu.exposeAllDevices=false`，不会把全部 `/dev/davinci*` 暴露给业务容器。


A3/910C 的调度与配额单位是单个 DIE：一个 DIE UUID 对应一个 `PhyID`，配置中的 `physical-npu-id` 和 `ASCEND_VISIBLE_DEVICES` 使用同一个 `PhyID`，不是双 DIE 模块的 `CardID`。这与[官方软切分部署要求](https://www.hiascend.com/document/detail/en/mindcluster/2610/clustersched/schedulingug/docs/en/scheduling/usage/virtual_instance/virtual_instance_with_vcann_rt/01_soft_allocation_scheduling_inference.md)中的 `useSingleDieMode=true` 一致。HAMi 不使用 MindCluster 的启动参数；部署 ENPU 节点前，在主机配置等效的单 DIE 独立模式：

```shell
npu-smi info -t multi-die-policy
# 在确认节点业务允许切换后执行；该设置作用于整个节点。
npu-smi set -t multi-die-policy -d 1
npu-smi info -t multi-die-policy # 应为 INDEP_POLICY
```

ENPU 在 A3 分配前检查该模式，未启用时返回具体配置提示；插件不会自动改变节点模式，也不会为此扩大设备挂载。此检查仅针对 ENPU 的 Ascend910C 分配，不改变 hami-core 或模板分配路径。业务使用容器内的逻辑设备 `npu:0`；无需额外设置 `ASCEND_RT_VISIBLE_DEVICES`。

`systemd-detect-virt` 必须能在业务镜像内实际执行；优先通过镜像自身的软件包安装，并一同提供动态库依赖。仅挂载另一发行版的宿主机二进制可能因依赖或 glibc 不匹配而失败。运行前检查 `systemd-detect-virt --container` 和 `ldd /usr/bin/systemd-detect-virt`。

预发布 mem-swap 源码还必须包含上游 `3d87a1d678cd9b4e9bf3b0770285dc6b53d357ee` 的多 DIE 编号修复：CANN 逻辑设备号不能被 DCMI 卡内芯片号覆盖。官方 `1.0.0` 已包含该修复；测试分支 `1072945` 尚未包含。新增 swap 线程及物理内存操作也需要保持这两类编号分离；扩大设备可见范围不能替代此修复。

使用 mem-swap 时，设置 `huawei.com/enpu-memory-limit`（单位 MiB）；manager 开启超分后，limit 可以大于 request。`huawei.com/enpu-memory-request` 默认使用 HAMi 已调度的 `<chip>-memory` 资源，通常可省略；若显式设置，插件要求它与该资源配额相等，否则拒绝分配，保证调度器和 manager 预留同一个 request。例如资源 `huawei.com/Ascend910C-memory: 256` 配合注解 `huawei.com/enpu-memory-limit: "65536"` 即表示 request=256 MiB、limit=65536 MiB。没有这些注解时仍保持 request=limit=原显存配额；fixed-share 不允许 request 与 limit 不同。设置 `enpu.managerURL`，并让 `enpu.managerConfigRoot` 指向 enpu-manager 的 `config_dir`（例如 `/etc/enpu`）或已经展开的 `/etc/enpu/vcann-rt` 目录，两种写法都支持；两者不同时 chart 会把该目录额外挂载到 device-plugin。HAMI 保持 `/dev/shm` 使用主机共享空间，以便同一物理 NPU 上的多个 Pod 看到 manager 生成的 `shm-id`。同一物理 NPU 上的 Pod 必须使用同一种调度策略，且不会让 hami-core 与 ENPU 混用同一个物理 DIE。

ENPU 采用显式启用：业务 Pod 必须设置 `huawei.com/vnpu-mode: enpu`。ENPU-only 节点会拒绝没有该注解的普通 Pod，避免旧业务绕过运行时挂载。

### hami-vnpu-core 多卡 vLLM 示例（TP=2）

假设单块物理卡显存为 **64Gi**，计划在 2 块卡上各使用 **32Gi**（总计 64Gi）：

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: vllm-npu-2card
  annotations:
    huawei.com/vnpu-mode: 'hami-core' # 启用 hami-vnpu-core 软切分
spec:
  runtimeClassName: ascend
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
| `hami_vgpu_memory_used_bytes` | `namespace`, `pod`, `container`, `vdevice_index`, `device_uuid` | 每容器 vNPU 已用显存(字节) |
| `hami_vgpu_memory_limit_bytes` | `namespace`, `pod`, `container`, `vdevice_index`, `device_uuid` | 每容器 vNPU 显存上限(字节) |
| `hami_container_device_utilization_ratio` | `namespace`, `pod`, `container`, `vdevice_index`, `device_uuid` | 容器所在设备的 AICore 利用率(0–100) |
