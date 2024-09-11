ARG BASE_IMAGE=ubuntu:20.04

FROM $BASE_IMAGE AS build
RUN apt update -y && apt install -y gcc make wget
ARG GO_VERSION=1.22.5
RUN wget https://golang.google.cn/dl/go$GO_VERSION.linux-arm64.tar.gz
RUN rm -rf /usr/local/go && tar -C /usr/local -xzf go$GO_VERSION.linux-arm64.tar.gz
ENV PATH=/usr/local/go/bin:$PATH
ARG GOPROXY
ARG VERSION
ADD . /build
RUN --mount=type=cache,target=/go/pkg/mod \
    cd /build && make all

FROM $BASE_IMAGE
ENV LD_LIBRARY_PATH /usr/local/Ascend/driver/lib64:/usr/local/Ascend/driver/lib64/driver:/usr/local/Ascend/driver/lib64/common
COPY --from=build /build/ascend-device-plugin /usr/local/bin/ascend-device-plugin

ENTRYPOINT ["ascend-device-plugin"]
