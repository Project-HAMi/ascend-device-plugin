GO ?= go
VERSION ?= unknown
BUILDARGS ?= -ldflags '-s -w -X github.com/Project-HAMi/ascend-device-plugin/version.version=$(VERSION)'
IMG_NAME = projecthami/ascend-device-plugin

all: ascend-device-plugin

tidy:
	$(GO) mod tidy

docker:
	docker build \
	--build-arg BASE_IMAGE=ubuntu:20.04 \
	--build-arg GOPROXY=https://goproxy.cn,direct \
	-t ${IMG_NAME}:${VERSION} .

lint:
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.61.0
	golangci-lint run

ascend-device-plugin:
	$(GO) build $(BUILDARGS) -o ./ascend-device-plugin ./cmd/main.go

clean:
	rm -rf ./ascend-device-plugin

.PHONY: all clean