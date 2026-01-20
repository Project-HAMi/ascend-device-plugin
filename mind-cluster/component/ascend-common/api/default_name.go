// Copyright (c) Huawei Technologies Co., Ltd. 2025-2025. All rights reserved.

// Package api common brand moniker
package api

// common
const (
	// Pod910DeviceAnno annotation value is for generating 910 hccl rank table
	Pod910DeviceAnno = "ascend.kubectl.kubernetes.io/ascend-910-configuration"

	// ResourceNamePrefix pre resource name
	ResourceNamePrefix = "huawei.com/"
	// PodRealAlloc pod annotation key, means pod real mount device
	PodRealAlloc = "AscendReal"

	// PodAnnotationAscendReal pod annotation ascend real
	PodAnnotationAscendReal = "huawei.com/AscendReal"

	// Ascend brand name
	Ascend = "Ascend"
	// AscendJob job kind is AscendJob
	AscendJob = "AscendJob"
	// AscendJobsLowerCase for ascend jobs lowercase
	AscendJobsLowerCase = "ascendjobs"

	// AscendOperator ascend-Operator
	AscendOperator = "ascend-Operator"
)

// common 910
const (
	// Ascend910 for 910 chip
	Ascend910 = "Ascend910"
	// Ascend910Lowercase for 910 chip lowercase
	Ascend910Lowercase = "ascend910"
	// HuaweiAscend910 ascend 910 chip with prefix
	HuaweiAscend910 = "huawei.com/Ascend910"
	// Ascend910MinuxPrefix name prefix of ascend 910 chip
	Ascend910MinuxPrefix = "Ascend910-"
	// Ascend910MinuxCase minus type of ascend 910 chip
	Ascend910MinuxCase = "ascend-910"
	// Ascend910No 910 chip number
	Ascend910No = "910"
)

// common 910 A1
const (
	// Ascend910A ascend 910A chip
	Ascend910A = "Ascend910"
	// Ascend910APattern regular expression for 910A
	Ascend910APattern = `^910`
)

// common 910 A2
const (
	// Ascend910B ascend 910B chip
	Ascend910B = "Ascend910B"
	// Ascend910BPattern regular expression for 910B
	Ascend910BPattern = `^(910B\d{1}|A2G\d{1})`
)

// common 910 A3
const (
	// Ascend910A3 ascend Ascend910A3 chip
	Ascend910A3 = "Ascend910A3"
)

// common 310
const (
	// Ascend310 ascend 310 chip
	Ascend310 = "Ascend310"
	// Ascend310Lowercase ascend 310 chip lowercase
	Ascend310Lowercase = "ascend310"
	// Ascend310No 310 chip number
	Ascend310No = "310"
	// HuaweiAscend310 ascend 310 chip with prefix
	HuaweiAscend310 = "huawei.com/Ascend310"
	// Ascend310MinuxPrefix name prefix of ascend 310 chip
	Ascend310MinuxPrefix = "Ascend310-"
)

// common 310B
const (
	// Ascend310B ascend 310B chip
	Ascend310B = "Ascend310B"
	// Ascend310BNo 310B chip number
	Ascend310BNo = "310B"
)

// common 310P
const (
	// Ascend310P ascend 310P chip
	Ascend310P = "Ascend310P"
	// Ascend310PLowercase ascend 310P chip lowercase
	Ascend310PLowercase = "ascend310P"
	// Ascend310PNo 310P chip number
	Ascend310PNo = "310P"
	// Ascend310PPattern regular expression for 310P
	Ascend310PPattern = `^(310P\d{0,1}|I2\d{0,1})`
	// HuaweiAscend310P ascend 310P chip with prefix
	HuaweiAscend310P = "huawei.com/Ascend310P"
	// Ascend310PMinuxPrefix name prefix of ascend 310P chip
	Ascend310PMinuxPrefix = "Ascend310P-"
)

