module github.com/Project-HAMi/ascend-device-plugin

go 1.22.2

require (
	github.com/Project-HAMi/HAMi v0.0.0
	github.com/fsnotify/fsnotify v1.7.0
	google.golang.org/grpc v1.63.2
	huawei.com/npu-exporter/v6 v6.0.0-RC3.b001
	k8s.io/api v0.29.3
	k8s.io/apimachinery v0.29.3
	k8s.io/klog/v2 v2.120.1
	k8s.io/kubelet v0.29.3
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/emicklei/go-restful/v3 v3.11.3 // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-openapi/jsonpointer v0.20.2 // indirect
	github.com/go-openapi/jsonreference v0.20.4 // indirect
	github.com/go-openapi/swag v0.22.9 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/gnostic-models v0.6.8 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/imdario/mergo v0.3.16 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/onsi/ginkgo/v2 v2.17.1 // indirect
	github.com/onsi/gomega v1.32.0 // indirect
	github.com/smartystreets/goconvey v1.7.2 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	golang.org/x/net v0.26.0 // indirect
	golang.org/x/oauth2 v0.17.0 // indirect
	golang.org/x/sys v0.21.0 // indirect
	golang.org/x/term v0.21.0 // indirect
	golang.org/x/text v0.16.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	google.golang.org/appengine v1.6.8 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240227224415-6ceb2ff114de // indirect
	google.golang.org/protobuf v1.33.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/client-go v0.29.3 // indirect
	k8s.io/kube-openapi v0.0.0-20240227032403-f107216b40e2 // indirect
	k8s.io/utils v0.0.0-20240102154912-e7106e64919e // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.1 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

replace (
	github.com/Project-HAMi/HAMi v0.0.0 => github.com/Project-HAMi/HAMi v0.0.0-20250107033239-d04fc8baaad6
	huawei.com/npu-exporter/v6 => gitee.com/ascend/ascend-npu-exporter/v6 v6.0.0-RC3
)
