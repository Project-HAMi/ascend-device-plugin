ARG BASE_IMAGE=ubuntu:20.04
FROM $BASE_IMAGE AS build

ARG GO_VERSION=1.24.6
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
ARG VERSION
WORKDIR /build
ADD . .
RUN go mod download github.com/Project-HAMi/HAMi
RUN go get github.com/Project-HAMi/ascend-device-plugin/internal/server
RUN go get huawei.com/npu-exporter
RUN go get huawei.com/npu-exporter/utils/logger@v0.0.0-00010101000000-000000000000
RUN make all

FROM $BASE_IMAGE
ENV LD_LIBRARY_PATH=/usr/local/Ascend/driver/lib64:/usr/local/Ascend/driver/lib64/driver:/usr/local/Ascend/driver/lib64/common
COPY --from=build /build/ascend-device-plugin /usr/local/bin/ascend-device-plugin
COPY --from=build /build/lib/hami-vnpu-core/* /usr/local/hami-vnpu-core-assets/
RUN chmod +x /usr/local/hami-vnpu-core-assets/limiter

ENTRYPOINT ["ascend-device-plugin"]
