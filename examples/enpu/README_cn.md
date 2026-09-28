# ENPU 普通软切分与显存超分示例

[English](README.md) | **中文**

ENPU 是 HAMi 的一个可选 Ascend 切分后端。普通软切分使用 vCANN-RT 控制算力和显存配额；mem-swap 在此基础上通过宿主机 enpu-manager 将冷实例的显存换出到 CPU 内存，供其他实例使用。两者均由 Pod 注解 `huawei.com/vnpu-mode: enpu` 显式选择。

## 官方配置依据

使用 ENPU 前，**请先按照对应版本的 [openEuler vCANN-RT 官方文档](https://docs.openeuler.org/zh/docs/24.03_LTS_SP3/unifiedbus/unifiedbus/ubs-virt/ubs-virt-enpu/vcann-rt/README.html) 配置硬件、CANN/HDK、容器共享模式、运行库和业务镜像**。该文档提供 Kubernetes/Docker 配置示例以及三种算力策略的说明。源码和版本入口见 [ubs-virt 官方仓库](https://gitcode.com/openeuler/ubs-virt/tree/master/ubs-virt-enpu) 与 [tags/releases](https://gitcode.com/openeuler/ubs-virt/tags)。

mem-swap 还需阅读所使用版本源码中的 `ubs-virt-enpu/enpu-manager/README.md` 和 `vcann-rt/README.md` 的显存超分章节。它是独立的版本能力：普通 release 不能仅通过添加注解获得超分；使用预发布代码时，manager 与 runtime 必须来自兼容版本。上述 openEuler 发布文档不代表每个 release 都包含 mem-swap。

官方 Kubernetes 示例使用 MindCluster/Volcano 的 `AscendJob` 和资源字段。本目录将同一运行时能力接入 HAMi，使用下面的 Pod 注解和 HAMi 资源名；无需照搬 `AscendJob`、Volcano 调度器或手工配置 `physical-npu-id`、`virtual-npu-id`、`shm-id`。

## 示例文件

| 文件 | 用途 |
| --- | --- |
| [device-plugin-values.yaml](device-plugin-values.yaml) | 复用 HAMi ConfigMap，仅在指定节点启用 ENPU |
| [soft-slicing.yaml](soft-slicing.yaml) | 普通软切分：PyTorch 单设备、16 GiB、20% 算力配额 |
| [mem-swap-values.yaml](mem-swap-values.yaml) | 超分附加配置：连接本节点 manager、挂载生成配置的目录 |
| [mem-swap.yaml](mem-swap.yaml) | 超分：单实例 vLLM，256 MiB request、64 GiB limit，TP1/eager |
| [podmonitor.yaml](podmonitor.yaml) | 可选、手动应用的 Prometheus Operator 抓取配置 |

这些文件是配置模板。应用前替换镜像、节点名、manager 地址、模型路径和 PVC；`registry.example.com` 与 `192.0.2.10` 都是占位值。示例默认使用 `Ascend910C`，其他型号请按下面的型号表修改资源名和显存预算。

## 部署前提与启用方式

1. 部署包含 ENPU 适配的 HAMi scheduler 和 ascend-device-plugin。沿用现有 release 镜像编译、打包流程，将对应版本的 `libvruntime.so`、`enpu-monitor`、`ld.so.preload` 放入 device-plugin 镜像；插件安装到宿主机 `/usr/local/enpu/vcann-rt` 并注入业务容器。构建环境可参考官方文档中的预编译镜像，但该镜像不等于已经包含所需版本运行库的业务镜像。
2. 管理员负责在节点安装 Ascend 驱动，在业务镜像中安装兼容版本的 CANN，按官方文档配置 `device-share`，并部署 ascend-docker-runtime 和 `ascend` RuntimeClass。业务镜像还需提供可执行的 `/usr/bin/systemd-detect-virt` 及其依赖。PyTorch 示例另需 `torch_npu`，vLLM 示例另需 vLLM-Ascend。
3. 本适配在 A3/910C 按物理 DIE 分配，当前路径要求节点为 `INDEP_POLICY`。这是本适配采用的单 DIE 部署方式；[MindCluster 官方软切分说明](https://www.hiascend.com/document/detail/en/mindcluster/2610/clustersched/schedulingug/docs/en/scheduling/usage/virtual_instance/virtual_instance_with_vcann_rt/01_soft_allocation_scheduling_inference.md) 对应 `useSingleDieMode=true`，不是“所有 mem-swap 实现都不支持联合模式”的结论。节点模式由管理员按官方说明配置，插件只检查、不自动更改。
4. scheduler 与 plugin 共用完整的 `hami-scheduler-device` ConfigMap，保证 `vnpus.configs` 的型号、资源名和 `vnpus.enpuPolicy` 一致。示例使用节点级 `enpu: true`；插件会注册节点能力供 scheduler 识别，不必把整个集群的 `vnpus.enpu` 改成 `true`。

插件复用内容相同的运行库文件；已有文件与镜像内文件内容不同时，插件启动失败。更换运行库版本前，按[升级与回滚流程](../../enpu-runtime-assets/README.md)停止 ENPU 业务、暂停插件，只备份并移走这三个 ENPU 运行库文件，再部署新镜像。保留原有 hami-vnpu-core 文件。

将 [device-plugin-values.yaml](device-plugin-values.yaml) 合并到现有插件 chart 的 values，保留现有镜像、节点选择及其他设置。目标节点须匹配插件的 `nodeSelector`（默认 `ascend=on`）；`nodeConfig` 本身不会给节点加标签。Pod 中的 `schedulerName: hami-scheduler` 也应与实际 HAMi 配置一致。`nodeConfig` 是整段 YAML 字符串，Helm 会整段替换；必须保留已有节点条目，仅追加或修改选定的 ENPU 节点。节点条目中的 `hami-vnpu-core` 应显式填写，省略也会覆盖全局值为 `false`。

例如，在仓库根目录更新**现有独立插件 release**（替换 release 名、命名空间和现有 values 路径）：

```shell
helm upgrade <existing-plugin-release> ./charts/ascend-device-plugin \
  --namespace <same-namespace-as-hami> \
  -f <existing-plugin-values.yaml> -f examples/enpu/device-plugin-values.yaml
```

如已由其他 chart 管理插件，修改原有部署，不要重复安装同名 DaemonSet、ConfigMap 或 RuntimeClass。其他节点继续保留原 hami-core/模板硬切分配置；同一物理 DIE 不混用 hami-core 和 ENPU。全局 `vnpus.enpu` / chart `enpu.enabled` 适用于明确要全局启用的情况，而本示例选择节点级启用。

## 普通软切分

修改镜像后执行 `kubectl apply -f examples/enpu/soft-slicing.yaml`，用 `kubectl logs enpu-soft-slicing` 查看输出。无需配置 manager；`enpu.managerURL` 留空时，插件自行生成每容器配置，显存 request 与 limit 都等于 `-memory` 配额。

普通 ENPU 模式的 `enpu.configRoot`（默认 `/var/lib/hami-enpu`）记录每个 Pod UID/容器在选中 DIE 上的独立虚拟槽位，插件重启时必须保留该目录。只有 Pod 已从 Kubernetes API 删除，且 kubelet 已释放其持久分配记录，后续分配才能回收槽位。API 或 checkpoint 不可读取、checkpoint 格式不受支持时，插件保留原有记录。manager 分配仍按下方 mem-swap 章节说明管理生命周期。

`huawei.com/enpu-policy` 支持 `fixed-share`、`elastic`、`best-effort`。同一 DIE 的 ENPU 实例必须使用同一种策略。`fixed-share` 按配额执行时间片，`elastic` 可借用空闲算力，`best-effort` 不按配额限制算力；三者不能都解读为严格的算力上限。`-core` 单位为百分比，取值 1–100，省略为 100；普通显存配额不会因 `best-effort` 而放开。当前每个容器只支持一个物理 DIE，应用使用容器内逻辑设备 `npu:0`。

## mem-swap 显存超分

先按对应版本的 enpu-manager 文档在目标宿主机部署服务。配置 `oversub-ratio` 为目标物理 DIE 开启超分，并按官方公式核算宿主机 CPU 内存池容量；单值会应用到所有 DIE，需要按卡启用时使用 per-DIE 列表。`swap-pre-watermark` 控制预换出水位，需结合版本和业务负载设置；预发布版本的预换出与按需换出可能存在竞争，不能把调整水位当作协议修复。

将 [mem-swap-values.yaml](mem-swap-values.yaml) 作为上述 Helm 命令的最后一个 `-f` 参数。`managerURL` 必须指向 **Pod 所在节点的 manager**；插件未启用 hostNetwork，不能填 `127.0.0.1`。该值目前是 DaemonSet 全局配置，此示例面向一个 ENPU 节点；多节点需要分别配置到本节点的地址，不能用普通负载均衡 Service 随机访问其他节点。`managerConfigRoot` 必须与 manager 的 `config_dir` 对应（例如 `/etc/enpu`，也可用展开后的 `/etc/enpu/vcann-rt`）。本 chart 不负责安装或管理 enpu-manager 服务。

修改业务镜像、模型 PVC 和路径后执行 `kubectl apply -f examples/enpu/mem-swap.yaml`。示例字段含义如下：

| 字段 | 含义 |
| --- | --- |
| `huawei.com/Ascend910C-memory: "256"` | HAMi 调度预留显存，单位 MiB |
| `huawei.com/enpu-memory-request` | 可省略；默认为上面的配额，显式填写必须与其相等 |
| `huawei.com/enpu-memory-limit: "65536"` | 单实例显存上限，单位 MiB；不是再向 HAMi 申请 64 GiB 预留 |
| `huawei.com/enpu-policy: best-effort` | 本示例使用的算力策略；fixed-share 要求 request 等于 limit，不能用于此类 request 小于 limit 的超分 |

256/65536 只是示例，须根据实际设备容量、模型和不可交换内存调整；`--gpu-memory-utilization` 是 vLLM 自身的内存预算，也需留出运行开销。不要为实现超分把 ConfigMap 的物理显存容量直接乘倍数。HAMi 根据选中的 DIE、Pod UID、容器名调用 manager，并挂载其生成的配置，业务 YAML 不需要手工挂载运行库、设备节点或 manager 配置。保留 `runtimeClassName: ascend`，由插件设置设备可见性，不额外设置 `ASCEND_RT_VISIBLE_DEVICES`。

多个实例共享同一 DIE 时，复制 Pod 示例并修改名称，按需在各 Pod 设置同一个真实的 `hami.io/use-Ascend910C-uuid`；仅使用相同资源名并不保证调度到同一 DIE。每 Pod 保持一个 ENPU 业务容器。接近整 DIE 容量的实例应依次初始化，确认已有实例已换出并释放 HBM 后再加载下一个；不要假定三个大模型同时启动一定能完成。正常请求访问由 runtime/manager 协调换入换出，不等同于暂停容器或 vLLM 的 sleep API。避免通过 SIGSTOP/docker pause 控制交换，也不要仅凭管理 API 返回成功判断交换完成。

当前接入未自动回收 Pod 退出后的 manager 分配登记；清理时先确认对应容器已退出，再按官方 API 释放准确的 Pod UID/容器名。不要删除其他存活实例的配置或共享内存。

## 监控指标

节点启用 ENPU 后，插件在原有 `:9395/metrics` 端点（端口名 `monitorport`）提供 Prometheus 指标，无需新增环境变量。静态 DaemonSet 和 chart 将宿主机 `/proc` 只读挂载到 `/host/proc`，沿用已有 privileged 安全配置。采集无需 `hostPID`、`pods/exec` 权限，也不会在业务容器中执行命令；自定义部署须保留该挂载和原有 ENPU 配置目录挂载。

采集器读取 DCMI 进程 HBM 数据，通过宿主机 PID 的 cgroup 匹配当前 Pod/container ID，并核对物理设备与分配 UUID，同一容器的进程用量求和。除特别说明外，容器指标具有 `namespace`、`pod`、`container`、`vdevice_index`（当前为 `0`）、`device_uuid` 五个标签。下表显存值均以 bytes 为单位。

| 指标 | 含义 |
| --- | --- |
| `hami_vgpu_memory_used_bytes` | 按 DCMI 进程数据归属到容器的实际 HBM 驻留量 |
| `hami_vgpu_memory_limit_bytes` | ENPU 配置的显存上限 |
| `hami_enpu_memory_request_bytes` | 配置中由 HAMi 预留的显存量 |
| `hami_enpu_aicore_quota_percent` | 配置的算力份额，不是实测利用率；约束方式取决于策略 |
| `hami_enpu_allocation_info` | 值为 `1`；另加 `physical_device_id`、`virtual_device_id`、`policy` 标签 |
| `hami_enpu_memory_collection_success` | 每容器显存归属状态：成功为 `1`，失败为 `0` |
| `hami_enpu_config_collection_success` | 采集器全局状态，无指标标签：所有运行中 ENPU 分配配置均读取成功为 `1`，否则为 `0` |
| `hami_enpu_device_collection_success` | 采集器全局状态，无指标标签：设备采集成功为 `1`，否则为 `0` |
| `hami_host_gpu_memory_used_bytes` | 整个 DIE 的 HBM 用量；标签为 `device_index`、`device_uuid`、`device_type` |
| `hami_host_gpu_utilization_ratio` | 整个 DIE 的 AI Core 利用率，取值 **0–100**，标签同上 |

缺失或失败的数据不会填成零：对应 success 指标变为 `0`，受影响的样本不输出。配置读取失败时，该分配的全部指标都可能缺失，因此应同时检查全局状态和每容器状态；成功采集得到的零值仍是有效数据。

本接入不提供 ENPU 每 Pod 算力利用率，不会将整 DIE 利用率复制给每个 Pod，配置配额也不是所有策略下的严格上限。mem-swap 配置支持不同的显存 request 和 limit，但实测用量仅包含 HBM 驻留，不包含换出到 CPU 的字节数或 sleep 状态；该端点不是 enpu-manager exporter。仅启用 hami-core 及原有 template 路径时保持原采集行为；同时启用 hami-core 与 ENPU 时，整设备指标由 DCMI 提供且只输出一次。

手动检查运行时可在 ENPU 业务容器内执行 `/usr/local/enpu/vcann-rt/tools/enpu-monitor`。上游 release **1.0.0** 向 stderr 输出三项：AI Core 配额（%）、显存上限和显存用量；显存字段虽标为 `MB`，实际为整数 MiB。它没有 HTTP 端点或每 Pod 利用率输出；`best-effort` 下 CLI 配额显示 `0` 不代表 HAMi 配置了零配额。插件采集器不会调用或解析该 CLI。

### 可选 Prometheus Operator 发现配置

[podmonitor.yaml](podmonitor.yaml) 仅供手动应用，chart 不默认安装，也不默认依赖 PodMonitor CRD。已安装 Prometheus Operator 时，先调整示例，再执行 `kubectl apply -f examples/enpu/podmonitor.yaml`：

- 将其命名空间和 `spec.namespaceSelector.matchNames` 改为插件命名空间。Pod selector 中 `app.kubernetes.io/instance` 对应 Helm release，`app.kubernetes.io/name` 对应 chart 名或 `nameOverride`，保留 `app.kubernetes.io/component: hami-ascend-device-plugin`。这些是 chart 的 Pod 标签，不是 DaemonSet 名；静态部署应使用实际 Pod 标签。
- PodMonitor 的 metadata 标签与所在命名空间须匹配 Prometheus 的 `podMonitorSelector` 和 `podMonitorNamespaceSelector`。示例的 `release: prometheus` 是占位值，不是插件 release 标签。
- 端点使用 `monitorport`（9395）、`/metrics` 和 `honorLabels: true`，以保留业务标签。若 Prometheus 设置 `overrideHonorLabels: true`，会覆盖该选项：冲突的原始业务指标标签改名为 `exported_*`，抓取目标的 `namespace`、`pod` 等标签仍指向被抓取的插件 Pod，查询时须相应处理。见 [Prometheus Operator API 文档](https://prometheus-operator.dev/docs/api-reference/api/)。

## NPU 型号与 ConfigMap

型号配置位于 [ascend-device-configmap.yaml](../../ascend-device-configmap.yaml) 的 `data.device-config.yaml → vnpus.configs`，chart 内对应 `deviceConfig`；使用共享 ConfigMap 时以 HAMi scheduler 管理的版本为准。插件用 DCMI 返回的 `chipName` 精确匹配，`commonWord` 和资源名是 HAMi 标识，不能只按卡的营销名称推断。

| `chipName` | HAMi 资源前缀 |
| --- | --- |
| `910A` | `huawei.com/Ascend910A` |
| `910B2` / `910B3` / `910B4-1` / `910B4` | `huawei.com/Ascend910B2` / `huawei.com/Ascend910B3` / `huawei.com/Ascend910B4-1` / `huawei.com/Ascend910B4` |
| `310P3` | `huawei.com/Ascend310P` |
| `Ascend910` | `huawei.com/Ascend910C` |

每项的 `resourceName`、`resourceMemoryName`、`resourceCoreName` 应与 Pod 资源字段对应。`memoryCapacity` / `memoryAllocatable` 描述物理容量与可分配容量，`aiCore` 和 `templates` 保留原模板模式的配置；ENPU 的 `-core` 为百分比，不是物理核数。插件独立 ConfigMap 的部分型号缺少 `resourceCoreName`，使用算力配额前应补齐并同步 scheduler 配置；不要删除原有模板。上述示例复用 HAMi 已包含完整资源字段的共享 ConfigMap。

**本适配支持 A2/910B 和 A3/910C 的 ENPU 接入。** A2/910B 不需要设置 `useSingleDieMode=true` 或 A3 的 `multi-die-policy`，使用对应型号的资源字段即可；独立 DIE 检查只针对 `Ascend910C`。两者都须满足官方 CANN/HDK 和运行库要求，mem-swap 另需兼容的 manager/runtime 版本。示例从 910C 改为 910B3 时，应同时将三个资源名改为 `Ascend910B3`、`Ascend910B3-memory`、`Ascend910B3-core`，并根据设备容量调整显存值及 UUID 注解名。

**ConfigMap 有某型号，不等于 ENPU 已支持该型号。** 官方文档未列出 910A，不能仅凭配置存在或 DCMI 能枚举就声明它支持 ENPU，暂不列入 ENPU 支持范围。310P3 等已有条目同样不能直接视作 ENPU 支持承诺；新型号须同时满足上游 runtime、manager 和 HAMi 型号识别条件。
