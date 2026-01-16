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

// Package common for collector
package common

import (
	"ascend-common/devmanager/common"
)

// HuaWeiAIChip chip info
type HuaWeiAIChip struct {

	// CardId npu card id
	CardId int32 `json:"card_id"`
	// PhyId npu chip phy id
	PhyId int32 `json:"phy_id"`
	// DeviceID the chip physic ID
	DeviceID int32 `json:"device_id"`
	// the chip logic ID
	LogicID int32 `json:"logic_id"`
	// VDieID the vdie id
	VDieID string `json:"vdie_id"`
	// MainBoardId main board id , used to distinguish between A900A3SuperPod and A9000A3SuperPod
	MainBoardId uint32
	// ChipInfo the chip info
	ChipInfo *common.ChipInfo `json:"chip_info"`
	// BoardInfo board info of device, but not display
	BoardInfo *common.BoardInfo

	// VDevActivityInfo the activity virtual device info
	VDevActivityInfo *common.VDevActivityInfo `json:"v_dev_activity_info"`
	// VDevInfos the virtual device info
	VDevInfos *common.VirtualDevInfo `json:"v_dev_infos"`
	// PCIeBusInfo bus info
	PCIeBusInfo string
	// ElabelInfo elabel info including SN
	ElabelInfo *common.ElabelInfo `json:"elabel_info"`
}
