/* Copyright(C) 2021. Huawei Technologies Co.,Ltd. All rights reserved.
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

// Package common this for util method
package common

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"ascend-common/api"
	"ascend-common/common-utils/hwlog"
)

var (
	reg910A = regexp.MustCompile(api.Ascend910APattern)
	reg910B = regexp.MustCompile(api.Ascend910BPattern)
	reg310P = regexp.MustCompile(api.Ascend310PPattern)
)

// IsGreaterThanOrEqualInt32 check num range
func IsGreaterThanOrEqualInt32(num int64) bool {
	if num >= int64(math.MaxInt32) {
		return true
	}

	return false
}

// IsValidUtilizationRate valid utilization rate is 0-100
func IsValidUtilizationRate(num uint32) bool {
	if num > uint32(Percent) || num < 0 {
		return false
	}

	return true
}

// IsValidChipInfo valid chip info is or not empty
func IsValidChipInfo(chip *ChipInfo) bool {
	return chip.Name != "" || chip.Type != "" || chip.Version != ""
}

// IsValidBoardInfo check whether the board info is valid
func IsValidBoardInfo(board *BoardInfo) bool {
	return board.BoardId != InvalidID || board.PcbId != InvalidID ||
		board.BomId != InvalidID || board.SlotId != InvalidID
}

// IsValidMainBoardInfo check whether the mainBoardId is valid
func IsValidMainBoardInfo(mainBoardId uint32) bool {
	return mainBoardId != InvalidID
}

// IsValidCardID valid card id
func IsValidCardID(cardID int32) bool {
	// for cardID, please watch the maximum value of the driver is changed in the future version
	return cardID >= 0 && cardID < HiAIMaxCardID
}

// IsValidDeviceID valid device id
func IsValidDeviceID(deviceID int32) bool {
	return deviceID >= 0 && deviceID < HiAIMaxDeviceNum
}

// IsValidLogicIDOrPhyID valid logic id
func IsValidLogicIDOrPhyID(id int32) bool {
	return id >= 0 && id < HiAIMaxCardNum*HiAIMaxDeviceNum
}

// IsValidCardIDAndDeviceID check two params both needs meet the requirement
func IsValidCardIDAndDeviceID(cardID, deviceID int32) bool {
	if !IsValidCardID(cardID) {
		return false
	}

	return IsValidDeviceID(deviceID)
}

// IsValidDevNumInCard valid devNum in card
func IsValidDevNumInCard(num int32) bool {
	return num > 0 && num <= HiAIMaxDeviceNum
}

// IsValidVDevID valid vir device id
func IsValidVDevID(vDevID uint32) bool {
	return vDevID >= MinVDevID && vDevID < MaxVDevID
}

// IsValidPortID valid port id
func IsValidPortID(portID int) bool {
	return portID == DefaultPingMeshPortID
}

// IsValidTaskID valid task id
func IsValidTaskID(taskID uint) bool {
	return taskID == InternalPingMeshTaskID || taskID == ExternalPingMeshTaskID
}

// IsValidHccspingMeshOperate valid hccsping mesh operate
func IsValidHccspingMeshOperate(operate HccspingMeshOperate) error {
	if len(operate.DstAddr) > MaxHccspingMeshAddr {
		return fmt.Errorf("dst addr length %d is invalid, should not be greater than %d", len(operate.DstAddr),
			MaxHccspingMeshAddr)
	}
	if operate.PktSize < MinPktSize || operate.PktSize > MaxPktSize {
		return fmt.Errorf("pkt size %d is invalid, should be between %d and %d", operate.PktSize, MinPktSize, MaxPktSize)
	}
	if operate.PktSendNum < MinPktSendNum || operate.PktSendNum > MaxPktSendNum {
		return fmt.Errorf("pkt send num %d is invalid, should be between %d and %d", operate.PktSendNum,
			MinPktSendNum, MaxPktSendNum)
	}
	if operate.PktInterval < MinPktInterval || operate.PktInterval > MaxPktInterval {
		return fmt.Errorf("pkt interval %d is invalid, should be between %d and %d", operate.PktInterval,
			MinPktInterval, MaxPktInterval)
	}
	if operate.TaskInterval < MinTaskInterval || operate.TaskInterval > MaxTaskInterval {
		return fmt.Errorf("task interval %d is invalid, should be between %d and %d", operate.TaskInterval,
			MinTaskInterval, MaxTaskInterval)
	}
	if !IsValidTaskID(uint(operate.TaskId)) {
		return fmt.Errorf("task id %d is invalid", operate.TaskId)
	}
	return nil
}

// GetDeviceTypeByChipName get device type by chipName
func GetDeviceTypeByChipName(chipName string) string {
	if reg310P.MatchString(chipName) {
		return api.Ascend310P
	}
	if strings.Contains(chipName, api.Ascend310BNo) {
		return api.Ascend310B
	}
	if strings.Contains(chipName, api.Ascend310No) {
		return api.Ascend310
	}
	if reg910B.MatchString(chipName) {
		return api.Ascend910B
	}
	if reg910A.MatchString(chipName) {
		return api.Ascend910A
	}
	return ""
}

func get910TemplateNameList() map[string]struct{} {
	return map[string]struct{}{"vir16": {}, "vir08": {}, "vir04": {}, "vir02": {}, "vir01": {}}
}

func get910BTemplateNameList() map[string]struct{} {
	return map[string]struct{}{
		"vir03_1c_8g": {}, "vir05_1c_8g": {}, "vir05_1c_16g": {},
		"vir06_1c_16g": {}, "vir10_3c_16g": {}, "vir10_3c_16g_nm": {},
		"vir10_3c_32g": {}, "vir10_4c_16g_m": {}, "vir12_3c_32g": {}}
}

func get310PTemplateNameList() map[string]struct{} {
	return map[string]struct{}{"vir04": {}, "vir02": {}, "vir01": {}, "vir04_3c": {}, "vir02_1c": {},
		"vir04_4c_dvpp": {}, "vir04_3c_ndvpp": {}}
}

// IsValidTemplateName check template name meet the requirement
func IsValidTemplateName(devType, templateName string) bool {
	isTemplateNameValid := false
	switch devType {
	case api.Ascend310P:
		_, isTemplateNameValid = get310PTemplateNameList()[templateName]
	case api.Ascend910A:
		_, isTemplateNameValid = get910TemplateNameList()[templateName]
	case api.Ascend910B:
		_, isTemplateNameValid = get910BTemplateNameList()[templateName]
	default:
	}
	return isTemplateNameValid
}

// RemoveDuplicate remove duplicate device
func RemoveDuplicate(list *[]string) []string {
	listValueMap := make(map[string]string, len(*list))
	var rmDupValueList []string
	for _, value := range *list {
		listValueMap[value] = value
	}
	for _, value := range listValueMap {
		rmDupValueList = append(rmDupValueList, value)
	}
	return rmDupValueList
}

// GetNpuName get npu name eg: name-type-version
func GetNpuName(chipInfo *ChipInfo) string {
	if chipInfo == nil {
		return ""
	}
	if len(chipInfo.Name) == 0 && len(chipInfo.Type) == 0 && len(chipInfo.Version) == 0 {
		return ""
	}
	return fmt.Sprintf("%s-%s-%s", chipInfo.Name, chipInfo.Type, chipInfo.Version)
}

// SetExternalParams transmit npu-exporter's startup parameters
func SetExternalParams(profilingTime int) {
	ProfilingTime = profilingTime
}

// SetHccsBWProfilingTime set hccs bw profiling time
func SetHccsBWProfilingTime(hccsbwProfilingTime int) {
	HccsBWProfilingTime = hccsbwProfilingTime
}

// DeepCopyChipInfo copy chip info deeply
func DeepCopyChipInfo(chipInfo *ChipInfo) *ChipInfo {
	if chipInfo == nil {
		return nil
	}

	return &ChipInfo{
		Type:    chipInfo.Type,
		Name:    chipInfo.Name,
		Version: chipInfo.Version,
	}
}

// DeepCopyVDevActivityInfo copy VDevActivityInfo deeply
func DeepCopyVDevActivityInfo(vDevActivityInfo *VDevActivityInfo) *VDevActivityInfo {
	if vDevActivityInfo == nil {
		return nil
	}

	return &VDevActivityInfo{
		VDevID:         vDevActivityInfo.VDevID,
		VDevAiCoreRate: vDevActivityInfo.VDevAiCoreRate,
		VDevTotalMem:   vDevActivityInfo.VDevTotalMem,
		VDevUsedMem:    vDevActivityInfo.VDevUsedMem,
		VDevAiCore:     vDevActivityInfo.VDevAiCore,
		IsVirtualDev:   vDevActivityInfo.IsVirtualDev,
	}
}

// DeepCopySlice Deep copy slice
func deepCopySlice(slice interface{}) interface{} {

	switch v := slice.(type) {
	case []int:
		newSlice := make([]int, len(v))
		copy(newSlice, v)
		return newSlice
	case []uint32:
		newSlice := make([]uint32, len(v))
		copy(newSlice, v)
		return newSlice
	case []float64:
		newSlice := make([]float64, len(v))
		copy(newSlice, v)
		return newSlice
	default:
		hwlog.RunLog.Warn("Unsupported slice type")
		return slice
	}
}

// GetDevType get device type by chip name,boardId
func GetDevType(chipName string, boardId uint32) string {
	var devType string
	if Is910A3Chip(boardId) {
		devType = api.Ascend910A3
	} else {
		devType = GetDeviceTypeByChipName(chipName)
	}
	return devType
}

// Is910A3Chip current chip is 910A3 or not,include A900A3 and A9000A3
func Is910A3Chip(boardId uint32) bool {
	return a3BoardIds.Has(int32(boardId))
}

// IsA900A3SuperPod current product is A900A3 super pod or not
func IsA900A3SuperPod(mainBoardId uint32) bool {
	return a900A3SuperPodMainBoardIds.Has(int32(mainBoardId))
}

// IsA9000A3SuperPod current product is A9000A3 super pod or not
func IsA9000A3SuperPod(mainBoardId uint32) bool {
	return a9000A3SuperPodMainBoardIds.Has(int32(mainBoardId))
}

// Is800IA3Chip current chip is 800IA3 or not
func Is800IA3Chip(mainBoardId uint32) bool {
	return mainBoardId == A800IA3MainBoardId
}
