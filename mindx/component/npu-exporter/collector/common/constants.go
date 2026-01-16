/* Copyright(C) 2021-2025. Huawei Technologies Co.,Ltd. All rights reserved.
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

// Package common for general constants
package common

// metric label name
const (
	npuID       = "id"
	modelName   = "model_name"
	npuUUID     = "vdie_id"
	npuPCIEInfo = "pcie_bus_info"
	namespace   = "namespace"
	podName     = "pod_name"
	cntrName    = "container_name"
)

const (
	// Healthy status of Health
	Healthy = "Healthy"
	// UnHealthy status of unhealth
	UnHealthy = "UnHealthy"
	// Abnormal status of Abnormal
	Abnormal = "Abnormal"

	// LinkUp npu interface up
	LinkUp = "UP"
	// LinkDown npu interface down
	LinkDown = "DOWN"

	// Base convert base
	Base = 10
	// ContainerNameLen container name length
	ContainerNameLen = 3
	// npuListCacheKey Cache key
	npuListCacheKey = "npu-exporter-npu-list"
	// Cache key for parsing-device result
	containersDevicesCacheKey = "npu-exporter-containers-devices"
	initSize                  = 8
	tickerFailedPattern       = "%s ticker failed, task shutdown"
	// UpdateCachePattern Update cache pattern
	UpdateCachePattern     = "update Cache,key is %s"
	connectRefusedMaxRetry = 3
)

const (
	cacheSize = 128
	// NameSpaceIdx is the index of namespace in container name
	NameSpaceIdx = 0
	// PodNameIdx is the index of pod name in container name
	PodNameIdx = 1
	// ConNameIdx is the index of container name in container name
	ConNameIdx = 2

	// DecimalPlaces is the decimal places of float64
	DecimalPlaces = 2
	// BitSize is the bit size of float64
	BitSize = 64
	// GeneralDevTagKey is the default value of devTagKey in telegraf, it means the metric is not related to any device
	GeneralDevTagKey = "GeneralDevTagKey"
)

// log limit domains for metrics
const (
	// DomainForLogicIdErr domain for faild to get cardId and deviceId by logicID
	DomainForLogicIdErr = "logicID"

	// DomainForHccs domain for hccs
	DomainForHccs = "hccs"

	// DomainForDDR domain for DDR
	DomainForDDR = "DDR"

	// DomainForSio domain for sio
	DomainForSio = "sio"

	// DomainForHBM domain for HBM
	DomainForHBM = "hbm"

	// DomainForHBMECC domain for hbmEcc
	DomainForHBMECC = "hbmEcc"

	// DomainForHccsBW domain for hccs bandwidth
	DomainForHccsBW = "hccsBw"

	// DomainForOptical domain for Optical
	DomainForOptical = "optical"

	// DomainForLinkState domain for linkState
	DomainForLinkState = "linkState"

	// DomainForBandwidth domain for bandwidth
	DomainForBandwidth = "bandwidth"

	// DomainForLinkStat domain for linkStat
	DomainForLinkStat = "linkStat"

	// DomainForLinkSpeed domain for linkSpeed
	DomainForLinkSpeed = "linkSpeed"

	// DomainForRoce domain for roce
	DomainForRoce = "roce"

	// DomainForMcuPower domain for mcu power
	DomainForMcuPower = "mcuPower"

	// DomainForChipPower domain for chip power
	DomainForChipPower = "chipPower"

	// DomainForAICoreUtilization domain for ai core utilization
	DomainForAICoreUtilization = "AICoreUtilization"

	// DomainForVectorCoreUtilization domain for vector core utilization
	DomainForVectorCoreUtilization = "vectorCoreUtilization"

	// DomainForProcess domain for process info
	DomainForProcess = "processInfo"

	// DomainForHbmUtilization domain for High Bandwidth Memory Utilization
	DomainForHbmUtilization = "hbmUtilization"

	// DomainForOverallUtilization domain for overall utilization
	DomainForOverallUtilization = "overallUtilization"

	// DomainForPcieBandwidth domain for pcie bandwidth
	DomainForPcieBandwidth = "pcieBandwidth"
	// DomainForContainerInfo domain for pcie container info
	DomainForContainerInfo = "containerInfo"
)
