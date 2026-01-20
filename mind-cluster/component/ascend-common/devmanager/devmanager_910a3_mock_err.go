/* Copyright(C) 2024. Huawei Technologies Co.,Ltd. All rights reserved.
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

// Package devmanager this for device driver manager error mock
package devmanager

import (
	"errors"

	"ascend-common/api"
	"ascend-common/devmanager/common"
)

// DeviceManager910A3MockErr common device manager mock error for Ascend910A3
type DeviceManager910A3MockErr struct {
	DeviceManagerMockErr
}

// GetDevType return mock type
func (d *DeviceManager910A3MockErr) GetDevType() string {
	return api.Ascend910A3
}

// GetHccsStatisticInfo get hccs statistic info
func (d *DeviceManager910A3MockErr) GetHccsStatisticInfo(logicID int32) (*common.HccsStatisticInfo, error) {
	return &common.HccsStatisticInfo{}, errors.New(errorMsg)
}

// GetHccsBandwidthInfo get hccs statistic info
func (d *DeviceManager910A3MockErr) GetHccsBandwidthInfo(logicID int32) (*common.HccsBandwidthInfo, error) {
	return &common.HccsBandwidthInfo{}, errors.New(errorMsg)
}
