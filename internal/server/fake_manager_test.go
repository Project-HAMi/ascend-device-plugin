/*
 * Copyright 2024 The HAMi Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package server

import (
	"github.com/Project-HAMi/ascend-device-plugin/internal/manager"
)

// FakeManager implements manager.Manager for testing.
// Each method delegates to the corresponding Func field if set;
// otherwise it returns a zero value.
type FakeManager struct {
	CommonWordFunc       func() string
	ResourceNameFunc     func() string
	VDeviceCountFunc     func() int
	UpdateDeviceFunc     func() error
	GetDevicesFunc       func() []*manager.Device
	GetDeviceByUUIDFunc  func(UUID string) *manager.Device
	GetUnHealthIDsFunc   func() []int32
	CleanupIdleVNPUsFunc func() error
	IsHamiVnpuCoreFunc   func() bool
}

func (f *FakeManager) CommonWord() string {
	if f.CommonWordFunc != nil {
		return f.CommonWordFunc()
	}
	return ""
}

func (f *FakeManager) ResourceName() string {
	if f.ResourceNameFunc != nil {
		return f.ResourceNameFunc()
	}
	return ""
}

func (f *FakeManager) VDeviceCount() int {
	if f.VDeviceCountFunc != nil {
		return f.VDeviceCountFunc()
	}
	return 0
}

func (f *FakeManager) UpdateDevice() error {
	if f.UpdateDeviceFunc != nil {
		return f.UpdateDeviceFunc()
	}
	return nil
}

func (f *FakeManager) GetDevices() []*manager.Device {
	if f.GetDevicesFunc != nil {
		return f.GetDevicesFunc()
	}
	return nil
}

func (f *FakeManager) GetDeviceByUUID(UUID string) *manager.Device {
	if f.GetDeviceByUUIDFunc != nil {
		return f.GetDeviceByUUIDFunc(UUID)
	}
	return nil
}

func (f *FakeManager) GetUnHealthIDs() []int32 {
	if f.GetUnHealthIDsFunc != nil {
		return f.GetUnHealthIDsFunc()
	}
	return nil
}

func (f *FakeManager) CleanupIdleVNPUs() error {
	if f.CleanupIdleVNPUsFunc != nil {
		return f.CleanupIdleVNPUsFunc()
	}
	return nil
}

func (f *FakeManager) IsHamiVnpuCore() bool {
	if f.IsHamiVnpuCoreFunc != nil {
		return f.IsHamiVnpuCoreFunc()
	}
	return false
}
