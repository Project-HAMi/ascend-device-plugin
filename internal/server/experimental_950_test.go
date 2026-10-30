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
	"reflect"
	"testing"

	"github.com/Project-HAMi/HAMi/pkg/device"
	"github.com/Project-HAMi/ascend-device-plugin/internal/manager"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test950AllocateMappingAndQuotas(t *testing.T) {
	oldPath := hostHookPath
	hostHookPath = t.TempDir()
	t.Cleanup(func() { hostHookPath = oldPath })
	physical := &manager.Device{UUID: "die-950", LogicID: 6, PhyID: 6, CardID: -1, DeviceID: -1, Memory: 98304}
	mgr := &FakeManager{
		CommonWordFunc:      func() string { return "Ascend950DT" },
		IsHamiVnpuCoreFunc:  func() bool { return true },
		GetDeviceByUUIDFunc: func(string) *manager.Device { return physical },
	}
	ps := &PluginServer{mgr: mgr}
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{UID: "test", Annotations: map[string]string{VNPUModeAnnotation: VNPUModeHamiCore}}}
	memory := int64(8192)
	core := int32(50)
	info := map[string]RuntimeInfo{"die-950": {Memory: &memory, Core: &core}}
	devices := device.ContainerDevices{{UUID: "die-950"}}
	resp, err := ps.buildContainerAllocateResponse(pod, "app", devices, info)
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{
		"ASCEND_VISIBLE_DEVICES": "6",
		"NPU_DCMI_DEVICE":        "v2:6",
		"NPU_MEM_QUOTA":          "8192",
		"NPU_PRIORITY":           "50",
		"NPU_FIXED_SHARE_RATIO":  "1",
		"NPU_FAIL_CLOSED":        "1",
		"NPU_GLOBAL_SHM_PATH":    "/hami-shared-region/6_global_registry",
	} {
		if resp.Envs[key] != want {
			t.Errorf("%s=%q, want %q", key, resp.Envs[key], want)
		}
	}
	if _, ok := resp.Envs["ASCEND_VNPU_SPECS"]; ok {
		t.Fatal("soft slicing must not inject a hardware template")
	}
	if _, err = ps.buildContainerAllocateResponse(pod, "app", append(devices, devices...), info); err == nil {
		t.Fatal("accepted multiple devices")
	}
	core = 0
	if _, err = ps.buildContainerAllocateResponse(pod, "app", devices, info); err == nil {
		t.Fatal("accepted missing quota")
	}
	core = 50
	pod.Annotations[VNPUModeAnnotation] = "template"
	if _, err = ps.buildContainerAllocateResponse(pod, "app", devices, info); err == nil {
		t.Fatal("accepted template mode")
	}
}

func Test950NpuIDDeviceShareCommand(t *testing.T) {
	var got []string
	withFakeNpuSmi(t, func(args ...string) ([]byte, error) {
		got = append([]string{}, args...)
		return nil, nil
	})
	if err := applyDeviceShare([]chipKey{{Card: 12, NPUOnly: true}}, true); err != nil {
		t.Fatal(err)
	}
	want := []string{"set", "-t", "device-share", "-i", "12", "-d", "1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
