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

// Package devmanager this Ascend310P device manager
package devmanager

import (
	"ascend-common/devmanager/dcmi"
)

// A310PManager Ascend310P device manager
type A310PManager struct {
	dcmi.DcManager
}

// DcGetDevicePowerInfo query power by mcu interface for 310P
func (d *A310PManager) DcGetDevicePowerInfo(cardID, deviceID int32) (float32, error) {
	return d.DcGetMcuPowerInfo(cardID)
}

// DcGetMcuPowerInfo this function is only for Ascend310P
func (d *A310PManager) DcGetMcuPowerInfo(cardID int32) (float32, error) {
	return dcmi.FuncDcmiMcuGetPowerInfo(cardID)
}
