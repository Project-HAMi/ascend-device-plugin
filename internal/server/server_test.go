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
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"testing"

	"google.golang.org/grpc/grpclog"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/Project-HAMi/HAMi/pkg/device"
	"github.com/Project-HAMi/HAMi/pkg/device/ascend"
	"github.com/Project-HAMi/HAMi/pkg/util"
	"github.com/Project-HAMi/HAMi/pkg/util/client"
	"github.com/Project-HAMi/HAMi/pkg/util/nodelock"
	"github.com/Project-HAMi/ascend-device-plugin/internal/manager"
)

const testCommonWord = "Ascend910"

// CleanupFunc is the return type of test setup helpers that modify global state.
// Callers typically pass the returned function to t.Cleanup.
type CleanupFunc func()

// cd is a test helper that constructs a device.ContainerDevice.
func cd(uuid, typ string, usedmem, usedcores int32) device.ContainerDevice {
	return device.ContainerDevice{
		UUID:      uuid,
		Type:      typ,
		Usedmem:   usedmem,
		Usedcores: usedcores,
	}
}

// setupInRequestDevices registers the test commonWord in device.InRequestDevices
// and DevicesToHandle, and returns a cleanup function to restore the original state.
func setupInRequestDevices(commonWord string) CleanupFunc {
	origAnno := device.InRequestDevices[commonWord]
	device.InRequestDevices[commonWord] = fmt.Sprintf("hami.io/%s-devices-to-allocate", commonWord)

	origDevicesToHandle := device.DevicesToHandle
	device.DevicesToHandle = append(device.DevicesToHandle, commonWord)

	return func() {
		delete(device.InRequestDevices, commonWord)
		if origAnno != "" {
			device.InRequestDevices[commonWord] = origAnno
		}
		device.DevicesToHandle = origDevicesToHandle
	}
}

// setupFakeClient sets client.KubeClient to a fake clientset pre-loaded with
// the given pods and nodes, and returns a cleanup function to restore the
// original client.
func setupFakeClient(pods []*v1.Pod, nodes []*v1.Node) CleanupFunc {
	orig := client.KubeClient
	fc := fake.NewSimpleClientset()
	for _, p := range pods {
		_, _ = fc.CoreV1().Pods(p.Namespace).Create(context.Background(), p, metav1.CreateOptions{})
	}
	for _, n := range nodes {
		_, _ = fc.CoreV1().Nodes().Create(context.Background(), n, metav1.CreateOptions{})
	}
	client.KubeClient = fc
	return func() { client.KubeClient = orig }
}

// setupAllocateEnv creates a fake clientset with a node (with nodelock annotation
// pointing to the pod) and a pod with the specified number of containers, returning
// both along with a cleanup function.
func setupAllocateEnv(nodeName, podName, podNamespace string, numContainers int, podAnnotations map[string]string) (*v1.Node, *v1.Pod, CleanupFunc) {
	lockValue := fmt.Sprintf("2024-01-01T00:00:00Z,%s,%s", podNamespace, podName)
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
			Annotations: map[string]string{
				nodelock.NodeLockKey: lockValue,
			},
		},
	}
	containers := make([]v1.Container, numContainers)
	for i := range containers {
		containers[i].Name = fmt.Sprintf("ctr-%d", i)
	}
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        podName,
			Namespace:   podNamespace,
			Annotations: podAnnotations,
		},
		Spec: v1.PodSpec{
			NodeName:   nodeName,
			Containers: containers,
		},
	}
	cleanup := setupFakeClient([]*v1.Pod{pod}, []*v1.Node{node})
	return node, pod, cleanup
}

// ============================================================================
// Allocate tests
// ============================================================================

// composeCleanup combines multiple CleanupFuncs into one that runs in reverse order.
func composeCleanup(fns ...CleanupFunc) CleanupFunc {
	return func() {
		for i := len(fns) - 1; i >= 0; i-- {
			fns[i]()
		}
	}
}

