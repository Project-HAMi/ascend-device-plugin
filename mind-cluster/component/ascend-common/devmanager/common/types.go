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

// Package common define common types
package common

// MemoryInfo memory information struct
type MemoryInfo struct {
	MemorySize      uint64 `json:"memory_size"`
	MemoryAvailable uint64 `json:"memory_available"`
	Frequency       uint32 `json:"memory_frequency"`
	Utilization     uint32 `json:"memory_utilization"`
}

// HbmInfo high bandwidth memory info
type HbmInfo struct {
	MemorySize        uint64 `json:"memory_size"`        // total size,MB
	Frequency         uint32 `json:"hbm_frequency"`      // frequency MHz
	Usage             uint64 `json:"memory_usage"`       // memory usage,MB
	Temp              int32  `json:"hbm_temperature"`    // temperature
	BandWidthUtilRate uint32 `json:"hbm_bandwidth_util"` // bandwidth utilization
}

// HbmAggregateInfo more comprehensive high bandwidth memory information with ecc information
type HbmAggregateInfo struct {
	*HbmInfo
	ECCInfo *ECCInfo `json:"hbm_ecc_info"` // ECC information
}

// ChipInfo chip info
type ChipInfo struct {
	Type      string `json:"chip_type"`
	Name      string `json:"chip_name"`
	Version   string `json:"chip_version"`
	NpuName   string `json:"npu_name"`
	AICoreCnt int    `json:"aicore_cnt"`
}

// ChipBaseInfo all id of chip
type ChipBaseInfo struct {
	PhysicID int32
	LogicID  int32
	CardID   int32
	DeviceID int32
}

// CgoCreateVDevOut create virtual device output info
type CgoCreateVDevOut struct {
	VDevID     uint32
	PcieBus    uint32
	PcieDevice uint32
	PcieFunc   uint32
	VfgID      uint32
	Reserved   []uint8
}

// CgoCreateVDevRes create virtual device input info
type CgoCreateVDevRes struct {
	VDevID       uint32
	VfgID        uint32
	TemplateName string
	Reserved     []uint8
}

// CgoBaseResource base resource info
type CgoBaseResource struct {
	Token       uint64
	TokenMax    uint64
	TaskTimeout uint64
	VfgID       uint32
	VipMode     uint8
	Reserved    []uint8
}

// CgoComputingResource compute resource info
type CgoComputingResource struct {
	// accelator resource
	Aic     float32
	Aiv     float32
	Dsa     uint16
	Rtsq    uint16
	Acsq    uint16
	Cdqm    uint16
	CCore   uint16
	Ffts    uint16
	Sdma    uint16
	PcieDma uint16

	// memory resource, MB as unit
	MemorySize uint64

	// id resource
	EventID  uint32
	NotifyID uint32
	StreamID uint32
	ModelID  uint32

	// cpu resource
	TopicScheduleAicpu uint16
	HostCtrlCPU        uint16
	HostAicpu          uint16
	DeviceAicpu        uint16
	TopicCtrlCPUSlot   uint16

	Reserved []uint8
}

// CgoMediaResource media resource info
type CgoMediaResource struct {
	Jpegd    float32
	Jpege    float32
	Vpc      float32
	Vdec     float32
	Pngd     float32
	Venc     float32
	Reserved []uint8
}

// CgoVDevQueryInfo virtual resource special info
type CgoVDevQueryInfo struct {
	Name            string
	Status          uint32
	IsContainerUsed uint32
	Vfid            uint32
	VfgID           uint32
	ContainerID     uint64
	Base            CgoBaseResource
	Computing       CgoComputingResource
	Media           CgoMediaResource
}

// CgoVDevQueryStru virtual resource info
type CgoVDevQueryStru struct {
	VDevID    uint32
	QueryInfo CgoVDevQueryInfo
}

// CgoSocFreeResource soc free resource info
type CgoSocFreeResource struct {
	VfgNum    uint32
	VfgBitmap uint32
	Base      CgoBaseResource
	Computing CgoComputingResource
	Media     CgoMediaResource
}

// CgoSocTotalResource soc total resource info
type CgoSocTotalResource struct {
	VDevNum   uint32
	VDevID    []uint32
	VfgNum    uint32
	VfgBitmap uint32
	Base      CgoBaseResource
	Computing CgoComputingResource
	Media     CgoMediaResource
}

