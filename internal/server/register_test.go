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
	"context"
	"strings"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Project-HAMi/HAMi/pkg/device"
	"github.com/Project-HAMi/HAMi/pkg/util/client"
	"github.com/Project-HAMi/ascend-device-plugin/internal/manager"
)

func TestGetDeviceNetworkID(t *testing.T) {
	t.Parallel()

	type getDeviceNetworkIDArgs struct {
		idx        int
		deviceType string
	}

	tests := []struct {
		name    string
		args    getDeviceNetworkIDArgs
		want    int
		wantErr bool
	}{
		{name: "Ascend910A_idx0_non910C", args: getDeviceNetworkIDArgs{idx: 0, deviceType: "Ascend910A"}, want: 0},
		{name: "Ascend910A_idx5_non910C", args: getDeviceNetworkIDArgs{idx: 5, deviceType: "Ascend910A"}, want: 1},
		{name: "Ascend910B_idx0", args: getDeviceNetworkIDArgs{idx: 0, deviceType: "Ascend910B"}, want: 0},
		{name: "Ascend910B_idx3_boundary", args: getDeviceNetworkIDArgs{idx: 3, deviceType: "Ascend910B"}, want: 0},
		{name: "Ascend910B_idx4", args: getDeviceNetworkIDArgs{idx: 4, deviceType: "Ascend910B"}, want: 1},
		{name: "Ascend910B_idx100_large", args: getDeviceNetworkIDArgs{idx: 100, deviceType: "Ascend910B"}, want: 1},
		{name: "Ascend910C_idx0", args: getDeviceNetworkIDArgs{idx: 0, deviceType: "Ascend910C"}, want: 0},
		{name: "Ascend910C_idx4_still0", args: getDeviceNetworkIDArgs{idx: 4, deviceType: "Ascend910C"}, want: 0},
		{name: "Ascend910C_idx100_still0", args: getDeviceNetworkIDArgs{idx: 100, deviceType: "Ascend910C"}, want: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ps := &PluginServer{}
			got, err := ps.getDeviceNetworkID(tc.args.idx, tc.args.deviceType)
			if (err != nil) != tc.wantErr {
				t.Fatalf("getDeviceNetworkID(%d, %q) error = %v, wantErr %v", tc.args.idx, tc.args.deviceType, err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("getDeviceNetworkID(%d, %q) = %d, want %d", tc.args.idx, tc.args.deviceType, got, tc.want)
			}
		})
	}
}

