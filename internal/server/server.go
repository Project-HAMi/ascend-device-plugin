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
	"flag"
	"fmt"
	"net"
	"os"
	"path"
	"time"

	"google.golang.org/grpc"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	// "github.com/Project-HAMi/HAMi/pkg/device/ascend"
	"github.com/Project-HAMi/HAMi/pkg/device"
	"github.com/Project-HAMi/HAMi/pkg/util"
	"github.com/Project-HAMi/ascend-device-plugin/internal/manager"
)

const (
	// RegisterAnnos = "hami.io/node-register-ascend"
	// PodAllocAnno = "huawei.com/AscendDevices"
	NodeLockAscend             = "hami.io/mutex.lock"
	Ascend910Prefix            = "Ascend910"
	Ascend910CType             = "Ascend910C"
	VNPUModeAnnotation         = "huawei.com/vnpu-mode"
	VNPUModeHamiCore           = "hami-core"
	VNPUNodeSelectorAnnotation = "hami-vnpu-core"
)

var (
	reportTimeOffset = flag.Int64("report_time_offset", 1, "report time offset")
)

type PluginServer struct {
	commonWord            string
	nodeName              string
	registerAnno          string
	handshakeAnno         string
	allocAnno             string
	toAllocDeviceAnno     string
	grpcServer            *grpc.Server
	mgr                   manager.Manager
	socket                string
	stopCh                chan interface{}
	healthCh              chan int32
	checkIdleVNPUInterval int
}

type RuntimeInfo struct {
	UUID   string `json:"UUID,omitempty"`
	Temp   string `json:"temp,omitempty"`
	Memory *int64 `json:"memory,omitempty"`
	Core   *int32 `json:"core,omitempty"`
}

func NewPluginServer(mgr manager.Manager, nodeName string, checkIdleVNPUInterval int) (*PluginServer, error) {
	commonWord := mgr.CommonWord()
	server := &PluginServer{
		commonWord:            commonWord,
		nodeName:              nodeName,
		registerAnno:          fmt.Sprintf("hami.io/node-register-%s", commonWord),
		handshakeAnno:         fmt.Sprintf("hami.io/node-handshake-%s", commonWord),
		allocAnno:             fmt.Sprintf("huawei.com/%s", commonWord),
		toAllocDeviceAnno:     fmt.Sprintf("hami.io/%s-devices-to-allocate", commonWord),
		grpcServer:            grpc.NewServer(),
		mgr:                   mgr,
		socket:                path.Join(v1beta1.DevicePluginPath, fmt.Sprintf("%s.sock", commonWord)),
		stopCh:                make(chan interface{}),
		healthCh:              make(chan int32),
		checkIdleVNPUInterval: checkIdleVNPUInterval,
	}
	// enable calling hami methods
	device.InRequestDevices[commonWord] = server.toAllocDeviceAnno
	return server, nil
}

func (ps *PluginServer) Start() error {
	// Automatically prepare host environment when the plugin starts
	if err := prepareHostResources(); err != nil {
		klog.Errorf("Failed to prepare host resources: %v. vNPU core functionality will be impaired.", err)
		return err
	}

	ps.stopCh = make(chan interface{})
	err := ps.mgr.UpdateDevice()
	if err != nil {
		return err
	}
	err = ps.serve()
	if err != nil {
		return err
	}
	err = ps.registerKubelet()
	if err != nil {
		return err
	}
	go ps.startPeriodicCheckIdleVNPUs()
	go ps.watchAndRegister()
	return nil
}

func (ps *PluginServer) startPeriodicCheckIdleVNPUs() {
	ticker := time.NewTicker(time.Duration(ps.checkIdleVNPUInterval) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			klog.Info("Running scheduled idle vNPU cleanup")
			if err := ps.CleanupIdleVNPUs(); err != nil {
				klog.Errorf("Failed to cleanup idle vNPUs: %v", err)
			}
		case <-ps.stopCh:
			klog.Info("Stopping cleanup goroutine")
			return
		}
	}
}

func (ps *PluginServer) Stop() error {
	close(ps.stopCh)
	ps.grpcServer.Stop()
	return nil
}

func (ps *PluginServer) StopCh() <-chan interface{} {
	return ps.stopCh
}

func (ps *PluginServer) CleanupIdleVNPUs() error {
	return ps.mgr.CleanupIdleVNPUs()
}

func (ps *PluginServer) serve() error {
	_ = os.Remove(ps.socket)
	sock, err := net.Listen("unix", ps.socket)
	if err != nil {
		return err
	}
	v1beta1.RegisterDevicePluginServer(ps.grpcServer, ps)
	resourceName := ps.mgr.ResourceName()
	go func() {
		lastCrashTime := time.Now()
		restartCount := 0
		for {
			select {
			case <-ps.stopCh:
				return
			default:
			}
			klog.Infof("Starting GRPC server for '%s'", resourceName)
			err := ps.grpcServer.Serve(sock)
			if err == nil {
				break
			}

			klog.Infof("GRPC server for '%s' crashed with error: %v", resourceName, err)

			// restart if it has not been too often
			// i.e. if server has crashed more than 5 times and it didn't last more than one hour each time
			if restartCount > 5 {
				// quit
				klog.Fatalf("GRPC server for '%s' has repeatedly crashed recently. Quitting", resourceName)
			}
			timeSinceLastCrash := time.Since(lastCrashTime).Seconds()
			lastCrashTime = time.Now()
			if timeSinceLastCrash > 3600 {
				// it has been one hour since the last crash.. reset the count
				// to reflect on the frequency
				restartCount = 1
			} else {
				restartCount++
			}
		}
	}()

	// Wait for server to start by launching a blocking connexion
	conn, err := ps.dial(ps.socket, 5*time.Second)
	if err != nil {
		return fmt.Errorf("failed to dial device plugin socket: %w", err)
	}
	_ = conn.Close()

	return nil
}

