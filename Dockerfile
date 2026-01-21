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
RUN make all

FROM $BASE_IMAGE
ENV LD_LIBRARY_PATH=/usr/local/Ascend/driver/lib64:/usr/local/Ascend/driver/lib64/driver:/usr/local/Ascend/driver/lib64/common
COPY --from=build /build/ascend-device-plugin /usr/local/bin/ascend-device-plugin

ENTRYPOINT ["ascend-device-plugin"]
