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

package manager

import (
	"errors"
	"testing"

	"ascend-common/devmanager"
	"ascend-common/devmanager/dcmi"
	"github.com/Project-HAMi/ascend-device-plugin/internal"
)

type update950DeviceManager struct {
	*devmanager.DeviceManagerMock
	cardCalls int
	dieTypes  []dcmi.DieType
}

func (d *update950DeviceManager) GetDeviceList() (int32, []int32, error) {
	return 2, []int32{0, 7}, nil
}

func (d *update950DeviceManager) GetPhysicIDFromLogicID(logicID int32) (int32, error) {
	return logicID, nil
}

func (d *update950DeviceManager) GetCardIDDeviceID(int32) (int32, int32, error) {
	d.cardCalls++
	return -1, -1, errors.New("DCMI v2 does not support card/device coordinates")
}

func (d *update950DeviceManager) GetDieID(logicID int32, dieType dcmi.DieType) (string, error) {
	d.dieTypes = append(d.dieTypes, dieType)
	return "950-die-" + string(rune('0'+logicID)), nil
}

func (d *update950DeviceManager) GetDeviceHealth(int32) (uint32, error) {
	return 0, nil
}

func test950Manager() *AscendManager {
	return &AscendManager{
		config:       internal.VNPUConfig{ChipName: "Ascend950DT", CommonWord: "Ascend950DT", ResourceName: "huawei.com/Ascend950DT", MemoryAllocatable: 98304, MemoryCapacity: 98304},
		globalConfig: internal.Config{VNPUs: internal.VNPUsConfig{HamiVnpuMode: "hami-core", DeviceSplitCount: 4}},
		nodeConfig:   &internal.NodeConfig{EnableExperimental950: true, DeviceShareMode: "npu-id", VDeviceCount: 4},
	}
}

func Test950Validation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		change  func(*AscendManager)
		arch    string
		invalid bool
	}{
		{"explicit valid config", func(*AscendManager) {}, "arm64", false},
		{"disabled", func(m *AscendManager) { m.nodeConfig.EnableExperimental950 = false }, "arm64", true},
		{"template", func(m *AscendManager) { m.globalConfig.VNPUs.HamiVnpuMode = "template" }, "arm64", true},
		{"node override", func(m *AscendManager) { m.nodeConfig.HamiVnpuMode = "enpu" }, "arm64", true},
		{"unknown memory", func(m *AscendManager) { m.config.MemoryAllocatable = 0 }, "arm64", true},
		{"over capacity", func(m *AscendManager) { m.config.MemoryCapacity = 1 }, "arm64", true},
		{"no share mode", func(m *AscendManager) { m.nodeConfig.DeviceShareMode = "" }, "arm64", true},
		{"no slots", func(m *AscendManager) { m.nodeConfig.VDeviceCount = 0 }, "arm64", true},
		{"x86 assets unavailable", func(*AscendManager) {}, "amd64", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := test950Manager()
			tc.change(m)
			if err := m.validateExperimental950(tc.arch); (err != nil) != tc.invalid {
				t.Fatalf("validation = %v, invalid=%v", err, tc.invalid)
			}
		})
	}
}

func Test950LoadConfigAndSlotOverride(t *testing.T) {
	m := &AscendManager{mgr: &chipInfoDeviceManager{chipName: "Ascend950DT_957c"}}
	config := "vnpus:\n  hamiVnpuMode: hami-core\n  deviceSplitCount: 4\n  configs:\n  - chipName: Ascend950DT\n    commonWord: Ascend950DT\n"
	if err := m.LoadConfig(writeConfig(t, config)); err != nil {
		t.Fatal(err)
	}
	if m.CommonWord() != "Ascend950DT" || m.VDeviceCount() != 4 {
		t.Fatal("950 match/slot count incorrect")
	}
	m.nodeConfig = &internal.NodeConfig{VDeviceCount: 6}
	if m.VDeviceCount() != 6 {
		t.Fatal("node override lost")
	}
	if err := m.CleanupIdleVNPUs(); err != nil {
		t.Fatal(err)
	}
}

func Test950UpdateDeviceUsesDCMIV2IDs(t *testing.T) {
	fake := &update950DeviceManager{}
	m := &AscendManager{
		mgr:    fake,
		config: internal.VNPUConfig{ChipName: "Ascend950DT", MemoryAllocatable: 98304},
	}
	if err := m.UpdateDevice(); err != nil {
		t.Fatal(err)
	}
	if fake.cardCalls != 0 {
		t.Fatalf("GetCardIDDeviceID called %d times for DCMI v2", fake.cardCalls)
	}
	for _, dieType := range fake.dieTypes {
		if dieType != dcmi.DDIE {
			t.Fatalf("950 queried die type %d, want DDIE", dieType)
		}
	}
	devices := m.GetDevices()
	if len(devices) != 2 || devices[1].LogicID != 7 || devices[1].PhyID != 7 {
		t.Fatalf("unexpected devices: %+v", devices)
	}
	if devices[1].CardID != -1 || devices[1].DeviceID != -1 || devices[1].Memory != 98304 {
		t.Fatalf("unexpected DCMI v2 device metadata: %+v", devices[1])
	}
}