func TestAllocate(t *testing.T) {
	type allocateArgs struct {
		ps   *PluginServer
		reqs *v1beta1.AllocateRequest
	}

	type allocateWant struct {
		containerResponses []*v1beta1.ContainerAllocateResponse
		nodeLockReleased   bool
	}

	tests := []struct {
		name    string
		args    allocateArgs
		want    allocateWant
		wantErr string
		setup   func() CleanupFunc
	}{
		{
			name: "GetPendingPodFails",
			args: allocateArgs{
				ps: &PluginServer{
					commonWord:        testCommonWord,
					nodeName:          "missing-node",
					toAllocDeviceAnno: "hami.io/Ascend910-devices-to-allocate",
					allocAnno:         "huawei.com/Ascend910",
					mgr:               &FakeManager{},
				},
				reqs: &v1beta1.AllocateRequest{
					ContainerRequests: []*v1beta1.ContainerAllocateRequest{
						{DevicesIds: []string{"uuid1-0"}},
					},
				},
			},
			wantErr: "get pending pod",
			setup:   func() CleanupFunc { return setupFakeClient(nil, nil) },
		},
		{
			name: "SingleContainerSingleDevice",
			args: allocateArgs{
				ps: &PluginServer{
					commonWord:        testCommonWord,
					nodeName:          "test-node",
					toAllocDeviceAnno: "hami.io/Ascend910-devices-to-allocate",
					allocAnno:         "huawei.com/Ascend910",
					mgr: &FakeManager{
						GetDeviceByUUIDFunc: func(uuid string) *manager.Device {
							return &manager.Device{UUID: "uuid1", PhyID: 3}
						},
					},
				},
				reqs: &v1beta1.AllocateRequest{
					ContainerRequests: []*v1beta1.ContainerAllocateRequest{
						{DevicesIds: []string{"uuid1-0"}},
					},
				},
			},
			want: allocateWant{
				containerResponses: []*v1beta1.ContainerAllocateResponse{
					{Envs: map[string]string{"ASCEND_VISIBLE_DEVICES": "3", "ASCEND_VNPU_SPECS": "vir01"}},
				},
				nodeLockReleased: true,
			},
			setup: func() CleanupFunc {
				c1 := setupInRequestDevices("Ascend910")
				toAllocAnno := "hami.io/Ascend910-devices-to-allocate"
				allocAnno := "huawei.com/Ascend910"
				containerDevs := device.EncodePodSingleDevice(device.PodSingleDevice{
					{cd("uuid1", "Ascend910", 1024, 4)},
				})
				rtInfo := []ascend.RuntimeInfo{{UUID: "uuid1", Temp: "vir01"}}
				rtData, _ := json.Marshal(rtInfo)
				_, _, c2 := setupAllocateEnv("test-node", "test-pod", "default", 1, map[string]string{
					toAllocAnno:                           containerDevs,
					allocAnno:                             string(rtData),
					util.BindTimeAnnotations:              "2024-01-01T00:00:00Z",
					util.DeviceBindPhase:                  util.DeviceBindAllocating,
					"hami.io/Ascend910-devices-allocated": containerDevs,
				})
				return composeCleanup(c2, c1)
			},
		},
		{
			name: "DeviceNumberMismatch",
			args: allocateArgs{
				ps: &PluginServer{
					commonWord:        testCommonWord,
					nodeName:          "test-node",
					toAllocDeviceAnno: "hami.io/Ascend910-devices-to-allocate",
					allocAnno:         "huawei.com/Ascend910",
					mgr: &FakeManager{
						GetDeviceByUUIDFunc: func(uuid string) *manager.Device {
							return &manager.Device{UUID: uuid, PhyID: 0}
						},
					},
				},
				reqs: &v1beta1.AllocateRequest{
					ContainerRequests: []*v1beta1.ContainerAllocateRequest{
						{DevicesIds: []string{"uuid1-0"}},
					},
				},
			},
			wantErr: "device number not matched",
			setup: func() CleanupFunc {
				c1 := setupInRequestDevices("Ascend910")
				toAllocAnno := "hami.io/Ascend910-devices-to-allocate"
				allocAnno := "huawei.com/Ascend910"
				containerDevs := device.EncodePodSingleDevice(device.PodSingleDevice{
					{cd("uuid1", "Ascend910", 1024, 4), cd("uuid2", "Ascend910", 2048, 8)},
				})
				rtInfo := []ascend.RuntimeInfo{
					{UUID: "uuid1", Temp: "vir01"},
					{UUID: "uuid2", Temp: "vir01"},
				}
				rtData, _ := json.Marshal(rtInfo)
				_, _, c2 := setupAllocateEnv("test-node", "test-pod", "default", 1, map[string]string{
					toAllocAnno:              containerDevs,
					allocAnno:                string(rtData),
					util.BindTimeAnnotations: "2024-01-01T00:00:00Z",
					util.DeviceBindPhase:     util.DeviceBindAllocating,
				})
				return composeCleanup(c2, c1)
			},
		},
		{
			name: "MultiContainer",
			args: allocateArgs{
				ps: &PluginServer{
					commonWord:        testCommonWord,
					nodeName:          "test-node",
					toAllocDeviceAnno: "hami.io/Ascend910-devices-to-allocate",
					allocAnno:         "huawei.com/Ascend910",
					mgr: &FakeManager{
						GetDeviceByUUIDFunc: func(uuid string) *manager.Device {
							switch uuid {
							case "uuid1":
								return &manager.Device{UUID: "uuid1", PhyID: 0}
							case "uuid2":
								return &manager.Device{UUID: "uuid2", PhyID: 1}
							default:
								return nil
							}
						},
					},
				},
				reqs: &v1beta1.AllocateRequest{
					ContainerRequests: []*v1beta1.ContainerAllocateRequest{
						{DevicesIds: []string{"uuid1-0"}},
						{DevicesIds: []string{"uuid2-0"}},
					},
				},
			},
			want: allocateWant{
				containerResponses: []*v1beta1.ContainerAllocateResponse{
					{Envs: map[string]string{"ASCEND_VISIBLE_DEVICES": "0", "ASCEND_VNPU_SPECS": "vir01"}},
					{Envs: map[string]string{"ASCEND_VISIBLE_DEVICES": "1", "ASCEND_VNPU_SPECS": "vir02"}},
				},
			},
			setup: func() CleanupFunc {
				c1 := setupInRequestDevices("Ascend910")
				toAllocAnno := "hami.io/Ascend910-devices-to-allocate"
				allocAnno := "huawei.com/Ascend910"
				containerDevs := device.EncodePodSingleDevice(device.PodSingleDevice{
					{cd("uuid1", "Ascend910", 1024, 4)},
					{cd("uuid2", "Ascend910", 2048, 8)},
				})
				rtInfo := []ascend.RuntimeInfo{
					{UUID: "uuid1", Temp: "vir01"},
					{UUID: "uuid2", Temp: "vir02"},
				}
				rtData, _ := json.Marshal(rtInfo)
				_, _, c2 := setupAllocateEnv("test-node", "test-pod", "default", 2, map[string]string{
					toAllocAnno:              containerDevs,
					allocAnno:                string(rtData),
					util.BindTimeAnnotations: "2024-01-01T00:00:00Z",
					util.DeviceBindPhase:     util.DeviceBindAllocating,
				})
				return composeCleanup(c2, c1)
			},
		},
		{
			name: "BuildResponseError",
			args: allocateArgs{
				ps: &PluginServer{
					commonWord:        testCommonWord,
					nodeName:          "test-node",
					toAllocDeviceAnno: "hami.io/Ascend910-devices-to-allocate",
					allocAnno:         "huawei.com/Ascend910",
					mgr:               &FakeManager{},
				},
				reqs: &v1beta1.AllocateRequest{
					ContainerRequests: []*v1beta1.ContainerAllocateRequest{
						{DevicesIds: []string{"unknown-uuid-0"}},
					},
				},
			},
			wantErr: "unknown uuid",
			setup: func() CleanupFunc {
				c1 := setupInRequestDevices("Ascend910")
				toAllocAnno := "hami.io/Ascend910-devices-to-allocate"
				allocAnno := "huawei.com/Ascend910"
				containerDevs := device.EncodePodSingleDevice(device.PodSingleDevice{
					{cd("unknown-uuid", "Ascend910", 1024, 4)},
				})
				rtInfo := []ascend.RuntimeInfo{{UUID: "unknown-uuid", Temp: "vir01"}}
				rtData, _ := json.Marshal(rtInfo)
				_, _, c2 := setupAllocateEnv("test-node", "test-pod", "default", 1, map[string]string{
					toAllocAnno:              containerDevs,
					allocAnno:                string(rtData),
					util.BindTimeAnnotations: "2024-01-01T00:00:00Z",
					util.DeviceBindPhase:     util.DeviceBindAllocating,
				})
				return composeCleanup(c2, c1)
			},
		},
		{
			name: "NodeLockReleasedOnError",
			args: allocateArgs{
				ps: &PluginServer{
					commonWord:        testCommonWord,
					nodeName:          "test-node",
					toAllocDeviceAnno: "hami.io/Ascend910-devices-to-allocate",
					allocAnno:         "huawei.com/Ascend910",
					mgr:               &FakeManager{},
				},
				reqs: &v1beta1.AllocateRequest{
					ContainerRequests: []*v1beta1.ContainerAllocateRequest{
						{DevicesIds: []string{"uuid1-0"}},
					},
				},
			},
			want: allocateWant{
				nodeLockReleased: true,
			},
			wantErr: "annotation",
			setup: func() CleanupFunc {
				c1 := setupInRequestDevices("Ascend910")
				toAllocAnno := "hami.io/Ascend910-devices-to-allocate"
				containerDevs := device.EncodePodSingleDevice(device.PodSingleDevice{
					{cd("uuid1", "Ascend910", 1024, 4)},
				})
				_, _, c2 := setupAllocateEnv("test-node", "test-pod", "default", 1, map[string]string{
					toAllocAnno:              containerDevs,
					util.BindTimeAnnotations: "2024-01-01T00:00:00Z",
					util.DeviceBindPhase:     util.DeviceBindAllocating,
				})
				return composeCleanup(c2, c1)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(tc.setup())

			resp, err := tc.args.ps.Allocate(context.Background(), tc.args.reqs)

			if tc.wantErr != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error should contain %q, got: %v", tc.wantErr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}

			// Check container responses
			if tc.want.containerResponses != nil && resp != nil {
				if len(resp.ContainerResponses) != len(tc.want.containerResponses) {
					t.Fatalf("expected %d container responses, got %d", len(tc.want.containerResponses), len(resp.ContainerResponses))
				}
				for i, wantCR := range tc.want.containerResponses {
					gotCR := resp.ContainerResponses[i]
					for k, wantVal := range wantCR.Envs {
						if gotCR.Envs[k] != wantVal {
							t.Fatalf("container[%d] env[%q] = %q, want %q", i, k, gotCR.Envs[k], wantVal)
						}
					}
				}
			}

			// Check node lock state
			if tc.want.nodeLockReleased && tc.args.ps.nodeName != "missing-node" {
				updatedNode, nErr := client.KubeClient.CoreV1().Nodes().Get(context.Background(), tc.args.ps.nodeName, metav1.GetOptions{})
				if nErr != nil {
					t.Fatalf("failed to get node: %v", nErr)
				}
				if _, hasLock := updatedNode.Annotations[nodelock.NodeLockKey]; hasLock {
					t.Fatal("node lock should have been released")
				}
			}
		})
	}
}

