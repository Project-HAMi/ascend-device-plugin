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

package internal

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/yaml"
)

// currentFormat is what HAMi >= v2.9.0 writes into the hami-scheduler-device
// ConfigMap: `vnpus` is a mapping holding hamiVnpuCore plus a configs list.
const currentFormat = `
vnpus:
  hamiVnpuCore: true
  configs:
  - chipName: 910B4
    commonWord: Ascend910B4
    resourceName: huawei.com/Ascend910B4
    resourceMemoryName: huawei.com/Ascend910B4-memory
    memoryAllocatable: 32768
    memoryCapacity: 32768
    aiCore: 20
    aiCPU: 7
    templates:
      - name: vir05_1c_8g
        memory: 8192
        aiCore: 5
        aiCPU: 1
      - name: vir10_3c_16g
        memory: 16384
        aiCore: 10
        aiCPU: 3
`

// legacyFormat carries the very same chip as currentFormat, in the layout HAMi
// v2.7.0 - v2.8.x writes: `vnpus` is a plain list and there is no hamiVnpuCore.
const legacyFormat = `
vnpus:
- chipName: 910B4
  commonWord: Ascend910B4
  resourceName: huawei.com/Ascend910B4
  resourceMemoryName: huawei.com/Ascend910B4-memory
  memoryAllocatable: 32768
  memoryCapacity: 32768
  aiCore: 20
  aiCPU: 7
  templates:
    - name: vir05_1c_8g
      memory: 8192
      aiCore: 5
      aiCPU: 1
    - name: vir10_3c_16g
      memory: 16384
      aiCore: 10
      aiCPU: 3
`

// legacyFormatWithHamiVnpuCore is the pre-v2.9.0 layout this plugin used to
// ship in ascend-device-configmap.yaml: the switch sat next to the list.
const legacyFormatWithHamiVnpuCore = "hamiVnpuCore: true\n" + legacyFormat

// otherVendorSections mirrors a real hami-scheduler-device ConfigMap, which
// carries every vendor's config in the same document.
const otherVendorSections = `
nvidia:
  resourceCountName: nvidia.com/gpu
  resourceMemoryName: nvidia.com/gpumem
cambricon:
  resourceCountName: cambricon.com/vmlu
`

// wantVNPUs is what every fixture above must decode to. Comparing whole values
// against it is what keeps the two layouts honest: a field the fallback forgets
// to carry over, or a fixture that drifts, fails the test.
func wantVNPUs(hamiVnpuCore bool) VNPUsConfig {
	return VNPUsConfig{
		HamiVnpuCore: hamiVnpuCore,
		Configs: []VNPUConfig{{
			ChipName:           "910B4",
			CommonWord:         "Ascend910B4",
			ResourceName:       "huawei.com/Ascend910B4",
			ResourceMemoryName: "huawei.com/Ascend910B4-memory",
			MemoryAllocatable:  32768,
			MemoryCapacity:     32768,
			AICore:             20,
			AICPU:              7,
			Templates: []Template{
				{Name: "vir05_1c_8g", Memory: 8192, AICore: 5, AICPU: 1},
				{Name: "vir10_3c_16g", Memory: 16384, AICore: 10, AICPU: 3},
			},
		}},
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "device-config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

// TestLoadConfigAcceptsBothLayouts is the compatibility contract: the plugin
// must read the HAMi >= v2.9.0 device-config.yaml and the older one alike, and
// expose both through the same Config.
func TestLoadConfigAcceptsBothLayouts(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    VNPUsConfig
	}{
		{
			name:    "current layout",
			content: currentFormat,
			want:    wantVNPUs(true),
		},
		{
			name:    "current layout alongside other vendors",
			content: otherVendorSections + currentFormat,
			want:    wantVNPUs(true),
		},
		{
			name:    "current layout wins over a leftover top-level hamiVnpuCore",
			content: "hamiVnpuCore: false\n" + currentFormat,
			want:    wantVNPUs(true),
		},
		{
			name:    "legacy layout",
			content: legacyFormat,
			want:    wantVNPUs(false),
		},
		{
			name:    "legacy layout alongside other vendors",
			content: otherVendorSections + legacyFormat,
			want:    wantVNPUs(false),
		},
		{
			name:    "legacy layout with top-level hamiVnpuCore",
			content: legacyFormatWithHamiVnpuCore,
			want:    wantVNPUs(true),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config, err := LoadConfig(writeConfig(t, tc.content))
			if err != nil {
				t.Fatalf("LoadConfig() error = %v, want nil", err)
			}
			if !reflect.DeepEqual(config.VNPUs, tc.want) {
				t.Errorf("VNPUs = %+v, want %+v", config.VNPUs, tc.want)
			}
		})
	}
}

