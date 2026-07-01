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
- 芯片模式：`device-share` 现在由插件在**节点级自动管理**。当节点被配置为 hami-vnpu-core 软切分（`device-node-config` 中的 `hami-vnpu-core` 字段）时，插件在**启动阶段对节点上的每块芯片**开启 device-share，逐芯片执行一次 `npu-smi set -t device-share -i <card> -c <chip> -d 1`，无需手动执行 `npu-smi`，也无需在 Pod 上加注解来开启。

  插件按以下顺序解析 `npu-smi`：`/usr/local/Ascend/driver/tools/npu-smi`（由已有的 driver hostPath 挂载提供）、`/usr/local/sbin/npu-smi`、`/usr/local/bin/npu-smi`，最后是 `PATH`。若宿主机的 `npu-smi` 在其他位置，可在 `ascend-device-plugin.yaml` 中增加单文件 hostPath 挂载（例如 `/usr/local/sbin/npu-smi`）。

  该配置仅在启动时读取一次，因此**修改 `hami-vnpu-core` 需重启插件**才能生效。若任一芯片翻转失败，**插件将启动失败**，在成功之前不会上报该节点的设备。

<details>
<summary>历史：手动开启 <code>device-share</code>（已不再需要）</summary>

**开启 `device-share` 模式**

**npu-smi set -t device-share -i** *id* **-d** *value*  用于设置指定设备的所有芯片的容器共享模式。

**参数说明**

| 类型    | 描述                                                        |
| ------- | ----------------------------------------------------------- |
| *id*    | 设备 ID。通过 **npu-smi info -l** 命令查出的 NPU ID 即为设备 ID。 |
| *value* | 容器使能状态：分为禁用、使能。默认禁用。<br>0：禁用<br>1：使能 |

</details>

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
