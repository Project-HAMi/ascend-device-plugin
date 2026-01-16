/* Copyright(C) 2021-2023. Huawei Technologies Co.,Ltd. All rights reserved.
   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

// Package common define common variable
package common

import (
	"math"

	"k8s.io/apimachinery/pkg/util/sets"
)

// DeviceType define device type
type DeviceType struct {
	// Code device type code
	Code int32
	// Name device type name
	Name string
}

var (
	// ProfilingTime for getting PCIe bandwidth
	ProfilingTime int

	// HccsBWProfilingTime for getting hccs bandwidth
	HccsBWProfilingTime int

	// a3BoardIds for A3 Board IDs
	a3BoardIds = sets.NewInt32(A900A3SuperPodBin1BoardId, A900A3SuperPodBin2BoardId,
		A900A3SuperPodBin3BoardId, A800IA3BoardId)

	// a900A3SuperPodMainBoardIds for A900 A3 Super Pod Main Board IDs
	a900A3SuperPodMainBoardIds = sets.NewInt32(A900A3SuperPodMainBoardId1, A900A3SuperPodMainBoardId2)

	// a9000A3SuperPodMainBoardIds for A9000 A3 Super Pod Main Board IDs
	a9000A3SuperPodMainBoardIds = sets.NewInt32(A9000A3SuperPodMainBoardId1, A9000A3SuperPodMainBoardId2)
)

// DeviceType for utilization
var (
	// AICore Ascend310 & Ascend910
	AICore = DeviceType{Code: 2, Name: "AICore"}
	// HbmUtilization utilization rate of hbm
	HbmUtilization = DeviceType{Code: 6, Name: "Hbm"}
	// VectorCore Ascend310P
	VectorCore = DeviceType{Code: 12, Name: "VectorCore"}
	// Overall Overall utilization rate of NPU
	Overall = DeviceType{Code: 13, Name: "Overall"}
)

// DeviceType for frequency
var (
	// AICoreCurrentFreq Ascend310 & Ascend910 & Ascend910B & Ascend310P
	AICoreCurrentFreq = DeviceType{Code: 7, Name: "AICore Current"}
)

const (
	// Success for interface return code
	Success = 0
	// DeviceNotReadyErrCodeStr for dcmi interface device not ready err code string
	DeviceNotReadyErrCodeStr = "-8012"
	// DeviceNotReadyErrCode for dcmi interface device not ready err code
	DeviceNotReadyErrCode = -8012
	// CardDropFaultCode card drop fault code
	CardDropFaultCode = 0x40F84E00
	// RetError return error when the function failed
	RetError = -1
	// Percent constant of 100
	Percent = 100
	// MaxErrorCodeCount number of error codes
	MaxErrorCodeCount = 128
	// UnRetError return unsigned int error
	UnRetError = math.MaxUint32
	// Abnormal status of Abnormal
	Abnormal = "Abnormal"
	// ChannelStateOk means out band channel is ok for resetting
	ChannelStateOk = 1

	// HiAIMaxCardID max card id for Ascend chip
	HiAIMaxCardID = math.MaxInt32

	// HiAIMaxCardNum max card number
	HiAIMaxCardNum = 64

	// HiAIMaxDeviceNum max device number
	HiAIMaxDeviceNum = 4

	// NpuType present npu chip
	NpuType = 0

	// ReduceOnePercent for calculation reduce one percent
	ReduceOnePercent = 0.01
	// ReduceTenth for calculation reduce one tenth
	ReduceTenth = 0.1
	// DefaultTemperatureWhenQueryFailed when get temperature failed, use this value
	DefaultTemperatureWhenQueryFailed = -275

	// Ascend310P ascend 310P chip
	Ascend310P = "Ascend310P"
	// Ascend910 ascend 910 chip
	Ascend910 = "Ascend910"
	// Ascend910B ascend 910B chip
	Ascend910B = "Ascend910B"
	// Ascend910A3 ascend Ascend910A3 chip
	Ascend910A3 = "Ascend910A3"
	// Atlas200ISoc 200 soc env
	Atlas200ISoc = "Atlas 200I SoC A1"

	// DcmiApiTimeout dcmi interface timeout seconds
	DcmiApiTimeout = 1

	// SubscribeAllDevice subscribe all device ID
	SubscribeAllDevice = -1
	// MinVDevID min value of virtual device id
	MinVDevID = 100
	// MaxVDevID max value of virtual device id
	MaxVDevID = 1124

	// InvalidID invalid ID
	InvalidID = 0xffffffff

	// FailedMetricValue for failed metric value
	FailedMetricValue = -1

	// FailedValue for failed value
	FailedValue = 0xffffffff

	// MaxErrorCodeLen max length of error code for Prometheus
	MaxErrorCodeLen = 10
)

const (
	// BootStartFinish chip hot reset finish
	BootStartFinish = 16
)

const (
	// FaultRecover device fault recover
	FaultRecover = int8(0)
	// FaultOccur device fault occur
	FaultOccur = int8(1)
	// FaultOnce once device fault
	FaultOnce = int8(2)
)

const (
	// AMPMode for AMP chip work mode
	AMPMode = "AMP"
	// SMPMode for SMP chip work mode
	SMPMode = "SMP"

	// NetworkInit init status
	NetworkInit = 6
	// NetworkSuccess chip network is healthy
	NetworkSuccess = 0

	// MaxProcNum process number in device side
	MaxProcNum = 32
	// UnitMB MB
	UnitMB float64 = 1024 * 1024

	// Chip910 chip name 910
	Chip910 = "910"

	// A300IA2BoardId board id of A300I A2 and 910proB
	A300IA2BoardId = 0x28

	// A300IA2GB64BoardId board id of A300I A2 64GB
	A300IA2GB64BoardId = 0x29

	// A900A3SuperPodBin1BoardId board id of A900/A9000 A3 SuperPod Bin1
	A900A3SuperPodBin1BoardId = 0xb0

	// A900A3SuperPodBin2BoardId board id of A900/A9000 A3 SuperPod Bin2
	A900A3SuperPodBin2BoardId = 0xb1

	// A900A3SuperPodBin3BoardId board id of A900/A9000 A3 SuperPod Bin3
	A900A3SuperPodBin3BoardId = 0xb2

	// A800IA3BoardId board id of A800I A3
	A800IA3BoardId = 0xb3

	// A900A3SuperPodMainBoardId1 board id of A900 A3 SuperPod MainBoard1
	A900A3SuperPodMainBoardId1 = 0x18

	// A900A3SuperPodMainBoardId2 board id of A900 A3 SuperPod MainBoard2
	A900A3SuperPodMainBoardId2 = 0x19

	// A800IA3MainBoardId A800I A3 MainBoardId
	A800IA3MainBoardId = 0x14

	// A9000A3SuperPodMainBoardId1 board id of A9000 A3 SuperPod MainBoard1
	A9000A3SuperPodMainBoardId1 = 0x1C

	// A9000A3SuperPodMainBoardId2 board id of A9000 A3 SuperPod MainBoard2
	A9000A3SuperPodMainBoardId2 = 0x1D
)

// log limit domains for metrics
const (
	// DomainForLogicIdErr domain for faild to get cardId and deviceId by logicID
	DomainForLogicIdErr = "logicID"
)

// DcmiDeviceType used to represent the dcmi device type
type DcmiDeviceType int32

const (
	// DcmiDeviceTypeDDR represents the component type DCMI_DEVICE_TYPE_DDR
	DcmiDeviceTypeDDR DcmiDeviceType = 0
	// DcmiDeviceTypeSRAM represents the component type DCMI_DEVICE_TYPE_SRAM
	DcmiDeviceTypeSRAM DcmiDeviceType = 1
	// DcmiDeviceTypeHBM represents the component type DCMI_DEVICE_TYPE_HBM
	DcmiDeviceTypeHBM DcmiDeviceType = 2
	// DcmiDeviceTypeNPU represents the component type DCMI_DEVICE_TYPE_NPU
	DcmiDeviceTypeNPU DcmiDeviceType = 3
	// DcmiDeviceTypeNONE represents the component type DCMI_DEVICE_TYPE_NONE
	DcmiDeviceTypeNONE DcmiDeviceType = 0xff
)

const (
	// ErrMsgInitCardListFailed is used where initialization of the card list fails
	ErrMsgInitCardListFailed = "get card list failed for init"
	// ErrMsgGetBoardInfoFailed is used where there is a failure in getting board info
	ErrMsgGetBoardInfoFailed = "get board info failed, no card found"
)

const (
	// MaxHccspingMeshAddr is the max number of hccsping addresses
	MaxHccspingMeshAddr = 1024
	// MinPktSize is the min packet size
	MinPktSize = 1792
	// MaxPktSize is the max packet size
	MaxPktSize = 3000
	// MinPktSendNum is the min packet send number
	MinPktSendNum = 1
	// MaxPktSendNum is the max packet send number
	MaxPktSendNum = 1000
	// MinPktInterval is the min packet interval
	MinPktInterval = 1
	// MaxPktInterval is the max packet interval
	MaxPktInterval = 1000
	// MinTaskInterval is the min task interval
	MinTaskInterval = 1
	// MaxTaskInterval is the max task interval
	MaxTaskInterval = 60
	// InternalPingMeshTaskID is the inner ping mesh task id
	InternalPingMeshTaskID uint = 0
	// ExternalPingMeshTaskID is the outer ping mesh task id
	ExternalPingMeshTaskID uint = 1
	// DefaultPingMeshPortID is the default ping mesh port
	DefaultPingMeshPortID = 0
	// DefaultPktSize is the default packet size
	DefaultPktSize = 1792
	// DefaultPktSendNum is the default packet send number
	DefaultPktSendNum = 10
	// DefaultPktInterval is the default packet interval
	DefaultPktInterval = 10
	// DefaultTimeout is the default timeout
	DefaultTimeout = 1
)
