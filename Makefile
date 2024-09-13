GO ?= go
VERSION ?= unknown
BUILDARGS ?= -ldflags '-s -w -X github.com/Project-HAMi/ascend-device-plugin/version.version=$(VERSION)'

all: ascend-device-plugin

tidy:
	$(GO) mod tidy

lint: tidy
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.61.0
	golangci-lint run

ascend-device-plugin: tidy
	$(GO) build $(BUILDARGS) -o ./ascend-device-plugin

clean:
	rm -rf ./ascend-device-plugin

.PHONY: all clean