// ============================================================================
// NewPluginServer tests
// ============================================================================

func TestNewPluginServer(t *testing.T) {
	t.Parallel()

	type newPluginServerArgs struct {
		commonWord string
		nodeName   string
	}

	type newPluginServerWant struct {
		registerAnno  string
		handshakeAnno string
		allocAnno     string
		toAllocAnno   string
	}

	tests := []struct {
		name    string
		args    newPluginServerArgs
		want    newPluginServerWant
		wantErr bool
	}{
		{
			name: "Ascend910A",
			args: newPluginServerArgs{commonWord: "Ascend910A", nodeName: "test-node"},
			want: newPluginServerWant{
				registerAnno:  "hami.io/node-register-Ascend910A",
				handshakeAnno: "hami.io/node-handshake-Ascend910A",
				allocAnno:     "huawei.com/Ascend910A",
				toAllocAnno:   "hami.io/Ascend910A-devices-to-allocate",
			},
		},
		{
			name: "Ascend910B2",
			args: newPluginServerArgs{commonWord: "Ascend910B2", nodeName: "test-node"},
			want: newPluginServerWant{
				registerAnno:  "hami.io/node-register-Ascend910B2",
				handshakeAnno: "hami.io/node-handshake-Ascend910B2",
				allocAnno:     "huawei.com/Ascend910B2",
				toAllocAnno:   "hami.io/Ascend910B2-devices-to-allocate",
			},
		},
		{
			name: "Ascend910B3",
			args: newPluginServerArgs{commonWord: "Ascend910B3", nodeName: "test-node"},
			want: newPluginServerWant{
				registerAnno:  "hami.io/node-register-Ascend910B3",
				handshakeAnno: "hami.io/node-handshake-Ascend910B3",
				allocAnno:     "huawei.com/Ascend910B3",
				toAllocAnno:   "hami.io/Ascend910B3-devices-to-allocate",
			},
		},
		{
			name: "Ascend910B4-1",
			args: newPluginServerArgs{commonWord: "Ascend910B4-1", nodeName: "test-node"},
			want: newPluginServerWant{
				registerAnno:  "hami.io/node-register-Ascend910B4-1",
				handshakeAnno: "hami.io/node-handshake-Ascend910B4-1",
				allocAnno:     "huawei.com/Ascend910B4-1",
				toAllocAnno:   "hami.io/Ascend910B4-1-devices-to-allocate",
			},
		},
		{
			name: "Ascend910B4",
			args: newPluginServerArgs{commonWord: "Ascend910B4", nodeName: "test-node"},
			want: newPluginServerWant{
				registerAnno:  "hami.io/node-register-Ascend910B4",
				handshakeAnno: "hami.io/node-handshake-Ascend910B4",
				allocAnno:     "huawei.com/Ascend910B4",
				toAllocAnno:   "hami.io/Ascend910B4-devices-to-allocate",
			},
		},
		{
			name: "Ascend310P",
			args: newPluginServerArgs{commonWord: "Ascend310P", nodeName: "test-node"},
			want: newPluginServerWant{
				registerAnno:  "hami.io/node-register-Ascend310P",
				handshakeAnno: "hami.io/node-handshake-Ascend310P",
				allocAnno:     "huawei.com/Ascend310P",
				toAllocAnno:   "hami.io/Ascend310P-devices-to-allocate",
			},
		},
		{
			name: "Ascend910C",
			args: newPluginServerArgs{commonWord: "Ascend910C", nodeName: "test-node"},
			want: newPluginServerWant{
				registerAnno:  "hami.io/node-register-Ascend910C",
				handshakeAnno: "hami.io/node-handshake-Ascend910C",
				allocAnno:     "huawei.com/Ascend910C",
				toAllocAnno:   "hami.io/Ascend910C-devices-to-allocate",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mgr := &FakeManager{CommonWordFunc: func() string { return tc.args.commonWord }}
			ps, err := NewPluginServer(mgr, tc.args.nodeName, 60)
			if (err != nil) != tc.wantErr {
				t.Fatalf("NewPluginServer() error = %v, wantErr %v", err, tc.wantErr)
			}
			if ps.registerAnno != tc.want.registerAnno {
				t.Errorf("registerAnno = %q, want %q", ps.registerAnno, tc.want.registerAnno)
			}
			if ps.handshakeAnno != tc.want.handshakeAnno {
				t.Errorf("handshakeAnno = %q, want %q", ps.handshakeAnno, tc.want.handshakeAnno)
			}
			if ps.allocAnno != tc.want.allocAnno {
				t.Errorf("allocAnno = %q, want %q", ps.allocAnno, tc.want.allocAnno)
			}
			if ps.toAllocDeviceAnno != tc.want.toAllocAnno {
				t.Errorf("toAllocDeviceAnno = %q, want %q", ps.toAllocDeviceAnno, tc.want.toAllocAnno)
			}
		})
	}
}