func (ps *PluginServer) apiDevices() []*v1beta1.Device {
	devs := ps.mgr.GetDevices()
	devices := make([]*v1beta1.Device, 0, len(devs))
	vCount := ps.mgr.VDeviceCount()
	for _, dev := range devs {
		health := v1beta1.Unhealthy
		if dev.Health {
			health = v1beta1.Healthy
		}
		for i := 0; i < vCount; i++ {
			device := v1beta1.Device{
				ID:     fmt.Sprintf("%s-%d", dev.UUID, i),
				Health: health,
			}
			devices = append(devices, &device)
		}
	}
	klog.V(5).Infof("api devices: %v", devices)
	return devices
}

func (ps *PluginServer) GetDevicePluginOptions(context.Context, *v1beta1.Empty) (*v1beta1.DevicePluginOptions, error) {
	return &v1beta1.DevicePluginOptions{}, nil
}

func (ps *PluginServer) ListAndWatch(e *v1beta1.Empty, s v1beta1.DevicePlugin_ListAndWatchServer) error {
	_ = s.Send(&v1beta1.ListAndWatchResponse{Devices: ps.apiDevices()})
	for {
		select {
		case <-ps.stopCh:
			return nil
		case <-ps.healthCh:
			_ = s.Send(&v1beta1.ListAndWatchResponse{Devices: ps.apiDevices()})
		}
	}
}

func (ps *PluginServer) GetPreferredAllocation(context.Context, *v1beta1.PreferredAllocationRequest) (*v1beta1.PreferredAllocationResponse, error) {
	return nil, fmt.Errorf("not supported")
}

func (ps *PluginServer) Allocate(ctx context.Context, reqs *v1beta1.AllocateRequest) (*v1beta1.AllocateResponse, error) {
	klog.V(5).Infof("Allocate: %v", reqs)
	success := false
	var pod *v1.Pod
	defer func() {
		if pod == nil {
			return
		}
		if success {
			ps.podAllocationTrySuccess(pod)
		} else {
			ps.podAllocationFailed(pod)
		}
	}()

	var err error
	pod, err = util.GetPendingPod(ctx, ps.nodeName)
	if err != nil {
		klog.Errorf("get pending pod error: %v", err)
		return nil, fmt.Errorf("get pending pod error: %w", err)
	}
	klog.Infof("allocating for pod %s/%s", pod.Namespace, pod.Name)

	rtInfoLookup, err := ps.buildRuntimeInfoLookup(pod)
	if err != nil {
		return nil, fmt.Errorf("build runtimeInfo lookup: %w", err)
	}

	podSingleDev, err := ps.decodeDeviceAnnotations(pod)
	if err != nil {
		return nil, fmt.Errorf("decode device annotations: %w", err)
	}

	// kubelet may call Allocate multiple times for the same pod, each time with
	// a subset of containers. Use pop semantics to match each request with its
	// corresponding containerDevices.
	responses := v1beta1.AllocateResponse{}
	for _, req := range reqs.ContainerRequests {
		containerDevs, err := ps.popNextContainerDevices(podSingleDev)
		if err != nil {
			return nil, fmt.Errorf("get next container devices: %w", err)
		}
		klog.Infof("containerDevs: %+v", containerDevs)

		if len(containerDevs) != len(req.DevicesIDs) {
			return nil, fmt.Errorf("device number not matched: annotation has %d, request has %d", len(containerDevs), len(req.DevicesIDs))
		}

		resp, err := ps.buildContainerAllocateResponse(pod, containerDevs, rtInfoLookup)
		if err != nil {
			return nil, fmt.Errorf("build container allocate response: %w", err)
		}
		responses.ContainerResponses = append(responses.ContainerResponses, resp)
	}

	// Patch the annotation with the in-memory erased podSingleDev.
	if err := ps.patchErasedAnnotation(pod, podSingleDev); err != nil {
		klog.Errorf("erase allocated containers annotation error: %v", err)
		return nil, fmt.Errorf("erase allocated containers annotation: %w", err)
	}

	klog.V(5).Infof("allocate response: %+v", responses.ContainerResponses)
	success = true
	return &responses, nil
}

func (ps *PluginServer) PreStartContainer(context.Context, *v1beta1.PreStartContainerRequest) (*v1beta1.PreStartContainerResponse, error) {
	return &v1beta1.PreStartContainerResponse{}, nil
}