// device plugin
const (
	// Use310PMixedInsert use 310P Mixed insert
	Use310PMixedInsert = "use310PMixedInsert"
	// Ascend310PMix dp use310PMixedInsert parameter usage
	Ascend310PMix = "ascend310P-V, ascend310P-VPro, ascend310P-IPro"
	// A300IA2Label the value of the A300I A2 node label
	A300IA2Label = "card-910b-infer"
	// A300IDuoLabel the value of the A300I Duo node label
	A300IDuoLabel = "card-300i-duo"
	//UseAscendDocker UseAscendDocker parameter
	UseAscendDocker = "useAscendDocker"
)

// docker runtime
const (
	// AscendDockerRuntime ascend-docker-runtime
	AscendDockerRuntime = "ascend-docker-runtime"
	// AscendDockerHook ascend-docker-hook
	AscendDockerHook = "ascend-docker-hook"
	// AscendDockerDestroy ascend-docker-destroy
	AscendDockerDestroy = "ascend-docker-destroy"
	// AscendDockerCli ascend-docker-cli
	AscendDockerCli = "ascend-docker-cli"

	// AscendDockerRuntimeEnv env variable
	AscendDockerRuntimeEnv = "ASCEND_DOCKER_RUNTIME"
	// AscendVisibleDevicesEnv env variable
	AscendVisibleDevicesEnv = "ASCEND_VISIBLE_DEVICES"
	// AscendRuntimeOptionsEnv env variable
	AscendRuntimeOptionsEnv = "ASCEND_RUNTIME_OPTIONS"
	// AscendRuntimeMountsEnv env variable
	AscendRuntimeMountsEnv = "ASCEND_RUNTIME_MOUNTS"
	// AscendAllowLinkEnv env variable
	AscendAllowLinkEnv = "ASCEND_ALLOW_LINK"
	// AscendVnpuSpescEnv env variable
	AscendVnpuSpescEnv = "ASCEND_VNPU_SPECS"

	// RunTimeLogDir dir path of runtime
	RunTimeLogDir = "/var/log/ascend-docker-runtime/"
	// HookRunLogPath run log path of hook
	HookRunLogPath = "/var/log/ascend-docker-runtime/hook-run.log"
	// InstallHelperRunLogPath run log path of install helper
	InstallHelperRunLogPath = "/var/log/ascend-docker-runtime/install-helper-run.log"
	// RunTimeRunLogPath run log path of runtime
	RunTimeRunLogPath = "/var/log/ascend-docker-runtime/runtime-run.log"

	// RunTimeDConfigPath config path
	RunTimeDConfigPath = "/etc/ascend-docker-runtime.d"
)

// npu exporter
const (
	// DevicePathPattern device path pattern
	DevicePathPattern = `^/dev/davinci\d+$`
	// HccsBWProfilingTimeStr  preset parameter name
	HccsBWProfilingTimeStr = "hccsBWProfilingTime"
	// Hccs log options domain value
	Hccs = "hccs"
	// Prefix pre statistic info
	Prefix = "npu_chip_info_hccs_statistic_info_"
	// BwPrefix pre bandwidth info
	BwPrefix = "npu_chip_info_hccs_bandwidth_info_"
	// AscendDeviceInfo
	AscendDeviceInfo = "ASCEND_VISIBLE_DEVICES"
)

const (
	// AscendJobKind is the kind name
	AscendJobKind = "AscendJob"
	// DefaultContainerName the default container name for AscendJob.
	DefaultContainerName = "ascend"
	// DefaultPortName is name of the port used to communicate between other process.
	DefaultPortName = "ascendjob-port"
	// ControllerName is the name of controller,used in log.
	ControllerName = "ascendjob-controller"
	// OperatorName name of operator
	OperatorName = "ascend-operator"
	// LogModuleName name of log module
	LogModuleName = "hwlog"
	// OperatorLogFilePath Operator log file name
	OperatorLogFilePath = "/var/log/mindx-dl/ascend-operator/ascend-operator.log"
)