func TestNewPluginServer_RegistersInRequestDevices(t *testing.T) {
	commonWord := "Ascend910"
	origVal := device.InRequestDevices[commonWord]
	delete(device.InRequestDevices, commonWord)
	defer func() {
		if origVal != "" {
			device.InRequestDevices[commonWord] = origVal
		} else {
			delete(device.InRequestDevices, commonWord)
		}
	}()

	mgr := &FakeManager{
		CommonWordFunc: func() string { return commonWord },
	}
	ps, err := NewPluginServer(mgr, "test-node", 60)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, ok := device.InRequestDevices[commonWord]
	if !ok {
		t.Fatal("InRequestDevices[commonWord] not registered")
	}
	want := ps.toAllocDeviceAnno
	if got != want {
		t.Errorf("InRequestDevices[%q] = %q, want %q", commonWord, got, want)
	}
}

// ============================================================================
// apiDevices tests
// ============================================================================

func TestApiDevices(t *testing.T) {
	type apiDevicesArgs struct {
		mgr *FakeManager
	}

	tests := []struct {
		name string
		args apiDevicesArgs
		want []*v1beta1.Device
	}{
		{
			name: "EmptyDeviceList",
			args: apiDevicesArgs{
				mgr: &FakeManager{
					GetDevicesFunc:   func() []*manager.Device { return nil },
					VDeviceCountFunc: func() int { return 1 },
				},
			},
			want: nil,
		},
		{
			name: "SingleDeviceVCount1",
			args: apiDevicesArgs{
				mgr: &FakeManager{
					GetDevicesFunc: func() []*manager.Device {
						return []*manager.Device{{UUID: "uuid1", Health: true}}
					},
					VDeviceCountFunc: func() int { return 1 },
				},
			},
			want: []*v1beta1.Device{
				{ID: "uuid1-0", Health: v1beta1.Healthy},
			},
		},
		{
			name: "SingleDeviceVCount3",
			args: apiDevicesArgs{
				mgr: &FakeManager{
					GetDevicesFunc: func() []*manager.Device {
						return []*manager.Device{{UUID: "uuid1", Health: true}}
					},
					VDeviceCountFunc: func() int { return 3 },
				},
			},
			want: []*v1beta1.Device{
				{ID: "uuid1-0", Health: v1beta1.Healthy},
				{ID: "uuid1-1", Health: v1beta1.Healthy},
				{ID: "uuid1-2", Health: v1beta1.Healthy},
			},
		},
		{
			name: "UnhealthyDevice",
			args: apiDevicesArgs{
				mgr: &FakeManager{
					GetDevicesFunc: func() []*manager.Device {
						return []*manager.Device{{UUID: "uuid1", Health: false}}
					},
					VDeviceCountFunc: func() int { return 1 },
				},
			},
			want: []*v1beta1.Device{
				{ID: "uuid1-0", Health: v1beta1.Unhealthy},
			},
		},
		{
			name: "MultipleDevicesDifferentHealth",
			args: apiDevicesArgs{
				mgr: &FakeManager{
					GetDevicesFunc: func() []*manager.Device {
						return []*manager.Device{
							{UUID: "uuid1", Health: true},
							{UUID: "uuid2", Health: false},
						}
					},
					VDeviceCountFunc: func() int { return 2 },
				},
			},
			want: []*v1beta1.Device{
				{ID: "uuid1-0", Health: v1beta1.Healthy},
				{ID: "uuid1-1", Health: v1beta1.Healthy},
				{ID: "uuid2-0", Health: v1beta1.Unhealthy},
				{ID: "uuid2-1", Health: v1beta1.Unhealthy},
			},
		},
		{
			name: "VDeviceCountZero",
			args: apiDevicesArgs{
				mgr: &FakeManager{
					GetDevicesFunc: func() []*manager.Device {
						return []*manager.Device{{UUID: "uuid1", Health: true}}
					},
					VDeviceCountFunc: func() int { return 0 },
				},
			},
			want: nil,
		},
		{
			name: "EmptySliceDeviceList",
			args: apiDevicesArgs{
				mgr: &FakeManager{
					GetDevicesFunc:   func() []*manager.Device { return []*manager.Device{} },
					VDeviceCountFunc: func() int { return 1 },
				},
			},
			want: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ps := &PluginServer{mgr: tc.args.mgr}
			got := ps.apiDevices()

			if len(got) != len(tc.want) {
				t.Fatalf("expected %d devices, got %d", len(tc.want), len(got))
			}
			for i, wantDev := range tc.want {
				if got[i].ID != wantDev.ID {
					t.Errorf("device[%d].ID = %q, want %q", i, got[i].ID, wantDev.ID)
				}
				if got[i].Health != wantDev.Health {
					t.Errorf("device[%d].Health = %v, want %v", i, got[i].Health, wantDev.Health)
				}
			}
		})
	}
}