// CgoSuperPodInfo super pod info
type CgoSuperPodInfo struct {
	SdId       uint32
	ScaleType  uint32
	SuperPodId uint32
	ServerId   uint32
	Reserve    []uint32
}

// VirtualDevInfo virtual device infos
type VirtualDevInfo struct {
	TotalResource    CgoSocTotalResource
	FreeResource     CgoSocFreeResource
	VDevInfo         []CgoVDevQueryStru
	VDevActivityInfo []VDevActivityInfo
}

// DevFaultInfo device's fault info
type DevFaultInfo struct {
	EventID         int64
	LogicID         int32
	ModuleType      int8 // ModuleType prototype is dcmi node_type
	ModuleID        int8 // ModuleID prototype is dcmi node_id
	SubModuleType   int8 // SubModuleType prototype is dcmi sub_node_type
	SubModuleID     int8 // SubModuleID prototype is dcmi sub_node_id
	Severity        int8
	Assertion       int8
	AlarmRaisedTime int64
}

// DevProcessInfo device process info
type DevProcessInfo struct {
	DevProcArray []DevProcInfo
	ProcNum      int32
}

// DevProcInfo process info in device side
type DevProcInfo struct {
	Pid int32
	// the total amount of memory occupied by the device side OS and allocated by the business, unit is MB
	MemUsage float64
}

// BoardInfo board info of device
type BoardInfo struct {
	BoardId uint32
	PcbId   uint32
	BomId   uint32
	SlotId  uint32
}

// VDevActivityInfo vNPU activity info for 310P
type VDevActivityInfo struct {
	VDevID         uint32
	VDevAiCoreRate uint32
	VDevTotalMem   uint64
	VDevUsedMem    uint64
	VDevAiCore     float64
	IsVirtualDev   bool
}

// PCIEBwStat contains pcie bandwidth
type PCIEBwStat struct {
	PcieRxPBw   PcieStatValue
	PcieRxNPBw  PcieStatValue
	PcieRxCPLBw PcieStatValue
	PcieTxPBw   PcieStatValue
	PcieTxNPBw  PcieStatValue
	PcieTxCPLBw PcieStatValue
}

// PcieStatValue pcie stat three value, like [min_bw,max_bw,avg_bw]
type PcieStatValue struct {
	PcieMinBw int32
	PcieMaxBw int32
	PcieAvgBw int32
}

// DeviceNetworkHealth dcmi_get_device_network_health api return value
type DeviceNetworkHealth struct {
	HealthCode uint32
	RetCode    int32
}

// ECCInfo dcmi_get_device_ecc_info api return value
type ECCInfo struct {
	EnableFlag                int32
	SingleBitErrorCnt         int64
	DoubleBitErrorCnt         int64
	TotalSingleBitErrorCnt    int64
	TotalDoubleBitErrorCnt    int64
	SingleBitIsolatedPagesCnt int64
	DoubleBitIsolatedPagesCnt int64
}

// NpuNetInfo network info of npu
type NpuNetInfo struct {
	// The optical info
	OpticalInfo *OpticalInfo
	// The transfer rate of network port
	LinkSpeedInfo *LinkSpeedInfo
	// Historical link statistics of network ports
	LinkStatInfo *LinkStatInfo
	// Statistics about packets
	StatInfo *StatInfo
	// Network port real-time bandwidth
	BandwidthInfo *BandwidthInfo
	// LinkStatusInfo refers to the link state
	LinkStatusInfo *LinkStatusInfo
}

// BandwidthInfo contains network port real-time bandwidth
type BandwidthInfo struct {
	// TxValue transform speed
	TxValue float64 `json:"tx_value"`
	// RxValue receive speed
	RxValue float64 `json:"rx_value"`
}

// HccsStatisticInfo contains hccs statistic info
type HccsStatisticInfo struct {
	TxCnt            []uint64
	RxCnt            []uint64
	CrcErrCnt        []uint64
	retryCnt         []uint64
	reservedFieldCnt []uint64
}

// HccsBandwidthInfo contains hccs bandwidth info
type HccsBandwidthInfo struct {
	ProfilingTime uint32
	TotalTxbw     float64
	TotalRxbw     float64
	TxBandwidth   []float64
	RxBandwidth   []float64
}