// TestLoadConfigWithoutChips covers the configs that declare no Ascend chip:
// they must stay a successful parse, so the caller reports "no vnpu config for
// chip X" rather than a parse error.
func TestLoadConfigWithoutChips(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{name: "no vnpus section at all", content: otherVendorSections},
		{name: "current layout, empty configs", content: "vnpus:\n  configs: []\n"},
		{name: "legacy layout, empty list", content: "vnpus: []\n"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config, err := LoadConfig(writeConfig(t, tc.content))
			if err != nil {
				t.Fatalf("LoadConfig() error = %v, want nil", err)
			}
			if len(config.VNPUs.Configs) != 0 {
				t.Errorf("len(VNPUs.Configs) = %d, want 0", len(config.VNPUs.Configs))
			}
		})
	}
}

// TestLoadConfigErrors keeps the original behaviour for input that matches
// neither layout, and checks the error still points at what is actually wrong:
// falling back must not turn a plain typo into "neither layout matched".
func TestLoadConfigErrors(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		contains string
	}{
		{
			name: "file does not exist",
			path: filepath.Join(t.TempDir(), "missing.yaml"),
		},
		{
			name: "not valid yaml",
			path: writeConfig(t, "vnpus:\n\t- chipName: 910B4\n  bad indent\n"),
		},
		{
			name:     "vnpus is neither a list nor a mapping",
			path:     writeConfig(t, "vnpus: 910B4\n"),
			contains: "Config.vnpus",
		},
		{
			name:     "wrong type inside the current layout",
			path:     writeConfig(t, "vnpus:\n  configs:\n  - memoryAllocatable: not-a-number\n"),
			contains: "vnpus.configs.memoryAllocatable",
		},
		{
			name:     "wrong type inside the legacy layout",
			path:     writeConfig(t, "vnpus:\n- chipName: [910B4]\n"),
			contains: "vnpus.chipName",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadConfig(tc.path)
			if err == nil {
				t.Fatal("LoadConfig() error = nil, want an error")
			}
			if tc.contains != "" && !strings.Contains(err.Error(), tc.contains) {
				t.Errorf("LoadConfig() error = %v, want it to mention %q", err, tc.contains)
			}
		})
	}
}

// TestIsLegacyVNPUsLayout pins down the discriminator that decides whether to
// retry with the older layout. It is deliberately fed real YAML rather than
// hand-built errors: if a future yaml library stops wrapping
// *json.UnmarshalTypeError, or encoding/json stops reporting nested fields with
// a dotted path, this fails instead of the fallback silently going dead.
func TestIsLegacyVNPUsLayout(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{name: "legacy layout", content: legacyFormat, want: true},
		{name: "legacy layout, empty list", content: "vnpus: []\n", want: true},
		{name: "current layout, nothing to classify", content: currentFormat, want: false},
		{name: "vnpus is a scalar", content: "vnpus: 910B4\n", want: false},
		{name: "wrong type under vnpus.configs", content: "vnpus:\n  configs:\n  - memoryAllocatable: x\n", want: false},
		{name: "wrong type under vnpus.configs.templates", content: "vnpus:\n  configs:\n  - templates:\n    - memory: x\n", want: false},
		{name: "yaml syntax error", content: "vnpus:\n\t- chipName: 910B4\n", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var config Config
			err := yaml.Unmarshal([]byte(tc.content), &config)
			if got := isLegacyVNPUsLayout(err); got != tc.want {
				t.Errorf("isLegacyVNPUsLayout(%v) = %v, want %v", err, got, tc.want)
			}
		})
	}
}

func TestAdvertisedDevcore(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		hami     bool
		scale    float64
		hardware int32
		want     int32
	}{
		{name: "template keeps hardware", hami: false, scale: 1.5, hardware: 20, want: 20},
		{name: "hami-core default", hami: true, scale: 0, hardware: 20, want: 100},
		{name: "hami-core 1.0", hami: true, scale: 1, hardware: 20, want: 100},
		{name: "hami-core 1.5", hami: true, scale: 1.5, hardware: 20, want: 150},
		{name: "hami-core 2.0", hami: true, scale: 2, hardware: 20, want: 200},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := AdvertisedDevcore(tc.hami, tc.scale, tc.hardware)
			if got != tc.want {
				t.Fatalf("AdvertisedDevcore(%v, %v, %d)=%d, want %d", tc.hami, tc.scale, tc.hardware, got, tc.want)
			}
		})
	}
}
