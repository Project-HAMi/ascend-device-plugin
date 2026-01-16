// Copyright (c) Huawei Technologies Co., Ltd. 2025-2025. All rights reserved.

// Package api common const
package api

// Env
const (
	NodeNameEnv = "NODE_NAME"

	// PtWorldSizeEnv the total number of npu used for the task for PyTorch
	PtWorldSizeEnv = "WORLD_SIZE"
	// PtLocalWorldSizeEnv number of npu used per pod for PyTorch
	PtLocalWorldSizeEnv = "LOCAL_WORLD_SIZE"
	// PtLocalRankEnv logic id List of npu used by pod for PyTorch
	PtLocalRankEnv = "LOCAL_RANK"

	// TfWorkerSizeEnv the total number of npu used for the task for TensorFlow
	TfWorkerSizeEnv = "CM_WORKER_SIZE"
	// TfLocalWorkerEnv number of npu used per pod for TensorFlow
	TfLocalWorkerEnv = "CM_LOCAL_WORKER"

	// MsWorkerNumEnv the total number of npu used for the task for MindSpore
	MsWorkerNumEnv = "MS_WORKER_NUM"
	// MsLocalWorkerEnv number of npu used per pod for MindSpore
	MsLocalWorkerEnv = "MS_LOCAL_WORKER"
)

// NameSpace
const (
	DLNamespace = "mindx-dl"
	ClusterNS   = "cluster-system"
	KubeNS      = "kube-system"
)

// Node
const (
	// NPUChipMemoryLabel label value is npu chip memory
	NPUChipMemoryLabel = "mind-cluster/npu-chip-memory"

	// NodeSNAnnotation annotation value is node sn
	NodeSNAnnotation = "product-serial-number"
	// BaseDevInfoAnno annotation value is device base info
	BaseDevInfoAnno = "baseDeviceInfos"

	// AcceleratorTypeKey the node label key of accelerator type
	AcceleratorTypeKey = "accelerator-type"
	// AcceleratorTypeModule910A3SuperPod for 910A3-SuperPod hardware
	AcceleratorTypeModule910A3SuperPod = "module-a3-16-super-pod"
)

// Pod
const (
	// PodUsedHardwareTypeAnno annotation value is the hardware type that real used in pod
	PodUsedHardwareTypeAnno = "mind-cluster/hardware-type"
	// PodRankIndexAnno annotation value is rank index of the pod
	PodRankIndexAnno = "hccl/rankIndex"
	// SuperPodIDAnno annotation key of the super pod id
	SuperPodIDAnno = "super-pod-id"

	// Hotswitch Annotations

	// InHotSwitchFlowKey in hot switch flow key
	InHotSwitchFlowKey = "inHotSwitchFlow"
	// InHotSwitchFlowValue in hot switch flow true
	InHotSwitchFlowValue = "true"
	// BackupNewPodNameKey backup new pod name key
	BackupNewPodNameKey = "backupNewPodName"
	// BackupSourcePodNameKey backup source pod name key
	BackupSourcePodNameKey = "backupSourcePodName"
	// NeedOperatorOpeKey need operator ope key
	NeedOperatorOpeKey = "needOperatorOpe"
	// NeedVolcanoOpeKey need volcano ope key
	NeedVolcanoOpeKey = "needVolcanoOpe"
	// OpeTypeDelete ope type delete
	OpeTypeDelete = "delete"
	// OpeTypeCreate ope type create
	OpeTypeCreate = "create"
	// PodTypeKey pod type key
	PodTypeKey = "podType"
	// PodTypeBackup pod type backup
	PodTypeBackup = "backup"
	// DefaultRetryTimes default retry times
	DefaultRetryTimes = 3
	// MasterPodRank master pod rank
	MasterPodRank = "0"
)

// PodGroup
const (
	// AtlasTaskLabel label value task kind, eg. ascend-910, ascend-{xxx}b
	AtlasTaskLabel = "ring-controller.atlas"
)

// ConfigMap
const (
	// DeviceInfoCMDataKey device-info-cm data key, record device info
	DeviceInfoCMDataKey = "DeviceInfoCfg"
	// SwitchInfoCMDataKey device-info-cm data key, record switch info
	SwitchInfoCMDataKey = "SwitchInfoCfg"
	// NodeInfoCMDataKey node-info-cm data key, record node info
	NodeInfoCMDataKey = "NodeInfo"
	// PubFaultCMDataKey public fault cm data key, record public fault info
	PubFaultCMDataKey = "PublicFault"

	// CIMCMLabelKey cm label key, who uses these cms
	CIMCMLabelKey = "mx-consumer-cim"
	// PubFaultCMLabelKey public fault cm label key
	PubFaultCMLabelKey = "mc-consumer-publicfault"
)