// SioCrcErrStatisticInfo contains sio crc error statistic info
type SioCrcErrStatisticInfo struct {
	TxErrCnt int64
	RxErrCnt int64
	Reserved []uint32
}

// StatInfo the statistics about packets
type StatInfo struct {
	// Total number of pause frames received by the MAC
	MacRxPauseNum float64
	// Total number of pause frames sent by MAC
	MacTxPauseNum float64
	// Total number of PFC frames received by MAC
	MacRxPfcPktNum float64
	// Total number of PFC frames sent by MAC
	MacTxPfcPktNum float64
	// Total number of bad packets received by MAC
	MacRxBadPktNum float64
	// Total number of bad packets sent by MAC
	MacTxBadPktNum float64
	// The total number of packets received by the RoCE network card
	RoceRxAllPktNum float64
	// The total number of packets sent by the RoCE network card
	RoceTxAllPktNum float64
	// The number of bad packets received by the RoCE network card
	RoceRxErrPktNum float64
	// The number of bad packets sent by the RoCE network card
	RoceTxErrPktNum float64
	// The number of CNP type packets received by the RoCE network card
	RoceRxCnpPktNum float64
	// The number of CNP type packets sent by the RoCE network card
	RoceTxCnpPktNum float64
	// Number of RoCE network card retry messages
	RoceNewPktRtyNum float64
	// Total number of bytes of bad packets sent by MAC
	MacTxBadOctNum float64
	// Total number of bytes of bad packets received by MAC
	MacRxBadOctNum float64
	// The number of unexpected ACK messages received by the RoCE network card
	RoceUnexpectedAckNum float64
	// The number of out-of-order packets received by the RoCE network card
	RoceOutOfOrderNum float64
	// The number of packets with domain segment verification errors received by the RoCE network card
	RoceVerificationErrNum float64
	// The number of messages generated by abnormal QP connection status received by the RoCE network card
	RoceQpStatusErrNum float64
	// The number of ecn
	RoceEcnDBNum float64
	// The number of err info
	MacRXFcsErrPktNum float64
}

// LinkStatInfo refers to the historical link statistics, including the times of link-up
type LinkStatInfo struct {
	// The times of link-up
	LinkUPNum float64
}

// LinkStatusInfo refers to the link state
type LinkStatusInfo struct {
	// The state of link
	LinkState string
}

// LinkSpeedInfo the transfer rate of network port
type LinkSpeedInfo struct {
	// The rate of network port
	Speed float64
}

// OpticalInfo indicates the optical module information
type OpticalInfo struct {
	// Optical module status, indicating whether it is in place (present)
	OpticalState float64
	// Power sent by No.0 optical module
	OpticalTxPower0 float64
	// Power sent by No.1 optical module
	OpticalTxPower1 float64
	// Power sent by No.2 optical module
	OpticalTxPower2 float64
	// Power sent by No.3 optical module
	OpticalTxPower3 float64
	// Reception power of No.0 optical module
	OpticalRxPower0 float64
	// Reception power of No.1 optical module
	OpticalRxPower1 float64
	// Reception power of No.2 optical module
	OpticalRxPower2 float64
	// Reception power of No.3 optical module
	OpticalRxPower3 float64
	// Optical module voltage
	OpticalVcc float64
	// Optical module temperature
	OpticalTemp float64
}

// HccspingMeshOperate refers to the operation of hccsping mesh
type HccspingMeshOperate struct {
	DstAddr      string
	PktSize      int
	PktSendNum   int
	PktInterval  int
	Timeout      int
	TaskInterval int
	TaskId       int
}

// HccspingMeshInfo refers to the result of hccsping mesh
type HccspingMeshInfo struct {
	DstAddr      []string
	SucPktNum    []uint
	FailPktNum   []uint
	MaxTime      []int
	MinTime      []int
	AvgTime      []int
	TP95Time     []int
	ReplyStatNum []int
	PingTotalNum []int
	DestNum      int
}

// ElabelInfo elabel information structure
type ElabelInfo struct {
	ProductName      string
	Model            string
	Manufacturer     string
	ManufacturerDate string
	SerialNumber     string
}
