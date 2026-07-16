GO ?= go
VERSION ?= unknown
BUILDARGS ?= -ldflags '-s -w -X github.com/Project-HAMi/ascend-device-plugin/version.version=$(VERSION)'
IMG_NAME = projecthami/ascend-device-plugin

all: ascend-device-plugin

tidy:
	$(GO) mod tidy

test:
	$(GO) test -v ./internal/...

docker:
	docker build \
	--build-arg BASE_IMAGE=ubuntu:20.04 \
	--build-arg GOPROXY=https://goproxy.cn,direct \
	-t ${IMG_NAME}:${VERSION} .

lint:
	$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.1.0
	golangci-lint run

ascend-device-plugin:
	$(GO) build $(BUILDARGS) -o ./ascend-device-plugin ./cmd/main.go

clean:
	rm -rf ./ascend-device-plugin

.PHONY: all tidy test lint clean