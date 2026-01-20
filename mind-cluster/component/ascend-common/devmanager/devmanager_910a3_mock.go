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

// Package devmanager this for device driver manager mock
package devmanager

import (
	"ascend-common/api"
)

// DeviceManager910A3Mock common device manager mock for Ascend910A3
type DeviceManager910A3Mock struct {
	DeviceManagerMock
}

// GetDevType return mock type
func (d *DeviceManager910A3Mock) GetDevType() string {
	return api.Ascend910A3
}
