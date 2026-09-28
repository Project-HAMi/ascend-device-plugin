# Ascend Device Plugin

[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2FProject-HAMi%2Fascend-device-plugin.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2FProject-HAMi%2Fascend-device-plugin?ref=badge_shield)

[English](README.md) | **中文**

## 说明

Ascend device plugin 是用来支持在 [HAMi](https://github.com/Project-HAMi/HAMi) 和 [volcano](https://github.com/volcano-sh/volcano) 中调度昇腾 NPU 设备，支持以下三种模式：

### 1. 基于模板的硬切分 (vNPU)

支持基于虚拟化模板的显存切分，系统会自动使用最小可用模板。详细信息请参阅 [template](https://github.com/Project-HAMi/ascend-device-plugin/blob/main/ascend-device-configmap.yaml)。

### 2. 基于运行时拦截的软切分 (hami-vnpu-core)

实现了基于 `libvnpu.so` 拦截和 limiter 令牌调度的软切分机制，能够实现精细化的资源共享。详细信息请参阅 [hami-vnpu-core](https://github.com/Project-HAMi/hami-vnpu-core)。

**注意：** `hami-vnpu-core` 目前只支持 ARM 平台。

### 3. 基于 ubs-virt-enpu 的软切分 (vCANN-RT)

`enpu` 后端通过 vCANN-RT 提供算力和显存配额；配套支持 mem-swap 的 runtime 与 enpu-manager 后，可启用显存超分。本适配支持 A2/910B 和 A3/910C，独立 DIE 模式要求仅针对 A3/910C；ConfigMap 中存在 910A 配置不代表 ENPU 支持 910A。

**启用 ENPU 前，请先按 [vCANN-RT 官方配置文档](https://docs.openeuler.org/zh/docs/24.03_LTS_SP3/unifiedbus/unifiedbus/ubs-virt/ubs-virt-enpu/vcann-rt/README.html) 准备硬件、驱动/CANN、运行库和容器环境。** HAMi 的配置差异见 [ENPU 使用说明](examples/enpu/README_cn.md)，完整 Pod 示例包括 [普通软切分](examples/enpu/soft-slicing.yaml) 和 [mem-swap 显存超分](examples/enpu/mem-swap.yaml)。超分需要包含该功能的版本，不能只靠注解启用旧 release 未包含的能力。

## 部署与使用

不同调度器所需的环境要求、部署步骤和使用示例并不完全相同：

- [HAMi](docs/hami_cn.md)
- [Volcano](docs/volcano_cn.md)

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

### 构建包含 ENPU 的镜像

CI 使用官方 ARM64 编译镜像构建 ENPU release `1.0.0`，校验产物后与 hami-vnpu-core 一起打包。若需本地构建，在具备 Docker、Git 和 Python 3 的 ARM64 Linux 构建机上，先准备原有 hami-vnpu-core 产物，再执行 `make docker-enpu VERSION=<image-tag>`。官方镜像已包含 CANN 和驱动 SDK，编译不需要 NPU 硬件或宿主机驱动。前提与参数见 [release 构建说明](enpu-runtime-assets/README.md)。直接执行 `docker build` 只打包已准备好的产物。
