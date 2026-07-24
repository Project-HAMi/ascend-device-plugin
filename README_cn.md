# Ascend Device Plugin

[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2FProject-HAMi%2Fascend-device-plugin.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2FProject-HAMi%2Fascend-device-plugin?ref=badge_shield)

[English](README.md) | **中文**

## 说明

Ascend device plugin 是用来支持在 [HAMi](https://github.com/Project-HAMi/HAMi) 和 [volcano](https://github.com/volcano-sh/volcano) 中调度昇腾 NPU 设备，支持以下两种模式：

### 1. 基于模板的硬切分 (vNPU)

支持基于虚拟化模板的显存切分，系统会自动使用最小可用模板。详细信息请参阅 [template](https://github.com/Project-HAMi/ascend-device-plugin/blob/main/ascend-device-configmap.yaml)。

### 2. 基于运行时拦截的软切分 (hami-vnpu-core)

实现了基于 `libvnpu.so` 拦截和 limiter 令牌调度的软切分机制，能够实现精细化的资源共享。详细信息请参阅 [hami-vnpu-core](https://github.com/Project-HAMi/hami-vnpu-core)。

**注意：** `hami-vnpu-core` 目前只支持 ARM 平台。

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
