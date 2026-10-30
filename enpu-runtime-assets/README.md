# ENPU release assets

CI and the default Dockerfile target copy these files from a public, digest-pinned runtime image and bundle them alongside hami-vnpu-core. They do not compile ENPU on every build. Generated binaries are not committed.

The default input is `ghcr.io/dynamia-ai/enpu-runtime:1.0.0-cann9.1.0-r1@sha256:9c50f27e6a361322c8eb68e4e5c0f97402c96f6c373bb94e28705f94e5ae176a`. It is anonymously readable and contains only release assets, their checksums, build metadata and the upstream license. The Dockerfile verifies the packaged checksums. Use `--build-arg ENPU_RUNTIME_IMAGE=<image>@sha256:<digest>` to select an explicitly verified replacement; avoid floating tags.

To rebuild the same official release from source and package the local output:

```sh
make docker-enpu VERSION=<image-tag>
```

`docker-enpu` runs `enpu-runtime` first, then selects the Dockerfile `runtime-local` target so the newly built files are used instead of the prebuilt image. Run it on an ARM64 Linux build host with Docker, Git and Python 3. The official image supplies the compiler, CANN and driver SDK; the build host does not need an NPU or an installed Ascend driver. The existing `lib/hami-vnpu-core` assets must also be prepared using the project's `build-vnpu` workflow; this target preserves that backend in the image. Use `make enpu-runtime` to build only the ENPU assets.

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

CI retains the existing `build-vnpu` job and downloads its artifacts for each native AMD64/ARM64 image build. ENPU assets are copied from the same pinned image for both platforms, without executing its binaries. PR and manually dispatched builds validate images without publishing. On main or version-tag pushes, both platform images must succeed before a combined manifest is published; the workflow verifies that the manifest contains both architectures and then removes its temporary digest artifacts. ENPU runtime assets remain ARM64; bundling them does not enable ENPU on AMD64. Plain `docker build` uses the prebuilt assets; use `make docker-enpu` for a source rebuild.
