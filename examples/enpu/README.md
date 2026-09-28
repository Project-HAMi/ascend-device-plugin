# ENPU soft slicing and memory oversubscription examples

**English** | [中文](README_cn.md)

ENPU is an optional Ascend backend for HAMi. Ordinary soft slicing uses vCANN-RT to enforce compute and memory quotas. Mem-swap adds a host enpu-manager that moves cold instances' memory into CPU RAM so other instances can use the released HBM. Pods select this backend with `huawei.com/vnpu-mode: enpu`.

## Official configuration references

Before using ENPU, **follow the matching version of the [official openEuler vCANN-RT documentation](https://docs.openeuler.org/zh/docs/24.03_LTS_SP3/unifiedbus/unifiedbus/ubs-virt/ubs-virt-enpu/vcann-rt/README.html)** for hardware, CANN/HDK, device sharing, runtime assets and workload images. It includes Kubernetes/Docker examples and compute policy descriptions. See also the [official source repository](https://gitcode.com/openeuler/ubs-virt/tree/master/ubs-virt-enpu) and [tags/releases](https://gitcode.com/openeuler/ubs-virt/tags).

For mem-swap, also read `ubs-virt-enpu/enpu-manager/README.md` and the memory oversubscription section of `vcann-rt/README.md` in the version you use. Adding annotations does not enable mem-swap in a release without that capability; the manager and runtime must be compatible. The published openEuler guide does not imply that every release contains mem-swap.

The upstream Kubernetes example uses MindCluster/Volcano `AscendJob` resources. These examples integrate the runtime through HAMi Pod annotations and resource names instead. There is no need to copy an `AscendJob`, select Volcano, or manually assign `physical-npu-id`, `virtual-npu-id` or `shm-id`.

## Files

| File | Purpose |
| --- | --- |
| [device-plugin-values.yaml](device-plugin-values.yaml) | Reuse HAMi's ConfigMap and enable ENPU on a selected node |
| [soft-slicing.yaml](soft-slicing.yaml) | Single-device PyTorch, 16 GiB memory and a 20% compute quota |
| [mem-swap-values.yaml](mem-swap-values.yaml) | Additional manager endpoint and generated configuration directory |
| [mem-swap.yaml](mem-swap.yaml) | One vLLM instance, 256 MiB request, 64 GiB limit, TP1/eager |
| [podmonitor.yaml](podmonitor.yaml) | Optional, manually applied Prometheus Operator scrape configuration |

These are configuration templates. Replace the images, node name, manager address, model path and PVC before applying them; `registry.example.com` and `192.0.2.10` are placeholders. The examples use `Ascend910C`; adjust resource names and memory budgets for other models as described below.

## Prerequisites and deployment

1. Deploy HAMi scheduler and ascend-device-plugin builds containing the ENPU integration. Keep the existing release build and packaging process, including compatible `libvruntime.so`, `enpu-monitor` and `ld.so.preload` assets in the plugin image. The plugin installs them under `/usr/local/enpu/vcann-rt` on the node and injects them into workloads. The upstream prebuilt build-environment image is not a workload image containing every required runtime version.
2. Administrators install compatible Ascend drivers on the node and CANN in the workload image, configure `device-share` according to the official guide, and install ascend-docker-runtime with the `ascend` RuntimeClass. Workload images also need an executable `/usr/bin/systemd-detect-virt` with its dependencies. The PyTorch example needs `torch_npu`; the vLLM example needs vLLM-Ascend.
3. On A3/910C, this integration allocates physical DIEs and currently requires `INDEP_POLICY`. This follows the single-DIE deployment corresponding to `useSingleDieMode=true` in the [official MindCluster soft-slicing guide](https://www.hiascend.com/document/detail/en/mindcluster/2610/clustersched/schedulingug/docs/en/scheduling/usage/virtual_instance/virtual_instance_with_vcann_rt/01_soft_allocation_scheduling_inference.md); it is not a claim that every mem-swap implementation forbids union mode. Administrators configure the node policy; the plugin checks it without changing it.
4. Share the complete `hami-scheduler-device` ConfigMap between scheduler and plugin, keeping `vnpus.configs` and `vnpus.enpuPolicy` consistent. The example enables ENPU per node. The plugin registers the capability for the scheduler, so setting `vnpus.enpu: true` across the whole cluster is unnecessary.

The plugin reuses identical runtime files but fails startup if existing files differ from those in its image. Before changing the runtime version, follow the [upgrade and rollback procedure](../../enpu-runtime-assets/README.md): stop ENPU workloads, pause the plugin, and back up and move only the three ENPU runtime files before deploying the new image. Keep the hami-vnpu-core assets in place.

Merge [device-plugin-values.yaml](device-plugin-values.yaml) into the existing plugin chart values, preserving image, node selection and other settings. The target node must match the plugin's `nodeSelector` (default `ascend=on`); `nodeConfig` does not label nodes. Match the Pods' `schedulerName: hami-scheduler` to the deployed HAMi scheduler. Helm replaces the entire `nodeConfig` string: preserve existing node entries and change only the selected ENPU node. Explicitly set `hami-vnpu-core` in each entry; omitting it also overrides the global value with `false`.

For an **existing standalone plugin release**, run from the repository root after replacing the release, namespace and existing values path:

```shell
helm upgrade <existing-plugin-release> ./charts/ascend-device-plugin \
  --namespace <same-namespace-as-hami> \
  -f <existing-plugin-values.yaml> -f examples/enpu/device-plugin-values.yaml
```

If another chart owns the plugin, update that deployment rather than installing duplicate DaemonSets, ConfigMaps or RuntimeClasses. Retain hami-core/template slicing settings on other nodes. Do not mix hami-core and ENPU on one physical DIE. Global `vnpus.enpu` / chart `enpu.enabled` are available for intentional global enablement; these examples use node-level enablement.

## Ordinary soft slicing

After replacing the image, run `kubectl apply -f examples/enpu/soft-slicing.yaml` and inspect `kubectl logs enpu-soft-slicing`. No manager is required. With `enpu.managerURL` empty, the plugin generates per-container configurations locally; both memory request and limit equal the `-memory` quota.

In ordinary ENPU mode, `enpu.configRoot` (default `/var/lib/hami-enpu`) stores a separate virtual-slot reservation for each Pod UID and container on the selected DIE. Preserve this directory across plugin restarts. A later allocation can reclaim a slot only after the Pod is deleted from the Kubernetes API and kubelet releases its persistent allocation record. If the API or checkpoint is unavailable, or the checkpoint format is unsupported, the plugin retains reservations. Manager allocations follow the lifecycle described in the mem-swap section below.

`huawei.com/enpu-policy` accepts `fixed-share`, `elastic` and `best-effort`. All ENPU instances sharing a DIE must use the same policy. Fixed-share follows assigned time slices, elastic can borrow idle compute, and best-effort does not cap compute by quota. These policies therefore do not all impose a strict compute ceiling. `-core` is a percentage from 1 to 100 and defaults to 100 when omitted. Ordinary memory quotas still apply under best-effort. Each container currently supports one physical DIE; applications use the container-local device `npu:0`.

## Mem-swap

Deploy enpu-manager on the target host following its version's documentation. Configure `oversub-ratio` for the selected physical DIEs and size the CPU memory pool using the official formula. A scalar applies to all DIEs; use a per-DIE list for selective enablement. Choose `swap-pre-watermark` according to the runtime version and workload. Preview versions can have races between proactive and on-demand swap; changing the watermark is not a protocol fix.

Add [mem-swap-values.yaml](mem-swap-values.yaml) as the last `-f` argument to the Helm command above. `managerURL` must reach the manager **on the workload's node**. The plugin does not use hostNetwork, so `127.0.0.1` refers to the plugin Pod. This value is currently global to the DaemonSet; the example targets one ENPU node. Multi-node deployments need separate configuration that reaches each node's own manager, never a Service that randomly balances allocations across managers. Match `managerConfigRoot` to the manager's `config_dir`, such as `/etc/enpu`, or its expanded `/etc/enpu/vcann-rt` directory. This chart does not install or manage the enpu-manager service.

After replacing the image, model PVC and path, run `kubectl apply -f examples/enpu/mem-swap.yaml`.

| Field | Meaning |
| --- | --- |
| `huawei.com/Ascend910C-memory: "256"` | Memory reserved by HAMi, in MiB |
| `huawei.com/enpu-memory-request` | Optional; defaults to that quota and must equal it if supplied |
| `huawei.com/enpu-memory-limit: "65536"` | Instance memory ceiling in MiB; does not reserve another 64 GiB through HAMi |
| `huawei.com/enpu-policy: best-effort` | Policy used here; fixed-share requires request = limit and cannot provide this request < limit configuration |

256/65536 are example values: adjust them to the device, model and non-swappable memory requirements. vLLM's `--gpu-memory-utilization` controls its own budget and must leave runtime headroom. Do not multiply physical memory capacities in the ConfigMap to enable oversubscription. HAMi calls the manager with the selected DIE, Pod UID and container name and mounts the resulting configuration. Workload YAML does not need manual runtime, device or manager-config mounts. Keep `runtimeClassName: ascend`; let the plugin set device visibility without adding `ASCEND_RT_VISIBLE_DEVICES`.

To share one DIE across instances, copy the Pod with distinct names and, when needed, set the same actual `hami.io/use-Ascend910C-uuid` on each. Equal resource names alone do not ensure placement on one DIE. Keep one ENPU workload container per Pod. Initialize instances approaching the DIE's capacity sequentially, confirming HBM release from earlier instances before loading the next. Do not assume that three large models can initialize concurrently. Runtime/manager coordinate swapping on workload access; this is different from pausing containers or invoking vLLM's sleep API. Avoid SIGSTOP/docker pause for swap control, and do not treat a management API acknowledgement alone as proof of completed exchange.

The current integration does not automatically release manager allocation records after Pod exit. Confirm the container has exited, then use the official API to release the exact Pod UID/container pair. Preserve configurations and shared memory belonging to live instances.

## Metrics

When ENPU is enabled on a node, the plugin serves Prometheus metrics at its existing `:9395/metrics` endpoint (`monitorport`). No additional environment variable is needed. The static DaemonSet and chart mount host `/proc` read-only at `/host/proc`, retaining the existing privileged security context. Collection does not require `hostPID`, `pods/exec` permission, or commands inside workload containers. Custom deployments must include that mount and the existing ENPU configuration mounts.

The collector reads DCMI process HBM usage and matches host PIDs through their cgroups to current Pod/container IDs, checking the physical device and allocated UUID. It aggregates processes belonging to the same container. Container metrics use `namespace`, `pod`, `container`, `vdevice_index` (currently `0`) and `device_uuid` labels unless stated otherwise. Memory values below are bytes.

| Metric | Meaning |
| --- | --- |
| `hami_vgpu_memory_used_bytes` | Container's resident HBM, attributed from DCMI process memory |
| `hami_vgpu_memory_limit_bytes` | Configured ENPU memory limit |
| `hami_enpu_memory_request_bytes` | Configured memory reservation used by HAMi |
| `hami_enpu_aicore_quota_percent` | Configured compute share, not measured utilization; enforcement depends on the policy |
| `hami_enpu_allocation_info` | Value `1`; adds `physical_device_id`, `virtual_device_id` and `policy` labels |
| `hami_enpu_memory_collection_success` | Per-container attribution status: `1` on success, `0` on failure |
| `hami_enpu_config_collection_success` | Collector-wide status, with no metric labels: `1` if all running ENPU allocations were read successfully, otherwise `0` |
| `hami_enpu_device_collection_success` | Collector-wide status, with no metric labels: `1` if device collection succeeded, otherwise `0` |
| `hami_host_gpu_memory_used_bytes` | Whole-DIE HBM usage; labels are `device_index`, `device_uuid`, `device_type` |
| `hami_host_gpu_utilization_ratio` | Whole-DIE AI Core utilization on a **0–100** scale, with the same device labels |

Missing or failed measurements are not replaced with zero: the relevant success metric becomes `0` and affected samples are omitted. A configuration failure can omit all metrics for that allocation; check the collector-wide status as well as per-container status. A successfully measured zero remains valid.

There is no ENPU per-Pod compute utilization metric in this integration. Whole-DIE utilization is not copied onto each Pod, and the configured quota is not a strict ceiling under every policy. Mem-swap configurations support distinct memory request and limit values, but measured usage includes only resident HBM, not swapped-out CPU bytes or sleep state. This endpoint is not an enpu-manager exporter. The existing hami-core-only and template paths retain their collection behavior; when both hami-core and ENPU are enabled, whole-device metrics come from DCMI and are exported once.

For a manual runtime check, run `/usr/local/enpu/vcann-rt/tools/enpu-monitor` inside an ENPU workload container. In upstream release **1.0.0**, it prints three fields to stderr: AI Core quota (%), memory limit and memory usage. Its memory fields are integer MiB despite the `MB` labels; it has no HTTP endpoint or per-Pod utilization output. Under `best-effort`, a CLI quota of `0` does not mean HAMi configured a zero quota. The plugin collector does not invoke or parse this CLI.

### Optional Prometheus Operator discovery

[podmonitor.yaml](podmonitor.yaml) is a manual example, not installed by the chart; the plugin has no default PodMonitor CRD dependency. With Prometheus Operator installed, adjust the example before running `kubectl apply -f examples/enpu/podmonitor.yaml`:

- Set its namespace and `spec.namespaceSelector.matchNames` to the plugin namespace. Match the Pod selector's `app.kubernetes.io/instance` to the Helm release and `app.kubernetes.io/name` to the chart name (or `nameOverride`); keep `app.kubernetes.io/component: hami-ascend-device-plugin`. These are the chart's Pod labels, not the DaemonSet name. For a static deployment, use its actual Pod labels.
- Match the PodMonitor's metadata labels and namespace to Prometheus's `podMonitorSelector` and `podMonitorNamespaceSelector`. The example `release: prometheus` is a placeholder, not a plugin release label.
- The endpoint uses `monitorport` (9395), `/metrics` and `honorLabels: true` to preserve workload labels. Prometheus `overrideHonorLabels: true` overrides this: conflicting original workload metric labels become `exported_*`, while target labels such as `namespace` and `pod` still identify the scraped plugin Pod. Account for that in queries. See the [Prometheus Operator API reference](https://prometheus-operator.dev/docs/api-reference/api/).

## NPU models and ConfigMap

Per-model entries live under `data.device-config.yaml → vnpus.configs` in [ascend-device-configmap.yaml](../../ascend-device-configmap.yaml), or `deviceConfig` in the plugin chart. When sharing the ConfigMap, use the version managed by HAMi scheduler. The plugin matches `chipName` exactly against DCMI's chip name; `commonWord` and resource names are HAMi identifiers, not necessarily the product's marketing name.

| `chipName` | HAMi resource prefix |
| --- | --- |
| `910A` | `huawei.com/Ascend910A` |
| `910B2` / `910B3` / `910B4-1` / `910B4` | `huawei.com/Ascend910B2` / `huawei.com/Ascend910B3` / `huawei.com/Ascend910B4-1` / `huawei.com/Ascend910B4` |
| `310P3` | `huawei.com/Ascend310P` |
| `Ascend910` | `huawei.com/Ascend910C` |

Match `resourceName`, `resourceMemoryName` and `resourceCoreName` to the Pod fields. `memoryCapacity` / `memoryAllocatable` describe physical and allocatable memory; `aiCore` and `templates` retain the original template configuration. ENPU `-core` is a percentage, not a physical core count. Some entries in the standalone plugin ConfigMap omit `resourceCoreName`; add it consistently on both scheduler and plugin before requesting a compute quota. Preserve existing templates. These examples reuse HAMi's shared ConfigMap, which already includes the resource fields.

**This integration supports ENPU on A2/910B and A3/910C.** A2/910B does not need `useSingleDieMode=true` or A3's `multi-die-policy`: use the appropriate model's resource fields. The independent-DIE check only applies to `Ascend910C`. Both require the official CANN/HDK and runtime prerequisites; mem-swap additionally requires compatible manager/runtime versions. When adapting the example to 910B3, change all three resources to `Ascend910B3`, `Ascend910B3-memory` and `Ascend910B3-core`, and adjust memory budgets and the UUID annotation key accordingly.

**A ConfigMap entry is not an ENPU support guarantee.** The official guide does not list 910A; it is not currently included in the ENPU support scope merely because HAMi has a configuration entry or DCMI can enumerate it. The same distinction applies to existing entries such as 310P3. New models require support from the upstream runtime, manager and HAMi's device identification.
