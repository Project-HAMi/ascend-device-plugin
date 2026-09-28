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

package server

import (
	"errors"
	"os"
	"strings"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/Project-HAMi/HAMi/pkg/device"
	"github.com/Project-HAMi/ascend-device-plugin/internal/manager"
)

func TestENPUSingleDieMode(t *testing.T) {
	for _, tc := range []struct {
		name      string
		output    string
		want      bool
		wantError bool
	}{
		{"independent", "\tMulti_die policy               : INDEP_POLICY\n", true, false},
		{"union", "\tMulti_die policy               : UNION_POLICY\n", false, false},
		{"whitespace", "  Multi_die policy :  INDEP_POLICY\r\n", true, false},
		{"missing", "", false, true},
		{"help is not status", "(0: UNION_POLICY, 1: INDEP_POLICY)\n", false, true},
		{"unknown", "Multi_die policy : UNKNOWN_POLICY\n", false, true},
		{"unexpected suffix", "Multi_die policy : INDEP_POLICY invalid\n", false, true},
		{"duplicate", "Multi_die policy : INDEP_POLICY\nMulti_die policy : UNION_POLICY\n", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := enpuSingleDieMode([]byte(tc.output))
			if (err != nil) != tc.wantError || got != tc.want {
				t.Fatalf("enpuSingleDieMode(%q) = %v, %v; want %v, error=%v", tc.output, got, err, tc.want, tc.wantError)
			}
		})
	}
}

func TestAllocateENPUSingleDiePrerequisite(t *testing.T) {
	for _, tc := range []struct {
		name         string
		mode         string
		commonWord   string
		nodeHamiCore bool
		output       string
		queryError   error
		wantQueries  int
		wantError    string
	}{
		{"A3 independent", "enpu", "Ascend910C", false, "Multi_die policy : INDEP_POLICY\n", nil, 1, ""},
		{"A3 union", "enpu", "Ascend910C", false, "Multi_die policy : UNION_POLICY\n", nil, 1, "npu-smi set -t multi-die-policy -d 1"},
		{"A3 query failure", "enpu", "Ascend910C", false, "driver error", errors.New("query unavailable"), 1, "query unavailable"},
		{"A3 unrecognized output", "enpu", "Ascend910C", false, "INDEP_POLICY\n", nil, 1, "field missing"},
		{"other ENPU hardware", "enpu", "Ascend910B", false, "", nil, 0, ""},
		{"explicit hami-core", VNPUModeHamiCore, "Ascend910C", false, "", nil, 0, ""},
		{"default hami-core", "", "Ascend910C", true, "", nil, 0, ""},
		{"template", "template", "Ascend910C", false, "", nil, 0, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			configRoot := t.TempDir()
			t.Setenv("ENPU_CONFIG_ROOT", configRoot)
			t.Setenv("ENPU_MANAGER_URL", "")
			t.Setenv("ENPU_EXPOSE_ALL_DEVICES", "false")
			t.Setenv("ENPU_SCHEDULING_POLICY", "elastic")
			oldHookPath := hostHookPath
			hostHookPath = t.TempDir()
			t.Cleanup(func() { hostHookPath = oldHookPath })
			queries := 0
			withFakeNpuSmi(t, func(args ...string) ([]byte, error) {
				queries++
				if strings.Join(args, " ") != "info -t multi-die-policy" {
					t.Fatalf("unexpected npu-smi command: %v", args)
				}
				return []byte(tc.output), tc.queryError
			})
			dev := &manager.Device{UUID: "die-15", PhyID: 15, CardID: 7, DeviceID: 1, Memory: 65536}
			ps := &PluginServer{mgr: &FakeManager{
				CommonWordFunc:      func() string { return tc.commonWord },
				GetDeviceByUUIDFunc: func(string) *manager.Device { return dev },
				IsHamiVnpuCoreFunc:  func() bool { return tc.nodeHamiCore },
			}}
			pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{
				Name: "single-die-check", Namespace: "test", UID: types.UID("test-uid"),
				Annotations: map[string]string{VNPUModeAnnotation: tc.mode},
			}}
			memory := int64(16384)
			core := int32(20)
			resp, err := ps.buildContainerAllocateResponse(pod, "test",
				device.ContainerDevices{{UUID: dev.UUID, Type: tc.commonWord, Usedmem: int32(memory), Usedcores: core}},
				map[string]RuntimeInfo{dev.UUID: {UUID: dev.UUID, Memory: &memory, Core: &core}})
			if queries != tc.wantQueries {
				t.Fatalf("npu-smi queries=%d; want %d", queries, tc.wantQueries)
			}
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("error=%v; want containing %q", err, tc.wantError)
				}
				entries, readErr := os.ReadDir(configRoot)
				if readErr != nil || len(entries) != 0 {
					t.Fatalf("failed prerequisite must not create ENPU config: %v, %v", entries, readErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := resp.Envs["ASCEND_VISIBLE_DEVICES"]; got != "15" {
				t.Fatalf("physical DIE mapping changed: %q", got)
			}
		})
	}
}
