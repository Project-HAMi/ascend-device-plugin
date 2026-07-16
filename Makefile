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

.PHONY: update-chart-docs
update-chart-docs:
	cd charts/ascend-device-plugin && helm-docs --skip-version-footer
	cd charts/ascend-device-plugin && $(GO) run github.com/losisin/helm-values-schema-json@v1.9.2 -input values.yaml -output values.schema.json

.PHONY: verify-helm-chart
verify-helm-chart:
	$(MAKE) update-chart-docs
	git diff --exit-code -- charts/ascend-device-plugin/README.md charts/ascend-device-plugin/values.schema.json
	helm lint charts/ascend-device-plugin
	helm template ascend-device-plugin charts/ascend-device-plugin >/dev/null

clean:
	rm -rf ./ascend-device-plugin

.PHONY: all tidy test lint clean update-chart-docs verify-helm-chart
