# Deploy & Use with HAMi

**English** | [中文](hami_cn.md)

This guide covers deploying `ascend-device-plugin` for use with the [HAMi](https://github.com/Project-HAMi/HAMi) scheduler.

## Prerequisites

[ascend-docker-runtime](https://gitcode.com/Ascend/mind-cluster/tree/master/component/ascend-docker-runtime)

- **Ascend Driver Version**: ≥ 25.5
- **`npu-smi` must be reachable on the host**, at one of the following paths (checked in order):
  1. `/usr/local/Ascend/driver/tools/npu-smi` (from the existing driver hostPath mount)
  2. `/usr/local/sbin/npu-smi` (mounted by default)
  3. `/usr/local/bin/npu-smi`

  If `npu-smi` is located at `/usr/local/bin/npu-smi`, please add the path mount in `ascend-device-plugin.yaml`.

- **HAMi Version**:
  - ≥ 2.7.0 for template-based hard slicing (vNPU)
  - ≥ 2.9.0 for `hami-core` soft slicing (hami-vnpu-core)
  - `enpu` runtime slicing requires ubs-virt-enpu/vCANN-RT artifacts built against the workload image's CANN version

  All of these modes require `devices.ascend.enabled: true` to be set when deploying HAMi.

  HAMi v2.9.0 moved the Ascend chip list from `vnpus` to `vnpus.configs` in the `hami-scheduler-device` ConfigMap. The plugin reads both layouts, and logs a warning when it falls back to the older one, so it can be upgraded ahead of HAMi.

**Note:** `hami-vnpu-core` soft slicing currently only supports ARM platforms; template-based hard slicing has no such restriction.

## Deployment

### Label the Node with `ascend=on`

```bash
kubectl label node {ascend-node} ascend=on
```

### Deploy RuntimeClass

```bash
kubectl apply -f https://raw.githubusercontent.com/Project-HAMi/ascend-device-plugin/main/ascend-runtimeclass.yaml
```

### Deploy ConfigMap

* **HAMi and `ascend-device-plugin` in the same namespace (recommended)**: skip this step — HAMi's existing `hami-scheduler-device` ConfigMap already covers Ascend.
* **Different namespaces**: deploy the Ascend ConfigMap into `ascend-device-plugin`'s own namespace, then manually merge its `vnpus:` section into HAMi's existing `hami-scheduler-device` ConfigMap without touching HAMi's other device entries. On HAMi < v2.9.0, keep that ConfigMap's own `vnpus` list layout — its scheduler cannot read `vnpus.configs`. Keep both copies in sync whenever you change templates, resourceNames, or `hamiVnpuCore`.

  ```bash
  kubectl apply -f https://raw.githubusercontent.com/Project-HAMi/ascend-device-plugin/main/ascend-device-configmap.yaml
  ```

**Note:** `vnpus.hamiVnpuCore` and `vnpus.enpu` enable the two runtime soft-slicing backends; when both are `false`, template-based hard slicing is used. The `hami-vnpu-core` and `enpu` node settings override the global values.

#### (Optional) **Node Custom Configuration Description**

The `hami-device-node-config` is used to enable or override hami-vnpu-core for specific nodes within the cluster. Node-level settings take higher priority than the global `vnpus.hamiVnpuCore` switch.

It also supports `filterDevices` to configure devices ignored by HAMi on a specific node. By default, `filterDevices` is empty, which means no devices are ignored. A device is ignored when its UUID is listed in `uuid` or its index is listed in `index`, for example: `filterDevices: {index: [0, 1], uuid: []}`.

```bash
kubectl apply -f https://raw.githubusercontent.com/Project-HAMi/ascend-device-plugin/main/ascend-device-node-configmap.yaml
```

### Deploy `ascend-device-plugin`

```bash
kubectl apply -f https://raw.githubusercontent.com/Project-HAMi/ascend-device-plugin/main/ascend-device-plugin.yaml
```

## Usage

**Note:** Each Ascend chip model has its own `resourceName`, `resourceMemoryName`, and `resourceCoreName`; see the `hami-scheduler-device` ConfigMap for the full mapping.

**Note:** To exclusively use an entire card or request multiple cards, you only need to set the corresponding resourceName.

**Note:** How HAMi chooses soft vs template vNPU for a Pod:
- A Pod that sets `huawei.com/vnpu-mode: hami-core` always uses **soft slicing** (`libvnpu` / `hami-vnpu-core` mounts and environment) and is scheduled only onto hami-core-capable nodes.
- A Pod that **omits** the annotation is **mode-agnostic**: its effective mode **follows the node**. On a hami-core node the device plugin applies soft slicing; on a template node it uses the original vNPU path (virtualization templates and `ASCEND_VNPU_SPECS`). The plugin resolves this from the node's `hami-vnpu-core` setting (`IsHamiVnpuCore()`), so annotation-less Pods no longer remain **Pending** on hami-core-only clusters.

```yaml
...
metadata:
  name: ascend-soft-slice-pod
  annotations:
    huawei.com/vnpu-mode: 'hami-core' # Enables hami-vnpu-core soft-segmentation for this pod
spec:
  runtimeClassName: ascend
  containers:
    - name: npu_pod
      ...
      resources:
        limits:
          huawei.com/Ascend910B: "1"
          # if you don't specify Ascend910B-memory, it will use a whole NPU.
          huawei.com/Ascend910B-memory: "4096"
```

For more examples, see [examples](https://github.com/Project-HAMi/ascend-device-plugin/tree/main/examples)

### Soft Slicing Configuration (hami-vnpu-core)

Use the annotation below whenever you intend **soft** slicing; omitting it keeps **template-based vNPU** behavior (see the note above).

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: ascend-soft-slice-pod
  annotations:
    huawei.com/vnpu-mode: 'hami-core' # Enables hami-vnpu-core soft-segmentation for this pod
spec:
  runtimeClassName: ascend
  containers:
    - name: npu_pod
      ...
      resources:
        limits:
          huawei.com/Ascend910B3: "1"           # Request 1 physical NPU
          huawei.com/Ascend910B3-memory: "28672"     # Request 28Gi memory
          huawei.com/Ascend910B3-core: "40"     # Request 40% core
```

**Note:** `-core` is optional. Unlike `-memory` (which defaults to the whole NPU when omitted), omitting `-core` defaults to **0** — no dedicated compute-core reservation. It only takes effect under `huawei.com/vnpu-mode: hami-core`; setting it without that annotation is rejected.

The soft partitioning mechanism supports requesting multiple virtual devices within a same Pod. When performing multi-card parallel inference (e.g., using vLLM), the value of `--gpu-memory-utilization` must not exceed the ratio of the "container's total memory limit" to the "sum of physical memory of the selected cards".

### vCANN-RT runtime slicing (`enpu`)

Follow the [official vCANN-RT configuration examples](https://docs.openeuler.org/zh/docs/24.03_LTS_SP3/unifiedbus/unifiedbus/ubs-virt/ubs-virt-enpu/vcann-rt/README.html) for hardware/runtime prerequisites, then the [HAMi ENPU guide](../examples/enpu/README.md) for per-node enablement, model-specific ConfigMap fields and manager integration. Complete Pod YAMLs: [ordinary soft slicing](../examples/enpu/soft-slicing.yaml), [mem-swap](../examples/enpu/mem-swap.yaml). A2/910B uses the same integration without A3's single-DIE mode requirement; this requirement applies only to A3/910C. 910A is not currently included in the ENPU support scope.

`enpu` uses the ubs-virt-enpu/vCANN-RT runtime hook for compute and memory quotas. Set `huawei.com/vnpu-mode: enpu` and reuse HAMi's existing resources: for example `huawei.com/Ascend910C: "1"`, the corresponding `-memory` in MiB, and `-core` from 1 to 100 percent. One container may request a share of only one physical DIE; an omitted `-core` uses 100 percent. Set `runtimeClassName: ascend`. Administrators must install compatible Ascend drivers on the node and CANN in the workload image.

The ENPU-enabled plugin installs `libvruntime.so`, `enpu-monitor`, and `ld.so.preload` from its image into `/usr/local/enpu/vcann-rt` on the node. Identical existing files are reused; different contents cause plugin startup to fail instead of being overwritten. Follow the [runtime build, upgrade and rollback instructions](../enpu-runtime-assets/README.md) when changing versions, including stopping ENPU workloads and pausing the plugin before replacing runtime files. Keep the hami-vnpu-core assets in place.

```yaml
metadata:
  annotations:
    huawei.com/vnpu-mode: enpu
    huawei.com/enpu-policy: elastic # fixed-share, elastic, or best-effort
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

The plugin writes a per-container vCANN-RT config under `ENPU_CONFIG_ROOT` (default `/var/lib/hami-enpu`) and mounts it at `/etc/enpu/vcann-rt/npu_info.config`. It follows the official Kubernetes vCANN-RT layout by mounting the node's `/usr/local/sbin`, `/usr/local/Ascend/driver`, runtime library, monitor, `/etc/ld.so.preload`, and `/dev/shm` into the workload. The workload image must provide `systemd-detect-virt` at `/usr/bin/systemd-detect-virt`; if it does not, set `enpu.systemdDetectVirtPath` to a compatible host binary and HAMI will mount it there. This command is required before vCANN-RT can use DCMI. The `ascend` RuntimeClass remains required. The plugin sets only `ASCEND_VISIBLE_DEVICES` for the die selected by HAMi and leaves `enpu.exposeAllDevices=false`, so a workload does not receive every `/dev/davinci*` node.


On A3/910C, scheduling and quotas are per DIE. Each DIE UUID resolves to a `PhyID`; both `physical-npu-id` and `ASCEND_VISIBLE_DEVICES` use that ID, not the parent module's `CardID`. This follows the [official soft-partitioning requirement](https://www.hiascend.com/document/detail/en/mindcluster/2610/clustersched/schedulingug/docs/en/scheduling/usage/virtual_instance/virtual_instance_with_vcann_rt/01_soft_allocation_scheduling_inference.md) for `useSingleDieMode=true`. HAMi does not consume MindCluster's startup flags. Prepare an ENPU node with the equivalent driver setting:

```shell
npu-smi info -t multi-die-policy
# Run after confirming that existing workloads allow this node-wide change.
npu-smi set -t multi-die-policy -d 1
npu-smi info -t multi-die-policy # Must report INDEP_POLICY
```

Before an ENPU Ascend910C allocation, the plugin checks this prerequisite and returns an actionable error if it is missing. It does not change the node policy or expand device visibility automatically. The check does not apply to hami-core or template allocations. Applications use container-local logical device `npu:0`; no additional `ASCEND_RT_VISIBLE_DEVICES` is required.

The workload must be able to execute `systemd-detect-virt`, including all of its shared-library dependencies. Prefer installing the package from the workload image's own distribution. Mounting a binary from a different host distribution can fail due to missing dependencies or a glibc mismatch. Check `systemd-detect-virt --container` and `ldd /usr/bin/systemd-detect-virt` before running the workload.

Preview mem-swap sources must also include upstream multi-DIE fix `3d87a1d678cd9b4e9bf3b0770285dc6b53d357ee`: the CANN logical device ID must not be replaced with the DCMI card-relative chip ID. Official tag `1.0.0` includes this fix; preview checkout `1072945` does not. New swap threads and physical-memory operations must keep these ID spaces separate too. Expanding device visibility is not a substitute for this fix.

For mem-swap, set `huawei.com/enpu-memory-limit` in MiB; the limit may exceed the request when enpu-manager enables oversubscription. `huawei.com/enpu-memory-request` defaults to the memory resource allocated by HAMi and can normally be omitted. An explicit request must equal that `<chip>-memory` quota or the plugin rejects the allocation, keeping scheduler and manager reservations consistent. For example, the resource `huawei.com/Ascend910C-memory: 256` and annotation `huawei.com/enpu-memory-limit: "65536"` specify a 256 MiB request and 65536 MiB limit. Without these annotations, both retain the original memory quota. Fixed-share still requires request and limit to be equal. Set `enpu.managerURL` and point `enpu.managerConfigRoot` at the enpu-manager `config_dir` (for example `/etc/enpu`) or its already-expanded `/etc/enpu/vcann-rt` directory; both forms are accepted. The chart mounts that directory into the device-plugin when it differs from `enpu.configRoot`. HAMI keeps `/dev/shm` shared with the node because enpu-manager's `shm-id` must be visible to all Pods sharing a physical NPU. All ENPU Pods sharing a physical NPU must use the same scheduling policy. HAMI rejects mixing hami-core and ENPU tenants on one physical DIE.

ENPU is opt-in: a Pod must set `huawei.com/vnpu-mode: enpu`. An unannotated Pod is rejected on an ENPU-only node, which prevents a legacy workload from bypassing the runtime mounts.

### hami-vnpu-core: 2-Card Tensor Parallelism (TP=2) with vLLM

Assume each physical card has **64Gi** of memory, and you plan to use **32Gi** on each of the 2 cards (totaling 64Gi):

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: vllm-npu-2card
  annotations:
    huawei.com/vnpu-mode: 'hami-core' # Enable hami-vnpu-core soft partitioning
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
          --gpu-memory-utilization 0.5   # Key parameter: Total requested memory 64Gi / Total physical memory 128Gi = 0.5
      resources:
        limits:
          huawei.com/Ascend910B3: "2"           # Request 2 virtual devices for parallel computation
          huawei.com/Ascend910B3-memory: "65536" # Total memory limit for the container (64GiB combined across 2 cards)
          huawei.com/Ascend910B3-core: "50"
```

## Monitoring

When a node runs in **hami-vnpu-core (soft slicing) mode**, the device plugin starts an **embedded Prometheus exporter** on **`:9395/metrics`** that reports physical-device and per-container vNPU usage. It is **not** started for the legacy template-based vNPU (or whole-card) path, which has no soft-slice data to export.

Quick check from inside the cluster:

```bash
POD_IP=$(kubectl -n kube-system get pod -l app.kubernetes.io/component=hami-ascend-device-plugin -o jsonpath='{.items[0].status.podIP}')
curl -s $POD_IP:9395/metrics | grep hami_
```

### Exposed metrics

| Metric | Labels | Description |
| :--- | :--- | :--- |
| `hami_host_gpu_memory_used_bytes` | `device_index`, `device_uuid`, `device_type` | Physical NPU memory used (bytes) |
| `hami_host_gpu_utilization_ratio` | `device_index`, `device_uuid`, `device_type` | Physical NPU AICore utilization (0–100) |
| `hami_vgpu_memory_used_bytes` | `namespace`, `pod`, `container`, `vdevice_index`, `device_uuid` | Per-container vNPU memory used (bytes) |
| `hami_vgpu_memory_limit_bytes` | `namespace`, `pod`, `container`, `vdevice_index`, `device_uuid` | Per-container vNPU memory limit (bytes) |
| `hami_container_device_utilization_ratio` | `namespace`, `pod`, `container`, `vdevice_index`, `device_uuid` | AICore utilization of the device the container runs on (0–100) |
