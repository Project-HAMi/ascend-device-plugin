ARG BASE_IMAGE=ubuntu:20.04
ARG ENPU_RUNTIME_IMAGE=ghcr.io/dynamia-ai/enpu-runtime:1.0.0-cann9.1.0-r1@sha256:9c50f27e6a361322c8eb68e4e5c0f97402c96f6c373bb94e28705f94e5ae176a

# Data-only stage: the ARM64 runtime is installed on ENPU nodes, not executed here.
FROM --platform=linux/arm64 ${ENPU_RUNTIME_IMAGE} AS enpu-runtime

FROM $BASE_IMAGE AS build

ARG GO_VERSION=1.26.2
ARG TARGETARCH
ENV DEBIAN_FRONTEND=noninteractive
RUN apt update -y && apt install -y gcc make wget ca-certificates
# Install the official Go toolchain (matches go.mod). Avoids the fragile
# longsleep/golang-backports PPA, which depends on Launchpad API availability
# and flakes in CI (especially on the emulated arm64 leg).
RUN wget -qO /tmp/go.tgz "https://go.dev/dl/go${GO_VERSION}.linux-${TARGETARCH}.tar.gz" \
    && tar -C /usr/local -xzf /tmp/go.tgz \
    && rm /tmp/go.tgz
ENV PATH=/usr/local/go/bin:/root/go/bin:$PATH
ARG GOPROXY
ENV GOPATH=/go
WORKDIR /build
# Cache module downloads independently of application source changes.
COPY go.mod go.sum ./
COPY mind-cluster/component/ascend-common ./mind-cluster/component/ascend-common
COPY mind-cluster/component/npu-exporter ./mind-cluster/component/npu-exporter
RUN go mod download
COPY Makefile ./
COPY cmd ./cmd
COPY internal ./internal
COPY version ./version
ARG VERSION
RUN make all

FROM $BASE_IMAGE AS runtime-base
ENV LD_LIBRARY_PATH=/usr/local/Ascend/driver/lib64:/usr/local/Ascend/driver/lib64/driver:/usr/local/Ascend/driver/lib64/common
COPY --from=build /build/ascend-device-plugin /usr/local/bin/ascend-device-plugin
COPY lib/hami-vnpu-core/* /usr/local/hami-vnpu-core-assets/
ENTRYPOINT ["ascend-device-plugin"]

# Explicit source-rebuild path used by make docker-enpu.
FROM runtime-base AS runtime-local
COPY enpu-runtime-assets/ /usr/local/enpu-runtime-assets/
RUN cd /usr/local/enpu-runtime-assets && \
    test -s libvruntime.so && test -s enpu-monitor && test -s ld.so.preload && \
    test -s build-info.json && test -s LICENSE.ubs-virt && sha256sum -c SHA256SUMS

# Default path: reuse the pinned, publicly readable release assets.
FROM runtime-base AS runtime
COPY --from=enpu-runtime /enpu-runtime/ /usr/local/enpu-runtime-assets/
RUN cd /usr/local/enpu-runtime-assets && \
    test -s libvruntime.so && test -s enpu-monitor && test -s ld.so.preload && \
    test -s build-info.json && test -s LICENSE.ubs-virt && sha256sum -c SHA256SUMS
