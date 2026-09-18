/*
 * Copyright 2026 The HAMi Authors.
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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ascend-common/devmanager"
	"ascend-common/devmanager/common"

	"github.com/Project-HAMi/ascend-device-plugin/internal"
)

// TestVDeviceCount verifies that VDeviceCount honors a per-node override
// (nodeConfig.VDeviceCount) and otherwise falls back to the global default.
func TestVDeviceCount(t *testing.T) {
	// A representative Ascend910B4 config: 32768 MiB allocatable, smallest
	// template 8192 MiB -> global default = 32768/8192 = 4 vDevices/card.
	cfg910B4 := internal.VNPUConfig{
		CommonWord:        "Ascend910B4",
		MemoryAllocatable: 32768,
		Templates: []internal.Template{
			{Name: "vir03_1c_8g", Memory: 8192, AICore: 5},
			{Name: "vir06_1c_16g", Memory: 16384, AICore: 10},
			{Name: "vir12_3c_32g", Memory: 32768, AICore: 20},
		},
	}

	tests := []struct {
		name       string
		config     internal.VNPUConfig
		nodeConfig *internal.NodeConfig
		want       int
	}{
		{
			name:       "no node config -> global default (32768/8192=4)",
			config:     cfg910B4,
			nodeConfig: nil,
			want:       4,
		},
		{
			name:       "node config VDeviceCount=0 -> falls back to global default",
			config:     cfg910B4,
			nodeConfig: &internal.NodeConfig{Name: "node-001", VDeviceCount: 0},
			want:       4,
		},
		{
			name:       "node config VDeviceCount=8 -> honored (the fix)",
			config:     cfg910B4,
			nodeConfig: &internal.NodeConfig{Name: "node-001", VDeviceCount: 8},
			want:       8,
		},
		{
			name:       "node config VDeviceCount=2 -> honored, tracks the value",
			config:     cfg910B4,
			nodeConfig: &internal.NodeConfig{Name: "node-001", VDeviceCount: 2},
			want:       2,
		},
		{
			name:       "no templates, no node config -> 1",
			config:     internal.VNPUConfig{CommonWord: "AscendX", MemoryAllocatable: 32768},
			nodeConfig: nil,
			want:       1,
		},
		{
			name:       "no templates but node override present -> node override wins",
			config:     internal.VNPUConfig{CommonWord: "AscendX", MemoryAllocatable: 32768},
			nodeConfig: &internal.NodeConfig{Name: "node-001", VDeviceCount: 5},
			want:       5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			am := &AscendManager{
				config:     tt.config,
				nodeConfig: tt.nodeConfig,
			}
			if got := am.VDeviceCount(); got != tt.want {
				t.Fatalf("VDeviceCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

// chipInfoDeviceManager reports a fixed chip so LoadConfig can pick an entry
// without a real NPU.
type chipInfoDeviceManager struct {
	*devmanager.DeviceManagerMock
	chipName string
}

func (d *chipInfoDeviceManager) GetValidChipInfo() (common.ChipInfo, error) {
	return common.ChipInfo{Type: "Ascend", Name: d.chipName}, nil
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "device-config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

// TestLoadConfigAcrossHAMiVersions checks that the manager resolves its own
// chip out of a multi-chip list in either layout, so that upgrading the plugin
// ahead of HAMi does not break startup.
func TestLoadConfigAcrossHAMiVersions(t *testing.T) {
	// Two chips so that picking the right entry, not just the first one, is
	// actually exercised. 910B3: 65536/16384 = 4 vDevices; 910B4: 32768/8192 = 4.
	const chips = `
- chipName: 910B3
  commonWord: Ascend910B3
  resourceName: huawei.com/Ascend910B3
  memoryAllocatable: 65536
  templates:
  - {name: vir10_3c_32g, memory: 32768}
  - {name: vir05_1c_16g, memory: 16384}
- chipName: 910B4
  commonWord: Ascend910B4
  resourceName: huawei.com/Ascend910B4
  memoryAllocatable: 32768
  templates:
  - {name: vir05_1c_8g, memory: 8192}
`
	// The same chips under `vnpus.configs:` need one more level of indentation.
	currentLayout := "vnpus:\n  hamiVnpuCore: true\n  configs:" + strings.ReplaceAll(chips, "\n", "\n  ")
	legacyLayout := "vnpus:" + chips

	tests := []struct {
		name             string
		content          string
		chipName         string
		wantCommonWord   string
		wantResourceName string
		wantHamiVnpuCore bool
	}{
		{
			name:             "current layout",
			content:          currentLayout,
			chipName:         "910B3",
			wantCommonWord:   "Ascend910B3",
			wantResourceName: "huawei.com/Ascend910B3",
			wantHamiVnpuCore: true,
		},
		{
			name:             "legacy layout, second entry",
			content:          legacyLayout,
			chipName:         "910B4",
			wantCommonWord:   "Ascend910B4",
			wantResourceName: "huawei.com/Ascend910B4",
			wantHamiVnpuCore: false,
		},
		{
			name:             "legacy layout with top-level hamiVnpuCore",
			content:          "hamiVnpuCore: true\n" + legacyLayout,
			chipName:         "910B3",
			wantCommonWord:   "Ascend910B3",
			wantResourceName: "huawei.com/Ascend910B3",
			wantHamiVnpuCore: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			am := &AscendManager{mgr: &chipInfoDeviceManager{chipName: tt.chipName}}
			if err := am.LoadConfig(writeConfig(t, tt.content)); err != nil {
				t.Fatalf("LoadConfig() error = %v, want nil", err)
			}
			if got := am.CommonWord(); got != tt.wantCommonWord {
				t.Errorf("CommonWord() = %q, want %q", got, tt.wantCommonWord)
			}
			if got := am.ResourceName(); got != tt.wantResourceName {
				t.Errorf("ResourceName() = %q, want %q", got, tt.wantResourceName)
			}
			if got := am.VDeviceCount(); got != 4 {
				t.Errorf("VDeviceCount() = %d, want 4", got)
			}
			if got := am.IsHamiVnpuCore(); got != tt.wantHamiVnpuCore {
				t.Errorf("IsHamiVnpuCore() = %v, want %v", got, tt.wantHamiVnpuCore)
			}
		})
	}
}

// TestLoadConfigChipNotFound keeps the "no entry for this chip" path distinct
// from a parse failure in both layouts.
func TestLoadConfigChipNotFound(t *testing.T) {
	path := writeConfig(t, "vnpus:\n- chipName: 910B3\n  commonWord: Ascend910B3\n")
	am := &AscendManager{mgr: &chipInfoDeviceManager{chipName: "310P3"}}
	err := am.LoadConfig(path)
	if err == nil {
		t.Fatal("LoadConfig() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), "310P3") {
		t.Errorf("LoadConfig() error = %v, want it to name the missing chip", err)
	}
}

// TestDeviceCoreScaling checks that a per-node override wins over the global
// ratio and that an invalid override still reaches AdvertisedDevcore's
// validation instead of silently inheriting the global ratio.
func TestDeviceCoreScaling(t *testing.T) {
	const hardwareAICore = 20

	tests := []struct {
		name        string
		global      float64
		nodeConfig  *internal.NodeConfig
		wantScaling float64
		wantDevcore int32
	}{
		{
			name:        "no node config -> global ratio",
			global:      2,
			nodeConfig:  nil,
			wantScaling: 2,
			wantDevcore: 200,
		},
		{
			name:        "node override unset -> global ratio",
			global:      2,
			nodeConfig:  &internal.NodeConfig{Name: "node-001"},
			wantScaling: 2,
			wantDevcore: 200,
		},
		{
			name:        "node override 1.5 -> honored over the global ratio",
			global:      2,
			nodeConfig:  &internal.NodeConfig{Name: "node-001", DeviceCoreScaling: 1.5},
			wantScaling: 1.5,
			wantDevcore: 150,
		},
		{
			name:        "negative node override -> percentage base, not the global ratio",
			global:      2,
			nodeConfig:  &internal.NodeConfig{Name: "node-001", DeviceCoreScaling: -1},
			wantScaling: -1,
			wantDevcore: 100,
		},
		{
			name:        "node override below 1 -> percentage base",
			global:      2,
			nodeConfig:  &internal.NodeConfig{Name: "node-001", DeviceCoreScaling: 0.5},
			wantScaling: 0.5,
			wantDevcore: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			am := &AscendManager{
				nodeConfig:   tt.nodeConfig,
				globalConfig: internal.Config{VNPUs: internal.VNPUsConfig{DeviceCoreScaling: tt.global}},
			}
			if got := am.DeviceCoreScaling(); got != tt.wantScaling {
				t.Fatalf("DeviceCoreScaling() = %v, want %v", got, tt.wantScaling)
			}
			got := internal.AdvertisedDevcore(true, am.DeviceCoreScaling(), hardwareAICore)
			if got != tt.wantDevcore {
				t.Fatalf("AdvertisedDevcore(true, %v, %d) = %d, want %d",
					am.DeviceCoreScaling(), hardwareAICore, got, tt.wantDevcore)
			}
		})
	}
}