// ============================================================================
// CleanupIdleVNPUs tests
// ============================================================================

func TestCleanupIdleVNPUs(t *testing.T) {
	type cleanupIdleVNPUsArgs struct {
		mgr *FakeManager
	}

	tests := []struct {
		name    string
		args    cleanupIdleVNPUsArgs
		wantErr bool
	}{
		{
			name: "DelegatesToManager",
			args: cleanupIdleVNPUsArgs{
				mgr: &FakeManager{
					CleanupIdleVNPUsFunc: func() error { return nil },
				},
			},
		},
		{
			name: "ReturnsManagerError",
			args: cleanupIdleVNPUsArgs{
				mgr: &FakeManager{
					CleanupIdleVNPUsFunc: func() error { return fmt.Errorf("cleanup failed") },
				},
			},
			wantErr: true,
		},
		{
			name: "NilFuncReturnsNil",
			args: cleanupIdleVNPUsArgs{
				mgr: &FakeManager{},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ps := &PluginServer{mgr: tc.args.mgr}
			err := ps.CleanupIdleVNPUs()
			if (err != nil) != tc.wantErr {
				t.Fatalf("CleanupIdleVNPUs() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

// ============================================================================
// gRPC restart tests
// ============================================================================

// panicOnFatalLogger is a gRPC logger that converts Fatalf calls to panics.
// This allows tests to verify that gRPC does NOT call Fatalf (which would
// otherwise call os.Exit(1) and abort the test process).
//
// Usage:
//
//	defer grpclog.SetLoggerV2(grpclog.NewLoggerV2(os.Stderr, os.Stderr, os.Stderr))
//	grpclog.SetLoggerV2(newPanicOnFatalLogger())
type panicOnFatalLogger struct {
	inner grpclog.LoggerV2
}

func newPanicOnFatalLogger() *panicOnFatalLogger {
	return &panicOnFatalLogger{
		inner: grpclog.NewLoggerV2(os.Stderr, os.Stderr, os.Stderr),
	}
}

var _ grpclog.LoggerV2 = (*panicOnFatalLogger)(nil)

func (l *panicOnFatalLogger) Info(args ...interface{})   { l.inner.Info(args...) }
func (l *panicOnFatalLogger) Infoln(args ...interface{}) { l.inner.Infoln(args...) }
func (l *panicOnFatalLogger) Infof(format string, args ...interface{}) {
	l.inner.Infof(format, args...)
}
func (l *panicOnFatalLogger) Warning(args ...interface{})   { l.inner.Warning(args...) }
func (l *panicOnFatalLogger) Warningln(args ...interface{}) { l.inner.Warningln(args...) }
func (l *panicOnFatalLogger) Warningf(format string, args ...interface{}) {
	l.inner.Warningf(format, args...)
}
func (l *panicOnFatalLogger) Error(args ...interface{})   { l.inner.Error(args...) }
func (l *panicOnFatalLogger) Errorln(args ...interface{}) { l.inner.Errorln(args...) }
func (l *panicOnFatalLogger) Errorf(format string, args ...interface{}) {
	l.inner.Errorf(format, args...)
}
func (l *panicOnFatalLogger) V(level int) bool { return l.inner.V(level) }

func (l *panicOnFatalLogger) Fatalf(format string, args ...interface{}) {
	panic(fmt.Sprintf("grpc FATAL: "+format, args...))
}

func (l *panicOnFatalLogger) Fatalln(args ...interface{}) {
	panic(fmt.Sprintf("grpc FATAL: %v", fmt.Sprintln(args...)))
}

func (l *panicOnFatalLogger) Fatal(args ...interface{}) {
	panic(fmt.Sprintf("grpc FATAL: %v", fmt.Sprint(args...)))
}

// setupRestartablePluginServer creates a PluginServer with all test hooks
// injected so that Start()/Stop() work without real socket files or a kubelet.
func setupRestartablePluginServer(t *testing.T) *PluginServer {
	t.Helper()

	ps := &PluginServer{
		commonWord:            "test-ascend",
		registerAnno:          "hami.io/node-register-test-ascend",
		handshakeAnno:         "hami.io/node-handshake-test-ascend",
		allocAnno:             "huawei.com/test-ascend",
		toAllocDeviceAnno:     "hami.io/test-ascend-devices-to-allocate",
		mgr:                   &FakeManager{ResourceNameFunc: func() string { return "test-ascend" }},
		socket:                path.Join(t.TempDir(), "test-ascend.sock"),
		stopCh:                make(chan interface{}),
		healthCh:              make(chan int32),
		checkIdleVNPUInterval: 3600,
		dialFunc:              nil,
		registerKubeletFunc: func() error {
			return nil
		},
		prepareHostResourcesFunc: func() error {
			return nil
		},
	}
	return ps
}

// TestGrpcServer_RestartDoesNotPanic verifies that a single Stop+Start cycle
// does not trigger the gRPC "RegisterService after Serve" fatal error.
func TestGrpcServer_RestartDoesNotPanic(t *testing.T) {
	defer grpclog.SetLoggerV2(grpclog.NewLoggerV2(os.Stderr, os.Stderr, os.Stderr))
	grpclog.SetLoggerV2(newPanicOnFatalLogger())

	ps := setupRestartablePluginServer(t)

	// First Start
	if err := ps.Start(); err != nil {
		t.Fatalf("first Start() failed: %v", err)
	}

	// Stop
	if err := ps.Stop(); err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}

	// Second Start — this must not trigger grpc Fatalf
	if err := ps.Start(); err != nil {
		t.Fatalf("second Start() after restart failed: %v", err)
	}

	// Cleanup
	if err := ps.Stop(); err != nil {
		t.Fatalf("final Stop() failed: %v", err)
	}
}

// TestGrpcServer_MultipleRestarts verifies that the server can survive
// multiple Stop+Start cycles without panic.
func TestGrpcServer_MultipleRestarts(t *testing.T) {
	defer grpclog.SetLoggerV2(grpclog.NewLoggerV2(os.Stderr, os.Stderr, os.Stderr))
	grpclog.SetLoggerV2(newPanicOnFatalLogger())

	ps := setupRestartablePluginServer(t)

	for i := 0; i < 5; i++ {
		if err := ps.Start(); err != nil {
			t.Fatalf("Start() iteration %d failed: %v", i, err)
		}
		if err := ps.Stop(); err != nil {
			t.Fatalf("Stop() iteration %d failed: %v", i, err)
		}
	}
}

// TestGrpcServer_StopWithoutStart verifies that Stop() is safe when
// Start() was never called (no goroutines to wait for).
func TestGrpcServer_StopWithoutStart(t *testing.T) {
	ps := setupRestartablePluginServer(t)
	if err := ps.Stop(); err != nil {
		t.Fatalf("Stop() without Start() should be safe: %v", err)
	}
}

// TestGrpcServer_DoubleStop verifies that calling Stop() twice is safe.
func TestGrpcServer_DoubleStop(t *testing.T) {
	ps := setupRestartablePluginServer(t)

	if err := ps.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	if err := ps.Stop(); err != nil {
		t.Fatalf("first Stop() failed: %v", err)
	}

	if err := ps.Stop(); err != nil {
		t.Fatalf("second Stop() should be safe: %v", err)
	}
}

// TestGrpcServer_StopWaitForAllGoroutines verifies that Stop() returns
// only after all goroutines have exited. We verify this indirectly by
// checking that Start() after Stop() does not race (goroutine leak would
// manifest as stale channel reads).
func TestGrpcServer_StopWaitForAllGoroutines(t *testing.T) {
	ps := setupRestartablePluginServer(t)

	if err := ps.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	if err := ps.Stop(); err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}

	// bgWG and serveWG should be zero after Stop() returns.
	// A new cycle confirms there is no deadlock or hang.
	if err := ps.Start(); err != nil {
		t.Fatalf("Start() after Stop() failed: %v", err)
	}

	if err := ps.Stop(); err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}
}