const (
	// FaultJobCmName fault job cm name
	FaultJobCmName = "fault-job-info"
)

const (
	// PodScheduleLabel pod schedule label
	PodScheduleLabel = "pod-rescheduling"
	// ProcessScheduleLabel process schedule label
	ProcessScheduleLabel = "process-recover-enable"
	// RecoverStrategyKey recover strategy key in job annotation
	RecoverStrategyKey = "recover-strategy"
)

// process schedule strategy
const (
	// RecoverStrategy recover strategy
	RecoverStrategy = "recover"
	// RetryStrategy retry strategy
	RetryStrategy = "retry"
	// InPlaceStrategy recover in place strategy
	InPlaceStrategy = "recover-in-place"
	// DumpStrategy dump strategy
	DumpStrategy = "dump"
	// ExitStrategy exit strategy
	ExitStrategy = "exit"
	// ElasticTraining elastic-training strategy
	ElasticTraining = "elastic-training"
)

// process schedule common env
const (
	// ProcessRecoverEnv process recover env
	ProcessRecoverEnv = "PROCESS_RECOVER"
	// ElasticRecoverEnv elastic process recover env
	ElasticRecoverEnv = "ELASTIC_PROCESS_RECOVER_ENABLE"
	// EnableRestartEnv enable restart env
	EnableRestartEnv = "ENABLE_RESTART_FAULT_PROCESS"
)

// process schedule pytorch env
const (
	// HighAvailableEnv high available env
	HighAvailableEnv = "HIGH_AVAILABILITY"
	// PtCloseWatchDogKey pt close watch dog key
	PtCloseWatchDogKey = "HCCL_ASYNC_ERROR_HANDLING"
	// PtCloseWatchDogValue pt close watch dog value
	PtCloseWatchDogValue = "0"
)

// process schedule ms env
const (
	// MsRecoverEnv ms recover env
	MsRecoverEnv = "MS_ENABLE_TFT"
	// EnableMS enable ms
	EnableMS = "MINDIO_FOR_MINDSPORE"
	// MsDumpStrategy ms dump strategy
	MsDumpStrategy = "TTP:1"
	// MsUceStrategy ms uce strategy
	MsUceStrategy = "UCE:1"
	// MsArfStrategy ms arf strategy
	MsArfStrategy = "ARF:1"
	// MsHcceStrategy ms hcce strategy
	MsHcceStrategy = "HCCE:1"
	// MsRscStrategy ms rsc strategy
	MsRscStrategy = "RSC:1"
	// MsCloseWatchDogKey ms close watch dog key
	MsCloseWatchDogKey = "MS_ENABLE_THM"
	// MsCloseWatchDogValue ms close watch dog value
	MsCloseWatchDogValue = `{HCCL_WATCHDOG:0}`
)

const (
	//EnableFunc Enable Func
	EnableFunc = "on"
	// EnableFlag enable flag
	EnableFlag = "1"
	// PytorchFramework framework
	PytorchFramework = "pytorch"
	// MindSporeFramework framework
	MindSporeFramework = "mindspore"
)

const (
	// RescheduleInPlaceKey reschedule in place key
	RescheduleInPlaceKey = "reschedule-in-place"
	// RescheduleInPlaceValue reschedule in place value
	RescheduleInPlaceValue = "true"
)

const (
	// DeviceResetTimeout device reset timeout
	DeviceResetTimeout = "deviceResetTimeout"
	// DefaultDeviceResetTimeout default device reset timeout is 60 seconds
	DefaultDeviceResetTimeout = 60
	// MinDeviceResetTimeout min device reset timeout is 10 seconds
	MinDeviceResetTimeout = 10
	// MaxDeviceResetTimeout max device reset timeout is 600 seconds
	MaxDeviceResetTimeout = 600
)

const (
	// SubHealthyStrategy config in pod group label for subHealthy fault strategy
	SubHealthyStrategy = "subHealthyStrategy"
	// SubHealthyHotSwitch strategy name of hot switch
	SubHealthyHotSwitch = "hotSwitch"
)

const (
	// MinAvailableKey decide minAvailable of task
	MinAvailableKey = "huawei.com/schedule_minAvailable"
)