func TestRegisterHAMi(t *testing.T) {
	t.Parallel()

	type registerHAMiArgs struct {
		nodeName      string
		registerAnno  string
		handshakeAnno string
		mgr           *FakeManager
		nodes         []*v1.Node
	}

	type registerHAMiWant struct {
		deviceCount     int
		deviceCheck     func(t *testing.T, devs []*device.DeviceInfo)
		annotationCheck func(t *testing.T, annos map[string]string)
	}

	tests := []struct {
		name    string
		args    registerHAMiArgs
		want    registerHAMiWant
		wantErr string
	}{
		{
			name: "NodeNotFound",
			args: registerHAMiArgs{
				nodeName:      "missing-node",
				registerAnno:  "hami.io/node-register-Ascend910",
				handshakeAnno: "hami.io/node-handshake-Ascend910",
				mgr: &FakeManager{
					GetDevicesFunc:   func() []*manager.Device { return nil },
					VDeviceCountFunc: func() int { return 1 },
					CommonWordFunc:   func() string { return "Ascend910" },
				},
				nodes: nil,
			},
			wantErr: "get node",
		},
		{
			name: "SingleDevice",
			args: registerHAMiArgs{
				nodeName:      "test-node",
				registerAnno:  "hami.io/node-register-Ascend910",
				handshakeAnno: "hami.io/node-handshake-Ascend910",
				mgr: &FakeManager{
					GetDevicesFunc: func() []*manager.Device {
						return []*manager.Device{
							{UUID: "uuid1", Memory: 32768, AICore: 30, Health: true},
						}
					},
					VDeviceCountFunc: func() int { return 4 },
					CommonWordFunc:   func() string { return "Ascend910" },
				},
				nodes: []*v1.Node{
					{ObjectMeta: metav1.ObjectMeta{Name: "test-node", Annotations: map[string]string{}}},
				},
			},
			want: registerHAMiWant{
				deviceCount: 1,
				deviceCheck: func(t *testing.T, devs []*device.DeviceInfo) {
					t.Helper()
					d := devs[0]
					if d.ID != "uuid1" {
						t.Fatalf("device ID = %q, want uuid1", d.ID)
					}
					if d.Index != 0 {
						t.Fatalf("device Index = %d, want 0", d.Index)
					}
					if d.Count != 4 {
						t.Fatalf("device Count = %d, want 4", d.Count)
					}
					if d.Devmem != 32768 {
						t.Fatalf("device Devmem = %d, want 32768", d.Devmem)
					}
					if d.Devcore != 30 {
						t.Fatalf("device Devcore = %d, want 30", d.Devcore)
					}
					if d.Type != "Ascend910" {
						t.Fatalf("device Type = %q, want Ascend910", d.Type)
					}
					if !d.Health {
						t.Fatal("device Health = false, want true")
					}
				},
				annotationCheck: func(t *testing.T, annos map[string]string) {
					t.Helper()
					hs := annos["hami.io/node-handshake-Ascend910"]
					if !strings.HasPrefix(hs, "Reported_") {
						t.Fatalf("handshakeAnno = %q, want prefix 'Reported_'", hs)
					}
				},
			},
		},
		{
			name: "MultiDevice",
			args: registerHAMiArgs{
				nodeName:      "test-node",
				registerAnno:  "hami.io/node-register-Ascend910C",
				handshakeAnno: "hami.io/node-handshake-Ascend910C",
				mgr: &FakeManager{
					GetDevicesFunc: func() []*manager.Device {
						return []*manager.Device{
							{UUID: "uuid1", Memory: 65536, AICore: 60, Health: true},
							{UUID: "uuid2", Memory: 65536, AICore: 60, Health: false},
						}
					},
					VDeviceCountFunc: func() int { return 2 },
					CommonWordFunc:   func() string { return "Ascend910C" },
				},
				nodes: []*v1.Node{
					{ObjectMeta: metav1.ObjectMeta{Name: "test-node", Annotations: map[string]string{}}},
				},
			},
			want: registerHAMiWant{
				deviceCount: 2,
				deviceCheck: func(t *testing.T, devs []*device.DeviceInfo) {
					t.Helper()
					if devs[0].Index != 0 || devs[1].Index != 1 {
						t.Fatalf("device indices = %d, %d, want 0, 1", devs[0].Index, devs[1].Index)
					}
					if !devs[0].Health {
						t.Fatal("device[0] Health = false, want true")
					}
					if devs[1].Health {
						t.Fatal("device[1] Health = true, want false")
					}
					if devs[0].Type != "Ascend910C" {
						t.Fatalf("device[0] Type = %q, want Ascend910C", devs[0].Type)
					}
				},
			},
		},
		{
			name: "EmptyDevices",
			args: registerHAMiArgs{
				nodeName:      "test-node",
				registerAnno:  "hami.io/node-register-Ascend910",
				handshakeAnno: "hami.io/node-handshake-Ascend910",
				mgr: &FakeManager{
					GetDevicesFunc:   func() []*manager.Device { return nil },
					VDeviceCountFunc: func() int { return 1 },
					CommonWordFunc:   func() string { return "Ascend910" },
				},
				nodes: []*v1.Node{
					{ObjectMeta: metav1.ObjectMeta{Name: "test-node", Annotations: map[string]string{}}},
				},
			},
			want: registerHAMiWant{
				deviceCount: 0,
			},
		},
		{
			name: "NetworkID_Ascend910B_LowIdx",
			args: registerHAMiArgs{
				nodeName:      "test-node",
				registerAnno:  "hami.io/node-register-Ascend910B",
				handshakeAnno: "hami.io/node-handshake-Ascend910B",
				mgr: &FakeManager{
					GetDevicesFunc: func() []*manager.Device {
						return []*manager.Device{{UUID: "uuid1", Memory: 32768, AICore: 30, Health: true}}
					},
					VDeviceCountFunc:   func() int { return 1 },
					CommonWordFunc:     func() string { return "Ascend910B" },
					IsHamiVnpuCoreFunc: func() bool { return false },
				},
				nodes: []*v1.Node{
					{ObjectMeta: metav1.ObjectMeta{Name: "test-node", Annotations: map[string]string{}}},
				},
			},
			want: registerHAMiWant{
				deviceCount: 1,
				deviceCheck: func(t *testing.T, devs []*device.DeviceInfo) {
					t.Helper()
					ci, ok := devs[0].CustomInfo["NetworkID"]
					if !ok {
						t.Fatal("expected CustomInfo to contain NetworkID for Ascend910B device")
					}
					if int(ci.(float64)) != 0 {
						t.Fatalf("NetworkID = %d, want 0 for idx=0", int(ci.(float64)))
					}
				},
			},
		},
		{
			name: "NetworkID_Ascend910B_HighIdx",
			args: registerHAMiArgs{
				nodeName:      "test-node",
				registerAnno:  "hami.io/node-register-Ascend910B",
				handshakeAnno: "hami.io/node-handshake-Ascend910B",
				mgr: &FakeManager{
					GetDevicesFunc: func() []*manager.Device {
						return []*manager.Device{
							{UUID: "uuid0", Memory: 32768, AICore: 30, Health: true},
							{UUID: "uuid1", Memory: 32768, AICore: 30, Health: true},
							{UUID: "uuid2", Memory: 32768, AICore: 30, Health: true},
							{UUID: "uuid3", Memory: 32768, AICore: 30, Health: true},
							{UUID: "uuid4", Memory: 32768, AICore: 30, Health: true},
						}
					},
					VDeviceCountFunc:   func() int { return 1 },
					CommonWordFunc:     func() string { return "Ascend910B" },
					IsHamiVnpuCoreFunc: func() bool { return false },
				},
				nodes: []*v1.Node{
					{ObjectMeta: metav1.ObjectMeta{Name: "test-node", Annotations: map[string]string{}}},
				},
			},
			want: registerHAMiWant{
				deviceCount: 5,
				deviceCheck: func(t *testing.T, devs []*device.DeviceInfo) {
					t.Helper()
					for i, d := range devs {
						ci, ok := d.CustomInfo["NetworkID"]
						if !ok {
							t.Fatalf("device[%d] missing NetworkID", i)
						}
						netID := int(ci.(float64))
						if i < 4 && netID != 0 {
							t.Fatalf("device[%d] NetworkID = %d, want 0", i, netID)
						}
						if i == 4 && netID != 1 {
							t.Fatalf("device[4] NetworkID = %d, want 1", netID)
						}
					}
				},
			},
		},
		{
			name: "NetworkID_Ascend910C_AlwaysZero",
			args: registerHAMiArgs{
				nodeName:      "test-node",
				registerAnno:  "hami.io/node-register-Ascend910C",
				handshakeAnno: "hami.io/node-handshake-Ascend910C",
				mgr: &FakeManager{
					GetDevicesFunc: func() []*manager.Device {
						return []*manager.Device{
							{UUID: "uuid0", Memory: 65536, AICore: 60, Health: true},
							{UUID: "uuid5", Memory: 65536, AICore: 60, Health: true},
						}
					},
					VDeviceCountFunc:   func() int { return 1 },
					CommonWordFunc:     func() string { return "Ascend910C" },
					IsHamiVnpuCoreFunc: func() bool { return false },
				},
				nodes: []*v1.Node{
					{ObjectMeta: metav1.ObjectMeta{Name: "test-node", Annotations: map[string]string{}}},
				},
			},
			want: registerHAMiWant{
				deviceCount: 2,
				deviceCheck: func(t *testing.T, devs []*device.DeviceInfo) {
					t.Helper()
					for i, d := range devs {
						ci, ok := d.CustomInfo["NetworkID"]
						if !ok {
							t.Fatalf("device[%d] missing NetworkID", i)
						}
						if int(ci.(float64)) != 0 {
							t.Fatalf("device[%d] NetworkID = %d, want 0 (Ascend910C always 0)", i, int(ci.(float64)))
						}
					}
				},
			},
		},
		{
			name: "NonAscend910_NoCustomInfo",
			args: registerHAMiArgs{
				nodeName:      "test-node",
				registerAnno:  "hami.io/node-register-Ascend310P",
				handshakeAnno: "hami.io/node-handshake-Ascend310P",
				mgr: &FakeManager{
					GetDevicesFunc: func() []*manager.Device {
						return []*manager.Device{{UUID: "uuid1", Memory: 16384, AICore: 15, Health: true}}
					},
					VDeviceCountFunc:   func() int { return 1 },
					CommonWordFunc:     func() string { return "Ascend310P" },
					IsHamiVnpuCoreFunc: func() bool { return false },
				},
				nodes: []*v1.Node{
					{ObjectMeta: metav1.ObjectMeta{Name: "test-node", Annotations: map[string]string{}}},
				},
			},
			want: registerHAMiWant{
				deviceCount: 1,
				deviceCheck: func(t *testing.T, devs []*device.DeviceInfo) {
					t.Helper()
					if len(devs[0].CustomInfo) != 0 {
						t.Fatalf("expected no CustomInfo for non-Ascend910 device, got %v", devs[0].CustomInfo)
					}
				},
			},
		},
		{
			name: "IsHamiVnpuCore_True",
			args: registerHAMiArgs{
				nodeName:      "test-node",
				registerAnno:  "hami.io/node-register-Ascend910",
				handshakeAnno: "hami.io/node-handshake-Ascend910",
				mgr: &FakeManager{
					GetDevicesFunc:     func() []*manager.Device { return nil },
					VDeviceCountFunc:   func() int { return 1 },
					CommonWordFunc:     func() string { return "Ascend910" },
					IsHamiVnpuCoreFunc: func() bool { return true },
				},
				nodes: []*v1.Node{
					{ObjectMeta: metav1.ObjectMeta{Name: "test-node", Annotations: map[string]string{}}},
				},
			},
			want: registerHAMiWant{
				annotationCheck: func(t *testing.T, annos map[string]string) {
					t.Helper()
					if annos[VNPUNodeSelectorAnnotation] != "true" {
						t.Fatalf("VNPUNodeSelectorAnnotation = %q, want 'true'", annos[VNPUNodeSelectorAnnotation])
					}
				},
			},
		},
		{
			name: "IsHamiVnpuCore_False",
			args: registerHAMiArgs{
				nodeName:      "test-node",
				registerAnno:  "hami.io/node-register-Ascend910",
				handshakeAnno: "hami.io/node-handshake-Ascend910",
				mgr: &FakeManager{
					GetDevicesFunc:     func() []*manager.Device { return nil },
					VDeviceCountFunc:   func() int { return 1 },
					CommonWordFunc:     func() string { return "Ascend910" },
					IsHamiVnpuCoreFunc: func() bool { return false },
				},
				nodes: []*v1.Node{
					{ObjectMeta: metav1.ObjectMeta{Name: "test-node", Annotations: map[string]string{}}},
				},
			},
			want: registerHAMiWant{
				annotationCheck: func(t *testing.T, annos map[string]string) {
					t.Helper()
					if annos[VNPUNodeSelectorAnnotation] != "false" {
						t.Fatalf("VNPUNodeSelectorAnnotation = %q, want 'false'", annos[VNPUNodeSelectorAnnotation])
					}
				},
			},
		},
		{
			name: "HandshakeAnnotationFormat",
			args: registerHAMiArgs{
				nodeName:      "test-node",
				registerAnno:  "hami.io/node-register-Ascend910",
				handshakeAnno: "hami.io/node-handshake-Ascend910",
				mgr: &FakeManager{
					GetDevicesFunc:     func() []*manager.Device { return nil },
					VDeviceCountFunc:   func() int { return 1 },
					CommonWordFunc:     func() string { return "Ascend910" },
					IsHamiVnpuCoreFunc: func() bool { return false },
				},
				nodes: []*v1.Node{
					{ObjectMeta: metav1.ObjectMeta{Name: "test-node", Annotations: map[string]string{}}},
				},
			},
			want: registerHAMiWant{
				annotationCheck: func(t *testing.T, annos map[string]string) {
					t.Helper()
					hs := annos["hami.io/node-handshake-Ascend910"]
					if !strings.HasPrefix(hs, "Reported_") {
						t.Fatalf("handshake annotation = %q, want prefix 'Reported_'", hs)
					}
					timeStr := strings.TrimPrefix(hs, "Reported_")
					if _, err := time.Parse("2006.01.02 15:04:05", timeStr); err != nil {
						t.Fatalf("handshake time %q does not match expected format: %v", timeStr, err)
					}
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ps := &PluginServer{
				nodeName:      tc.args.nodeName,
				registerAnno:  tc.args.registerAnno,
				handshakeAnno: tc.args.handshakeAnno,
				mgr:           tc.args.mgr,
			}
			cleanup := setupFakeClient(nil, tc.args.nodes)
			defer cleanup()

			err := ps.registerHAMi()

			if tc.wantErr != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error should contain %q, got: %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			updated, err := client.KubeClient.CoreV1().Nodes().Get(context.Background(), tc.args.nodeName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("failed to get updated node: %v", err)
			}

			devs, err := device.UnMarshalNodeDevices(updated.Annotations[tc.args.registerAnno])
			if err != nil {
				t.Fatalf("failed to unmarshal node devices: %v", err)
			}

			if tc.want.deviceCount != 0 && len(devs) != tc.want.deviceCount {
				t.Fatalf("expected %d devices, got %d", tc.want.deviceCount, len(devs))
			}
			if tc.want.deviceCheck != nil {
				tc.want.deviceCheck(t, devs)
			}
			if tc.want.annotationCheck != nil {
				tc.want.annotationCheck(t, updated.Annotations)
			}
		})
	}
}
