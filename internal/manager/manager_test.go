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
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ascend-common/devmanager"
	"ascend-common/devmanager/common"
	"ascend-common/devmanager/dcmi"

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

func TestVDeviceCountSoftSlicing(t *testing.T) {
	config := internal.VNPUConfig{MemoryAllocatable: 32768, Templates: []internal.Template{{Memory: 8192}}}
	for _, tc := range []struct {
		name, mode string
		splitCount int
		node       *internal.NodeConfig
		want       int
	}{
		{name: "configured core count", mode: "hami-core", splitCount: 7, want: 7},
		{name: "core default", mode: "hami-core", want: 10},
		{name: "negative count falls back", mode: "hami-core", splitCount: -1, want: 10},
		{name: "node enables core", mode: "template", splitCount: 7, node: &internal.NodeConfig{HamiVnpuMode: "hami-core"}, want: 7},
		{name: "node count overrides core", mode: "hami-core", splitCount: 7, node: &internal.NodeConfig{VDeviceCount: 3}, want: 3},
		{name: "zero node count inherits core", mode: "hami-core", splitCount: 7, node: &internal.NodeConfig{VDeviceCount: 0}, want: 7},
		{name: "node selects template", mode: "hami-core", splitCount: 7, node: &internal.NodeConfig{HamiVnpuMode: "template"}, want: 4},
		{name: "template ignores split count", mode: "template", splitCount: 7, want: 4},
		{name: "configured ENPU count", mode: "enpu", splitCount: 7, want: 7},
		{name: "ENPU default", mode: "enpu", want: 10},
		{name: "ENPU negative count falls back", mode: "enpu", splitCount: -1, want: 10},
		{name: "ENPU global limit", mode: "enpu", splitCount: 101, want: 100},
		{name: "ENPU exact global limit", mode: "enpu", splitCount: 100, want: 100},
		{name: "ENPU node override", mode: "enpu", splitCount: 7, node: &internal.NodeConfig{VDeviceCount: 3}, want: 3},
		{name: "core count above ENPU limit", mode: "hami-core", splitCount: 101, want: 101},
		{name: "node selects ENPU", mode: "hami-core", splitCount: 7, node: &internal.NodeConfig{HamiVnpuMode: "enpu"}, want: 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			am := &AscendManager{
				config: config,
				globalConfig: internal.Config{VNPUs: internal.VNPUsConfig{
					HamiVnpuMode:     tc.mode,
					DeviceSplitCount: tc.splitCount,
				}},
				nodeConfig: tc.node,
			}
			if got := am.VDeviceCount(); got != tc.want {
				t.Fatalf("VDeviceCount() = %d, want %d", got, tc.want)
			}
		})
	}
	t.Run("core without templates", func(t *testing.T) {
		am := &AscendManager{globalConfig: internal.Config{VNPUs: internal.VNPUsConfig{HamiVnpuMode: "hami-core", DeviceSplitCount: 7}}}
		if got := am.VDeviceCount(); got != 7 {
			t.Fatalf("VDeviceCount() = %d, want 7", got)
		}
	})
}

