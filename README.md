# Ascend Device Plugin

[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2FProject-HAMi%2Fascend-device-plugin.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2FProject-HAMi%2Fascend-device-plugin?ref=badge_shield)

**English** | [中文](README_cn.md)

## Introduction

This Ascend device plugin is implemented for NPU-Slicing for [HAMi](https://github.com/Project-HAMi/HAMi) and [volcano](https://github.com/volcano-sh/volcano). It supports two modes:

### 1. Template-based Hard Slicing (vNPU)

Memory slicing is supported based on virtualization template. For detailed information, check [template](https://github.com/Project-HAMi/ascend-device-plugin/blob/main/ascend-device-configmap.yaml)

### 2. Soft Slicing with Runtime Interception (hami-vnpu-core)

This project implements  a soft slicing mechanism based on `libvnpu.so` interception and `limiter` token scheduling. For detailed information, check [hami-vnpu-core](https://github.com/Project-HAMi/hami-vnpu-core)

**Note:** `hami-vnpu-core` currently only supports ARM platforms.

## Deployment & Usage

Prerequisites, deployment steps and usage examples differ depending on which scheduler you use:

- [HAMi](docs/hami.md)
- [Volcano](docs/volcano.md)

## Compile

update submodule:

```bash
git submodule update --init --recursive
```

```bash
make all
```

### Build image

```bash
docker buildx build -t $IMAGE_NAME . --load
```

## License

[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2FProject-HAMi%2Fascend-device-plugin.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2FProject-HAMi%2Fascend-device-plugin?ref=badge_large)
