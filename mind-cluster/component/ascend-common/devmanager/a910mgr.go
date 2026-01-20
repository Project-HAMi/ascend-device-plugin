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

// Package devmanager this Ascend910 device manager
package devmanager

import (
	"ascend-common/devmanager/common"
	"ascend-common/devmanager/dcmi"
)

// A910Manager Ascend910 device manager
type A910Manager struct {
	dcmi.DcManager
}

// DcGetHbmInfo get HBM information, only for Ascend910
func (d *A910Manager) DcGetHbmInfo(cardID, deviceID int32) (*common.HbmInfo, error) {
	return dcmi.FuncDcmiGetDeviceHbmInfo(cardID, deviceID)
}
