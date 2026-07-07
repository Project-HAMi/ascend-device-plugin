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
	"testing"

	"github.com/Project-HAMi/ascend-device-plugin/internal"
)

// TestVDeviceCount verifies that VDeviceCount honors a per-node override
// (nodeConfig.VDeviceCount) when present, and otherwise falls back to the
// global default derived from MemoryAllocatable / smallest template.
//
// This guards the fix for the bug where VDeviceCount ignored the loaded
// nodeConfig.VDeviceCount and always used the global default, even though the
// sibling IsHamiVnpuCore() already preferred nodeConfig.
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
