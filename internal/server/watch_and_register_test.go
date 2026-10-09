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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/Project-HAMi/HAMi/pkg/device"
	"github.com/Project-HAMi/HAMi/pkg/util/client"
	"github.com/Project-HAMi/ascend-device-plugin/internal/manager"
)

const (
	watchRegisterNode       = "test-node"
	watchRegisterCommonWord = "Ascend910B4-1"
	// watchAndRegister runs its first iteration one second after it starts.
	watchRegisterRunFor = 2 * time.Second
)

// cachingFakeManager reproduces AscendManager's caching semantics on top of a
// mutable hardware view: UpdateDevice() copies the hardware view into the
// cache, GetDevices() returns the cache, and GetUnHealthIDs() reads the
// hardware live. That distinction is what the tests below exercise.
type cachingFakeManager struct {
	*FakeManager

	mu       sync.Mutex
	hardware []*manager.Device
	cache    []*manager.Device
	updates  atomic.Int32
}

func newCachingFakeManager(hardware ...*manager.Device) *cachingFakeManager {
	m := &cachingFakeManager{hardware: hardware}
	m.FakeManager = &FakeManager{
		CommonWordFunc:     func() string { return watchRegisterCommonWord },
		ResourceNameFunc:   func() string { return "huawei.com/" + watchRegisterCommonWord },
		VDeviceCountFunc:   func() int { return 4 },
		IsHamiVnpuCoreFunc: func() bool { return false },
		UpdateDeviceFunc: func() error {
			m.mu.Lock()
			defer m.mu.Unlock()
			m.updates.Add(1)
			m.cache = make([]*manager.Device, 0, len(m.hardware))
			for _, dev := range m.hardware {
				copied := *dev
				m.cache = append(m.cache, &copied)
			}
			return nil
		},
		GetDevicesFunc: func() []*manager.Device {
			m.mu.Lock()
			defer m.mu.Unlock()
			return m.cache
		},
		GetUnHealthIDsFunc: func() []int32 {
			m.mu.Lock()
			defer m.mu.Unlock()
			var unhealthy []int32
			for _, dev := range m.hardware {
				if !dev.Health {
					unhealthy = append(unhealthy, dev.LogicID)
				}
			}
			return unhealthy
		},
	}
	return m
}

func (m *cachingFakeManager) setHardware(hardware ...*manager.Device) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hardware = hardware
}

func fakeDevices(count int, healthy bool) []*manager.Device {
	devs := make([]*manager.Device, 0, count)
	for i := range count {
		devs = append(devs, &manager.Device{
			UUID:    fmt.Sprintf("uuid%d", i),
			LogicID: int32(i),
			PhyID:   int32(i),
			CardID:  int32(i),
			Memory:  65536,
			AICore:  20,
			Health:  healthy,
		})
	}
	return devs
}

// recordingLWStream captures every device list handed to kubelet.
type recordingLWStream struct {
	grpc.ServerStream

	mu        sync.Mutex
	responses [][]*v1beta1.Device
}

func (s *recordingLWStream) Send(resp *v1beta1.ListAndWatchResponse) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	devices := make([]*v1beta1.Device, len(resp.Devices))
	copy(devices, resp.Devices)
	s.responses = append(s.responses, devices)
	return nil
}

func (s *recordingLWStream) Context() context.Context { return context.Background() }

func (s *recordingLWStream) sent() [][]*v1beta1.Device {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([][]*v1beta1.Device(nil), s.responses...)
}

func countHealthy(devices []*v1beta1.Device) int {
	healthy := 0
	for _, dev := range devices {
		if dev.Health == v1beta1.Healthy {
			healthy++
		}
	}
	return healthy
}

