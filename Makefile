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
	@set -eu; \
	manifest="$$(mktemp)"; \
	trap 'rm -f "$$manifest"' EXIT; \
	helm lint --strict charts/ascend-device-plugin; \
	helm template ascend-device-plugin charts/ascend-device-plugin >"$$manifest"; \
	awk '/^[[:space:]]+args:$$/ { in_args = 1; next } in_args && /^[[:space:]]*$$/ { next } in_args && /^[[:space:]]+- / { value = $$0; sub(/^[[:space:]]+- /, "", value); args[++count] = value; next } in_args { in_args = 0 } END { exit !(args[1] == "--config_file" && args[2] == "/device-config.yaml" && args[3] == "--node_config_file" && args[4] == "/node-config.yaml" && args[5] == "--v=4") }' "$$manifest"; \
	if helm lint --strict charts/ascend-device-plugin --set image=null >/dev/null 2>&1; then echo 'null image value was accepted' >&2; exit 1; fi; \
	if helm lint --strict charts/ascend-device-plugin --set daemonSet=null >/dev/null 2>&1; then echo 'null daemonSet value was accepted' >&2; exit 1; fi; \
	if helm lint --strict charts/ascend-device-plugin --set image.repository= >/dev/null 2>&1; then echo 'empty image repository was accepted' >&2; exit 1; fi; \
	if helm lint --strict charts/ascend-device-plugin --set image.pullPolicy=Sometimes >/dev/null 2>&1; then echo 'invalid image pull policy was accepted' >&2; exit 1; fi; \
	if helm lint --strict charts/ascend-device-plugin --set image.unexpected=value >/dev/null 2>&1; then echo 'unknown image value was accepted' >&2; exit 1; fi; \
	if helm template ascend-device-plugin charts/ascend-device-plugin --kube-version 1.19.16 >/dev/null 2>&1; then echo 'unsupported Kubernetes version was accepted' >&2; exit 1; fi

.PHONY: verify-helm-release-path
verify-helm-release-path:
	@set -eu; \
	packages="$$(mktemp -d)"; \
	trap 'rm -rf "$$packages"' EXIT; \
	chart_version="$$(awk '$$1 == "version:" { print $$2; exit }' charts/ascend-device-plugin/Chart.yaml)"; \
	if [ -z "$$chart_version" ]; then echo 'failed to resolve chart version' >&2; exit 1; fi; \
	helm package charts/ascend-device-plugin --destination "$$packages" >/dev/null; \
	package_path="$$packages/ascend-device-plugin-$$chart_version.tgz"; \
	test -f "$$package_path"; \
	helm show chart "$$package_path" >/dev/null; \
	grep -Fq 'uses: helm/chart-releaser-action@v1.6.0' .github/workflows/build-helm-release.yaml; \
	grep -Fq 'charts_dir: charts' .github/workflows/build-helm-release.yaml; \
	if grep -Eq '^[[:space:]]*skip_packaging:[[:space:]]*true' .github/workflows/build-helm-release.yaml; then echo 'chart-releaser skip_packaging must remain disabled' >&2; exit 1; fi; \
	show_line="$$(grep -n 'helm show chart' .github/workflows/build-helm-release.yaml | cut -d: -f1)"; \
	package_line="$$(grep -n 'helm package "$${{ env.CHART_PATH }}" --destination .cr-release-packages' .github/workflows/build-helm-release.yaml | cut -d: -f1)"; \
	if [ -z "$$show_line" ] || [ -z "$$package_line" ] || [ "$$show_line" -ge "$$package_line" ]; then echo 'GHCR existence check must precede packaging' >&2; exit 1; fi

clean:
	rm -rf ./ascend-device-plugin

.PHONY: all tidy test lint clean update-chart-docs verify-helm-chart verify-helm-release-path