func TestVDeviceCountENPUSlotLimit(t *testing.T) {
	config := internal.VNPUConfig{MemoryAllocatable: 32768, Templates: []internal.Template{{Memory: 8192}}}
	for _, tc := range []struct {
		name, globalMode, nodeMode string
		count, want                int
	}{
		{"ENPU default", "enpu", "", 0, 10},
		{"ENPU inherits with count override", "enpu", "", 101, 100},
		{"node enables ENPU", "hami-core", "enpu", 200, 100},
		{"ENPU exact limit", "enpu", "", 100, 100},
		{"ENPU smaller override", "enpu", "", 32, 32},
		{"node selects core", "enpu", "hami-core", 200, 200},
		{"node selects template", "enpu", "template", 200, 200},
		{"core override", "hami-core", "", 200, 200},
		{"template override", "template", "", 200, 200},
		{"core default count", "hami-core", "", 0, 10},
		{"template count", "template", "", 0, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			am := &AscendManager{
				config:       config,
				globalConfig: internal.Config{VNPUs: internal.VNPUsConfig{HamiVnpuMode: tc.globalMode}},
				nodeConfig:   &internal.NodeConfig{HamiVnpuMode: tc.nodeMode, VDeviceCount: tc.count},
			}
			if got := am.VDeviceCount(); got != tc.want {
				t.Fatalf("VDeviceCount() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestNodeModeOverridesGlobal(t *testing.T) {
	for _, tc := range []struct {
		name, global, node, want string
	}{
		{"global ENPU", "hamiVnpuMode: enpu", "", "enpu"},
		{"global core alias", "hamiVnpuMode: hamiCore", "", "hami-core"},
		{"legacy global core", "hamiVnpuCore: true", "", "hami-core"},
		{"node filter inherits core", "hamiVnpuMode: hami-core", "filterDevices: {index: [7]}", "hami-core"},
		{"node count inherits ENPU", "hamiVnpuMode: enpu", "vDeviceCount: 20", "enpu"},
		{"node ENPU overrides core", "hamiVnpuMode: hami-core", "hamiVnpuMode: enpu", "enpu"},
		{"node core overrides ENPU", "hamiVnpuMode: enpu", "hamiVnpuMode: hamiCore", "hami-core"},
		{"node template overrides ENPU", "hamiVnpuMode: enpu", "hamiVnpuMode: template", "template"},
		{"legacy node true", "hamiVnpuMode: enpu", "hami-vnpu-core: true", "hami-core"},
		{"legacy node false", "hamiVnpuMode: hami-core", "hami-vnpu-core: false", "template"},
		{"node mode overrides legacy", "hamiVnpuMode: hami-core", "hamiVnpuMode: enpu\n    hami-vnpu-core: true", "enpu"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := "vnpus:\n  " + tc.global + "\n  configs:\n  - chipName: 910B3\n    commonWord: Ascend910B3\n"
			am := &AscendManager{mgr: &chipInfoDeviceManager{chipName: "910B3"}}
			if err := am.LoadConfig(writeConfig(t, config)); err != nil {
				t.Fatal(err)
			}
			if tc.node != "" {
				if err := am.LoadNodeConfig(writeConfig(t, "nodes:\n  - name: test-node\n    "+tc.node+"\n"), "test-node"); err != nil {
					t.Fatal(err)
				}
			}
			if got := am.IsHamiVnpuCore(); got != (tc.want == "hami-core") {
				t.Fatalf("IsHamiVnpuCore() = %v, want mode %s", got, tc.want)
			}
			if got := am.IsEnpu(); got != (tc.want == "enpu") {
				t.Fatalf("IsEnpu() = %v, want mode %s", got, tc.want)
			}
		})
	}
}

func TestHamiCorePolicyOverridesGlobal(t *testing.T) {
	for _, tc := range []struct{ name, global, node, want string }{
		{"default", "", "", "spatial"},
		{"global spatial", "hamiCorePolicy: spatial", "", "spatial"},
		{"global dynamic", "hamiCorePolicy: dynamic", "", "dynamic"},
		{"global fixed", "hamiCorePolicy: fixed", "", "fixed"},
		{"global overdraft", "hamiCorePolicy: overdraft", "", "overdraft"},
		{"node inherits", "hamiCorePolicy: fixed", "vDeviceCount: 20", "fixed"},
		{"empty node inherits", "hamiCorePolicy: overdraft", "hamiCorePolicy: ''", "overdraft"},
		{"blank node inherits", "hamiCorePolicy: dynamic", "hamiCorePolicy: '  '", "dynamic"},
		{"null node inherits", "hamiCorePolicy: fixed", "hamiCorePolicy: null", "fixed"},
		{"node spatial override", "hamiCorePolicy: fixed", "hamiCorePolicy: spatial", "spatial"},
		{"node time policy override", "hamiCorePolicy: spatial", "hamiCorePolicy: ' OVERDRAFT '", "overdraft"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := "vnpus:\n  hamiVnpuMode: hami-core\n  " + tc.global + "\n  configs:\n  - chipName: 910B3\n    commonWord: Ascend910B3\n"
			am := &AscendManager{mgr: &chipInfoDeviceManager{chipName: "910B3"}}
			if err := am.LoadConfig(writeConfig(t, config)); err != nil {
				t.Fatal(err)
			}
			if tc.node != "" {
				if err := am.LoadNodeConfig(writeConfig(t, "nodes:\n- name: test-node\n  "+tc.node+"\n"), "test-node"); err != nil {
					t.Fatal(err)
				}
			}
			got, err := internal.ResolveHamiCorePolicy(am.HamiCorePolicy())
			if err != nil || got != tc.want {
				t.Fatalf("resolved HamiCorePolicy() = %q, %v; want %q", got, err, tc.want)
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
			wantCount := 4
			if tt.wantHamiVnpuCore {
				wantCount = 10
			}
			if got := am.VDeviceCount(); got != wantCount {
				t.Errorf("VDeviceCount() = %d, want %d", got, wantCount)
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

func TestListVirtualDevicesCleanupFilter(t *testing.T) {
	uuidErr := errors.New("UUID query is unavailable")
	for _, tc := range []struct {
		name          string
		nodeConfig    *internal.NodeConfig
		uuid          string
		uuidErr       error
		wantIgnored   bool
		wantUUIDCalls int
	}{
		{name: "no node filter", uuidErr: uuidErr},
		{name: "empty filter", nodeConfig: &internal.NodeConfig{}, uuidErr: uuidErr},
		{
			name:       "unmatched index does not depend on UUID query",
			nodeConfig: &internal.NodeConfig{FilterDevices: internal.FilterDevices{Index: []int32{8}}},
			uuidErr:    uuidErr,
		},
		{
			name:        "matched index does not depend on UUID query",
			nodeConfig:  &internal.NodeConfig{FilterDevices: internal.FilterDevices{Index: []int32{7}}},
			uuidErr:     uuidErr,
			wantIgnored: true,
		},
		{
			name:          "unmatched UUID is queried once for all vNPUs",
			nodeConfig:    &internal.NodeConfig{FilterDevices: internal.FilterDevices{UUID: []string{"other-uuid"}}},
			uuid:          "card-uuid",
			wantUUIDCalls: 1,
		},
		{
			name:          "matched UUID protects all vNPUs",
			nodeConfig:    &internal.NodeConfig{FilterDevices: internal.FilterDevices{UUID: []string{"card-uuid"}}},
			uuid:          "card-uuid",
			wantIgnored:   true,
			wantUUIDCalls: 1,
		},
		{
			name:          "required UUID query failure protects all vNPUs",
			nodeConfig:    &internal.NodeConfig{FilterDevices: internal.FilterDevices{UUID: []string{"card-uuid"}}},
			uuidErr:       uuidErr,
			wantIgnored:   true,
			wantUUIDCalls: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			driver := &cleanupFilterDeviceManager{uuid: tc.uuid, uuidErr: tc.uuidErr}
			am := &AscendManager{mgr: driver, nodeConfig: tc.nodeConfig}
			devices, err := am.ListVirtualDevices(3)
			if err != nil {
				t.Fatal(err)
			}
			if len(devices) != 2 {
				t.Fatalf("ListVirtualDevices() returned %d vNPUs, want 2", len(devices))
			}
			for _, device := range devices {
				if device.Ignored != tc.wantIgnored {
					t.Errorf("vNPU %d Ignored = %v, want %v", device.VDevID, device.Ignored, tc.wantIgnored)
				}
			}
			if driver.uuidCalls != tc.wantUUIDCalls {
				t.Errorf("GetDieID() called %d times, want %d", driver.uuidCalls, tc.wantUUIDCalls)
			}
		})
	}
}

type cleanupFilterDeviceManager struct {
	*devmanager.DeviceManagerMock
	uuid      string
	uuidErr   error
	uuidCalls int
}

func (d *cleanupFilterDeviceManager) GetCardIDDeviceID(int32) (int32, int32, error) {
	return 7, 0, nil
}

func (d *cleanupFilterDeviceManager) GetVirtualDeviceInfo(int32) (common.VirtualDevInfo, error) {
	return common.VirtualDevInfo{VDevInfo: []common.CgoVDevQueryStru{{VDevID: 100}, {VDevID: 101}}}, nil
}

func (d *cleanupFilterDeviceManager) GetDieID(int32, dcmi.DieType) (string, error) {
	d.uuidCalls++
	return d.uuid, d.uuidErr
}