// newWatchRegisterServer builds a PluginServer wired to mgr and to a fake
// clientset holding the node that registerHAMi annotates. It also performs the
// initial UpdateDevice()/fingerprint that Start() performs.
func newWatchRegisterServer(t *testing.T, mgr *cachingFakeManager) *PluginServer {
	t.Helper()

	node := &v1.Node{ObjectMeta: metav1.ObjectMeta{
		Name:        watchRegisterNode,
		Annotations: map[string]string{},
	}}
	t.Cleanup(setupFakeClient(nil, []*v1.Node{node}))

	ps := &PluginServer{
		commonWord:        watchRegisterCommonWord,
		nodeName:          watchRegisterNode,
		registerAnno:      "hami.io/node-register-" + watchRegisterCommonWord,
		handshakeAnno:     "hami.io/node-handshake-" + watchRegisterCommonWord,
		allocAnno:         "huawei.com/" + watchRegisterCommonWord,
		toAllocDeviceAnno: "hami.io/" + watchRegisterCommonWord + "-devices-to-allocate",
		mgr:               mgr,
		stopCh:            make(chan any),
		healthCh:          make(chan int32),
	}
	if err := ps.mgr.UpdateDevice(); err != nil {
		t.Fatalf("initial UpdateDevice() failed: %v", err)
	}
	ps.lastPublishedDevices = ps.deviceFingerprint()
	mgr.updates.Store(0)
	return ps
}

// runWatchAndRegister runs watchAndRegister, and optionally a ListAndWatch
// consumer, for the given duration and then stops both.
func runWatchAndRegister(t *testing.T, ps *PluginServer, stream *recordingLWStream, d time.Duration) {
	t.Helper()

	var wg sync.WaitGroup
	if stream != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := ps.ListAndWatch(&v1beta1.Empty{}, stream); err != nil {
				t.Errorf("ListAndWatch returned an error: %v", err)
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		ps.watchAndRegister()
	}()

	time.Sleep(d)
	close(ps.stopCh)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("watchAndRegister did not return after stopCh was closed")
	}
}

func registeredDevices(t *testing.T, ps *PluginServer) []*device.DeviceInfo {
	t.Helper()

	node, err := client.KubeClient.CoreV1().Nodes().Get(context.Background(), ps.nodeName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get node: %v", err)
	}
	raw, ok := node.Annotations[ps.registerAnno]
	if !ok {
		t.Fatalf("annotation %s is missing", ps.registerAnno)
	}
	var devs []*device.DeviceInfo
	if err := json.Unmarshal([]byte(raw), &devs); err != nil {
		t.Fatalf("decode %s=%q: %v", ps.registerAnno, raw, err)
	}
	return devs
}

// TestWatchAndRegister_RepublishesRecoveredDevices covers the boot race: the
// plugin starts while the driver still reports the devices as unhealthy, and
// the hardware recovers right afterwards. Before the fix the cache was only
// refreshed while GetUnHealthIDs() was non-empty, so kubelet stayed on the
// startup device list and the node advertised no allocatable devices until the
// Pod was restarted.
func TestWatchAndRegister_RepublishesRecoveredDevices(t *testing.T) {
	mgr := newCachingFakeManager(fakeDevices(4, false)...)
	ps := newWatchRegisterServer(t, mgr)
	stream := &recordingLWStream{}

	mgr.setHardware(fakeDevices(4, true)...)
	runWatchAndRegister(t, ps, stream, watchRegisterRunFor)

	sent := stream.sent()
	if len(sent) < 2 {
		t.Fatalf("expected kubelet to receive an updated device list, got %d response(s)", len(sent))
	}
	last := sent[len(sent)-1]
	if got, want := countHealthy(last), 4*mgr.VDeviceCount(); got != want {
		t.Fatalf("expected %d healthy devices in the last response, got %d of %d", want, got, len(last))
	}
	for _, dev := range registeredDevices(t, ps) {
		if !dev.Health {
			t.Fatalf("expected the node register annotation to report healthy devices, got %+v", dev)
		}
	}
}

