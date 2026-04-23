ARG BASE_IMAGE=ubuntu:20.04
FROM $BASE_IMAGE AS build

ENV DEBIAN_FRONTEND=noninteractive
RUN apt update -y && apt install -y gcc make wget software-properties-common
RUN add-apt-repository ppa:longsleep/golang-backports
RUN apt update
RUN apt install -y golang-1.22
ENV PATH=/usr/lib/go-1.22/bin:/usr/local/go/bin:/root/go/bin:$PATH
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

COPY . .
RUN echo "=== Current directory: $(pwd) ===" && ls -la
RUN echo "=== Searching for limiter binary ===" && \
    find . -name "limiter" -type f 2>/dev/null || echo "limiter not found"

COPY ./lib/hami-vnpu-core/ /usr/local/hami-vnpu-core-assets/

ENTRYPOINT ["ascend-device-plugin"]
