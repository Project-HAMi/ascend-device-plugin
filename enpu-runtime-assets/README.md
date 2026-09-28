# ENPU release assets

CI builds these files from the official release and bundles them alongside hami-vnpu-core in the device-plugin image. Generated binaries are not committed. To build locally:

```sh
make docker-enpu VERSION=<image-tag>
```

`docker-enpu` runs `enpu-runtime` first. Run it on an ARM64 Linux build host with Docker, Git and Python 3. The official image supplies the compiler, CANN and driver SDK; the build host does not need an NPU or an installed Ascend driver. The existing `lib/hami-vnpu-core` assets must also be prepared using the project's `build-vnpu` workflow; this target preserves that backend in the image. Use `make enpu-runtime` to build only the ENPU assets.

The build is pinned to:

| Component | Version |
| --- | --- |
| Official repository | <https://gitcode.com/openeuler/ubs-virt> |
| Release tag | `1.0.0` |
| Release commit | `46b6d55a892429008b1e02f96b0a0f81d1176db8` |
| Official build image | `swr.cn-north-4.myhuaweicloud.com/ubscore/ubs-virt:oe2403-v1` |
| CANN | `9.1.0` |

The script verifies the release commit, exports its source, and runs the upstream `make_build.sh` in the official image. It validates the driver SDK inside the container and records the resolved builder image ID and SDK source. It does not copy an installed host ENPU runtime.

The output includes `libvruntime.so`, `enpu-monitor`, `ld.so.preload`, `build-info.json` and `SHA256SUMS`. The image also includes [LICENSE.ubs-virt](LICENSE.ubs-virt), copied verbatim from this release's upstream Mulan PSL v2 license. The Dockerfile bundles these files under `/usr/local/enpu-runtime-assets` and checks their hashes. On an ENPU node, the plugin installs the three runtime files under `/usr/local/enpu/vcann-rt` and mounts them into HAMi-allocated Pods.

Build options:

```sh
make docker-enpu VERSION=<image-tag> \
  ENPU_SOURCE_DIR=/path/to/clean/ubs-virt
```

`ENPU_SOURCE_DIR` is optional; without it, the script fetches the official tag. A local repository must be clean and contain the exact tag commit; its current branch is not used as the build source. `ENPU_BUILDER_IMAGE` can pin the official image by digest. To override the image's driver SDK, set `ENPU_DRIVER_PATH=/path/to/driver`; the script mounts it read-only and records the host path. These are build settings, not workload environment variables.

This release provides ordinary ENPU soft slicing, without mem-swap. To upgrade an existing node, stop its ENPU workloads and pause the device-plugin on that node. Back up and move only these three existing files out of their installation paths: `/usr/local/enpu/vcann-rt/lib/libvruntime.so`, `/usr/local/enpu/vcann-rt/tools/enpu-monitor`, and `/usr/local/enpu/vcann-rt/ld.so.preload`. Keep the hami-vnpu-core assets in place. The plugin refuses to overwrite files with different contents. Deploy the new image through the existing Helm chart and resume the plugin so it installs the verified new assets; remove any test hostPath mounts over the plugin executable or `/usr/local/enpu-runtime-assets`. For rollback, stop ENPU workloads and pause the plugin again, restore the three backed-up files and the previous plugin image, then resume the plugin.

CI runs `build-enpu` on an ARM64 runner and downloads its five generated assets alongside the existing `build-vnpu` artifacts. The image job checks that every ENPU asset is present and verifies the checksums before building the existing AMD64/ARM64 image variants. ENPU runtime assets are ARM64; bundling them does not enable ENPU on AMD64. Plain `docker build` only packages assets already prepared and does not compile ENPU automatically.