// TestWatchAndRegister_PublishesLateDiscoveredDevices covers the second shape
// of the same problem: only part of the devices are enumerated when the plugin
// starts, and the remaining ones appear a moment later.
func TestWatchAndRegister_PublishesLateDiscoveredDevices(t *testing.T) {
	mgr := newCachingFakeManager(fakeDevices(2, true)...)
	ps := newWatchRegisterServer(t, mgr)
	stream := &recordingLWStream{}

	mgr.setHardware(fakeDevices(4, true)...)
	runWatchAndRegister(t, ps, stream, watchRegisterRunFor)

	sent := stream.sent()
	last := sent[len(sent)-1]
	if got, want := len(last), 4*mgr.VDeviceCount(); got != want {
		t.Fatalf("expected %d devices in the last response, got %d", want, got)
	}
	if got := len(registeredDevices(t, ps)); got != 4 {
		t.Fatalf("expected 4 devices in the node register annotation, got %d", got)
	}
}

// TestWatchAndRegister_SteadyStateDoesNotResend makes sure the unconditional
// refresh does not turn into a ListAndWatch update on every iteration.
func TestWatchAndRegister_SteadyStateDoesNotResend(t *testing.T) {
	mgr := newCachingFakeManager(fakeDevices(4, true)...)
	ps := newWatchRegisterServer(t, mgr)
	stream := &recordingLWStream{}

	runWatchAndRegister(t, ps, stream, watchRegisterRunFor)

	if got := len(stream.sent()); got != 1 {
		t.Fatalf("expected only the initial ListAndWatch response in steady state, got %d", got)
	}
	if mgr.updates.Load() == 0 {
		t.Fatal("expected the device cache to be refreshed at least once")
	}
}

// TestWatchAndRegister_ContinuesWithoutListAndWatchConsumer makes sure a device
// change cannot block the loop when kubelet has no ListAndWatch stream open:
// healthCh is unbuffered, and a blocked send would also stop the HAMi node
// registration that follows it.
func TestWatchAndRegister_ContinuesWithoutListAndWatchConsumer(t *testing.T) {
	original := healthUpdateSendTimeout
	healthUpdateSendTimeout = 200 * time.Millisecond
	t.Cleanup(func() { healthUpdateSendTimeout = original })

	mgr := newCachingFakeManager(fakeDevices(4, false)...)
	ps := newWatchRegisterServer(t, mgr)

	mgr.setHardware(fakeDevices(4, true)...)
	runWatchAndRegister(t, ps, nil, watchRegisterRunFor)

	for _, dev := range registeredDevices(t, ps) {
		if !dev.Health {
			t.Fatalf("expected HAMi registration to keep running and report healthy devices, got %+v", dev)
		}
	}
}

func TestSoftSliceCapacityRegistration(t *testing.T) {
	for _, count := range []int{5, 10} {
		t.Run(fmt.Sprintf("split-count-%d", count), func(t *testing.T) {
			mgr := newCachingFakeManager(
				&manager.Device{UUID: "npu-0", Health: true},
				&manager.Device{UUID: "npu-1", Health: true},
			)
			mgr.VDeviceCountFunc = func() int { return count }
			mgr.IsHamiVnpuCoreFunc = func() bool { return true }
			ps := newWatchRegisterServer(t, mgr)
			devices := ps.apiDevices()
			if got := countHealthy(devices); got != 2*count {
				t.Fatalf("healthy kubelet devices = %d, want %d", got, 2*count)
			}
			ids := make(map[string]bool)
			for _, dev := range devices {
				if ids[dev.ID] {
					t.Fatalf("duplicate kubelet device ID %q", dev.ID)
				}
				ids[dev.ID] = true
			}
			if err := ps.registerHAMi(); err != nil {
				t.Fatal(err)
			}
			registered := registeredDevices(t, ps)
			if len(registered) != 2 {
				t.Fatalf("registered physical devices = %d, want 2", len(registered))
			}
			for _, dev := range registered {
				if dev.Count != int32(count) {
					t.Fatalf("registered Count = %d, want %d", dev.Count, count)
				}
			}
		})
	}
}
