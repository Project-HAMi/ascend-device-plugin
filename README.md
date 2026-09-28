# Ascend Device Plugin

[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2FProject-HAMi%2Fascend-device-plugin.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2FProject-HAMi%2Fascend-device-plugin?ref=badge_shield)

**English** | [中文](README_cn.md)

## Introduction

This Ascend device plugin is implemented for NPU-Slicing for [HAMi](https://github.com/Project-HAMi/HAMi) and [volcano](https://github.com/volcano-sh/volcano). It supports three modes:

### 1. Template-based Hard Slicing (vNPU)

Memory slicing is supported based on virtualization template. For detailed information, check [template](https://github.com/Project-HAMi/ascend-device-plugin/blob/main/ascend-device-configmap.yaml)

### 2. Soft Slicing with Runtime Interception (hami-vnpu-core)

This project implements  a soft slicing mechanism based on `libvnpu.so` interception and `limiter` token scheduling. For detailed information, check [hami-vnpu-core](https://github.com/Project-HAMi/hami-vnpu-core)

**Note:** `hami-vnpu-core` currently only supports ARM platforms.

### 3. Soft Slicing with ubs-virt-enpu (vCANN-RT)

The `enpu` backend provides compute and memory quotas through vCANN-RT, with optional memory oversubscription through a compatible enpu-manager/runtime. It supports A2/910B and A3/910C; the single-DIE mode prerequisite applies only to A3/910C. An existing ConfigMap entry for 910A does not imply ENPU support.

**Before enabling ENPU, follow the [official vCANN-RT configuration guide](https://docs.openeuler.org/zh/docs/24.03_LTS_SP3/unifiedbus/unifiedbus/ubs-virt/ubs-virt-enpu/vcann-rt/README.html) for the hardware, driver/CANN, runtime assets and container prerequisites.** Use the [HAMi ENPU guide](examples/enpu/README.md) for integration settings and the complete [ordinary soft-slicing](examples/enpu/soft-slicing.yaml) and [mem-swap](examples/enpu/mem-swap.yaml) Pod examples. Mem-swap requires a version that includes that feature; annotations alone do not enable it in an older release.

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
docker buildx build -t $IMAGE_NAME .
```

### Build an ENPU-enabled image

CI builds official ENPU release `1.0.0` in the official ARM64 build image and bundles the verified assets alongside hami-vnpu-core. For a local build, prepare the existing hami-vnpu-core assets and run `make docker-enpu VERSION=<image-tag>` on an ARM64 Linux host with Docker, Git and Python 3. The official image includes CANN and the driver SDK, so no NPU or host driver installation is required for compilation. See [release build prerequisites and options](enpu-runtime-assets/README.md). Plain `docker build` only packages assets already prepared.

## License

[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2FProject-HAMi%2Fascend-device-plugin.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2FProject-HAMi%2Fascend-device-plugin?ref=badge_large)
