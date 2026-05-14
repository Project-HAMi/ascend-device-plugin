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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	// "github.com/Project-HAMi/HAMi/pkg/device/ascend"
	"github.com/Project-HAMi/HAMi/pkg/device"
	"github.com/Project-HAMi/HAMi/pkg/device-plugin/nvidiadevice/nvinternal/plugin"
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
	mgr                   *manager.AscendManager
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

func NewPluginServer(mgr *manager.AscendManager, nodeName string, checkIdleVNPUInterval int) (*PluginServer, error) {
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

// fileSHA256 calculates the SHA256 checksum of the specified file
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Automatically creates directories, sets permissions, and copies core files on the host
func prepareHostResources() error {
	klog.Info("Starting host resource preparation for HAMi vNPU core...")

	// 1. Create shared memory directory
	sharedRegionPath := "/usr/local/hami-shared-region"
	if err := os.MkdirAll(sharedRegionPath, 0777); err != nil {
		if !os.IsExist(err) {
			return fmt.Errorf("failed to create %s: %w", sharedRegionPath, err)
		}
	}
	if err := os.Chmod(sharedRegionPath, 0777); err != nil {
		return fmt.Errorf("failed to chmod %s: %w", sharedRegionPath, err)
	}
	klog.Infof("Successfully prepared directory: %s", sharedRegionPath)

	// 2. Prepare /usr/local/hami-vnpu-core/ directory
	targetDir := "/usr/local/hami-vnpu-core"
	if err := os.MkdirAll(targetDir, 0775); err != nil {
		return fmt.Errorf("failed to create %s: %w", targetDir, err)
	}

	// Specify the in-container assets directory (can be overridden via environment variable, default follows standard DevicePlugin convention)
	assetsDir := os.Getenv("HAMI_VNPU_ASSETS_PATH")
	if assetsDir == "" {
		assetsDir = "/usr/local/hami-vnpu-core-assets"
	}

	// Define files to copy: source path in container -> target path on host
	filesToCopy := map[string]string{
		"limiter":       filepath.Join(targetDir, "limiter"),
		"libvnpu.so":    filepath.Join(targetDir, "libvnpu.so"),
		"ld.so.preload": filepath.Join(targetDir, "ld.so.preload"),
	}

	for srcName, destPath := range filesToCopy {
		srcPath := filepath.Join(assetsDir, srcName)

		// File already exists, skip if content is consistent
		if _, err := os.Stat(destPath); err == nil {
			srcSum, err1 := fileSHA256(srcPath)
			dstSum, err2 := fileSHA256(destPath)

			if err1 == nil && err2 == nil && srcSum == dstSum {
				klog.Infof("✓ %s already up-to-date, skipping", destPath)
				continue
			}
		}

		if err := copyFile(srcPath, destPath); err != nil {
			if strings.Contains(err.Error(), "text file busy") {
				klog.Warningf("⚠ %s is in use by running process, keeping existing version (safe)", destPath)
				continue
			}
			return fmt.Errorf("failed to copy %s: %w", destPath, err)
		}
		klog.Infof("✓ Copied %s -> %s", srcPath, destPath)
	}

	klog.Info("Host resource preparation completed successfully.")
	return nil
}

// A standard file copy implementation that preserves the original file permissions
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err = io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	// Sync source file permissions (ensure the limiter binary retains executable permission)
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}
	return os.Chmod(dst, srcInfo.Mode())
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

func (ps *PluginServer) dial(unixSocketPath string, timeout time.Duration) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	c, err := grpc.DialContext(ctx, unixSocketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithContextDialer(func(ctx2 context.Context, addr string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx2, "unix", addr)
		}),
	)

	if err != nil {
		return nil, err
	}
	return c, nil
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
		return err
	}
	_ = conn.Close()

	return nil
}

func (ps *PluginServer) registerKubelet() error {
	conn, err := ps.dial(v1beta1.KubeletSocket, 5*time.Second)
	if err != nil {
		return err
	}
	defer func(conn *grpc.ClientConn) {
		_ = conn.Close()
	}(conn)
	client := v1beta1.NewRegistrationClient(conn)
	reqt := &v1beta1.RegisterRequest{
		Version:      v1beta1.Version,
		Endpoint:     path.Base(ps.socket),
		ResourceName: ps.mgr.ResourceName(),
		Options: &v1beta1.DevicePluginOptions{
			GetPreferredAllocationAvailable: false,
		},
	}

	_, err = client.Register(context.Background(), reqt)
	if err != nil {
		return err
	}
	return nil
}

func (ps *PluginServer) getDeviceNetworkID(idx int, deviceType string) (int, error) {
	// For Ascend910C devices, all modules (dies) are interconnected via HCCS
	if deviceType == Ascend910CType {
		return 0, nil
	}

	if idx > 3 {
		return 1, nil
	}

	return 0, nil
}

func (ps *PluginServer) registerHAMi() error {
	devs := ps.mgr.GetDevices()
	apiDevices := make([]*device.DeviceInfo, 0, len(devs))
	// hami currently believes that the index starts from 0 and is continuous.
	for i, dev := range devs {
		device := &device.DeviceInfo{
			Index:   uint(i),
			ID:      dev.UUID,
			Count:   int32(ps.mgr.VDeviceCount()),
			Devmem:  int32(dev.Memory),
			Devcore: dev.AICore,
			Type:    ps.mgr.CommonWord(),
			Numa:    0,
			Health:  dev.Health,
		}
		if strings.HasPrefix(device.Type, Ascend910Prefix) {
			NetworkID, err := ps.getDeviceNetworkID(i, device.Type)
			if err != nil {
				return fmt.Errorf("get networkID error: %w", err)
			}
			device.CustomInfo = map[string]any{
				"NetworkID": NetworkID,
			}
		}
		apiDevices = append(apiDevices, device)
	}
	annos := make(map[string]string)
	annos[ps.registerAnno] = device.MarshalNodeDevices(apiDevices)
	annos[ps.handshakeAnno] = "Reported_" + time.Now().Add(time.Duration(*reportTimeOffset)*time.Second).Format("2006.01.02 15:04:05")

	if ps.mgr.IsHamiVnpuCore() {
		annos[VNPUNodeSelectorAnnotation] = "true"
		klog.V(4).Infof("Node %s has HamiVnpuCore enabled, patching annotation %s: true", ps.nodeName, VNPUNodeSelectorAnnotation)
	} else {
		annos[VNPUNodeSelectorAnnotation] = "false"
	}

	node, err := util.GetNode(ps.nodeName)
	if err != nil {
		return fmt.Errorf("get node %s error: %w", ps.nodeName, err)
	}
	err = util.PatchNodeAnnotations(node, annos)
	if err != nil {
		return fmt.Errorf("patch node %s annotations error: %w", ps.nodeName, err)
	}
	klog.V(5).Infof("patch node %s annotations: %v", ps.nodeName, annos)
	return nil
}

func (ps *PluginServer) watchAndRegister() {
	timer := time.After(1 * time.Second)
	for {
		select {
		case <-ps.stopCh:
			klog.Infof("stop watch and register")
			return
		case <-timer:
		}
		unhealthy := ps.mgr.GetUnHealthIDs()
		if len(unhealthy) > 0 {
			if err := ps.mgr.UpdateDevice(); err != nil {
				klog.Errorf("update device error: %v", err)
				timer = time.After(5 * time.Second)
				continue
			}
			ps.healthCh <- unhealthy[0]
		}
		err := ps.registerHAMi()
		if err != nil {
			klog.Errorf("register HAMi error: %v", err)
			timer = time.After(5 * time.Second)
		} else {
			klog.V(3).Infof("register HAMi success")
			timer = time.After(30 * time.Second)
		}
	}
}

// buildContainerAllocateResponse builds the allocate response for a single container.
func (ps *PluginServer) buildContainerAllocateResponse(pod *v1.Pod, containerDevs device.ContainerDevices, rtInfoLookup map[string]RuntimeInfo) (*v1beta1.ContainerAllocateResponse, error) {
	resp := &v1beta1.ContainerAllocateResponse{}

	var (
		IDs            []int32
		memories       []*int64
		cores          []*int32
		ascendVNPUSpec string
	)

	for _, dev := range containerDevs {
		d := ps.mgr.GetDeviceByUUID(dev.UUID)
		if d == nil {
			return nil, fmt.Errorf("unknown uuid: %s", dev.UUID)
		}
		IDs = append(IDs, d.PhyID)

		if info, ok := rtInfoLookup[dev.UUID]; ok {
			if ascendVNPUSpec == "" && info.Temp != "" {
				ascendVNPUSpec = info.Temp
			}
			if info.Memory != nil {
				memories = append(memories, info.Memory)
			}
			if info.Core != nil {
				cores = append(cores, info.Core)
			}
		}
	}

	if len(IDs) == 0 {
		return nil, fmt.Errorf("annotation %s value invalid", ps.allocAnno)
	}
	ascendVisibleDevices := fmt.Sprintf("%d", IDs[0])
	for i := 1; i < len(IDs); i++ {
		ascendVisibleDevices = fmt.Sprintf("%s,%d", ascendVisibleDevices, IDs[i])
	}
	resp.Envs = make(map[string]string)
	resp.Envs["ASCEND_VISIBLE_DEVICES"] = ascendVisibleDevices

	vnpuMode := pod.Annotations[VNPUModeAnnotation]
	klog.V(4).Infof("Pod %s vnpu mode: %s", pod.Name, vnpuMode)
	if vnpuMode == VNPUModeHamiCore {
		// 1. Handle volume mount injection
		var mounts []*v1beta1.Mount
		// A.Huawei driver and SMI toolchain (Read-Only)
		driverPaths := []string{
			"/usr/local/bin/npu-smi",
			"/etc/ascend_install.info",
			"/usr/local/Ascend/driver/lib64/driver",
			"/usr/local/Ascend/driver/version.info",
		}
		for _, p := range driverPaths {
			mounts = append(mounts, &v1beta1.Mount{HostPath: p, ContainerPath: p, ReadOnly: true})
		}

		mounts = append(mounts, &v1beta1.Mount{
			HostPath:      "/usr/local/hami-vnpu-core",
			ContainerPath: "/hami-vnpu-core",
			ReadOnly:      true,
		})
		// B. Inject HAMi library path by mounting /etc/ld.so.preload.
		mounts = append(mounts, &v1beta1.Mount{
			HostPath:      "/usr/local/hami-vnpu-core/ld.so.preload", // Template file on host
			ContainerPath: "/etc/ld.so.preload",                      // Overwrites the target file in container
			ReadOnly:      true,
		})

		// C. Shared directory for HAMi compute resource partitioning (Read/Write)
		mounts = append(mounts, &v1beta1.Mount{
			HostPath:      "/usr/local/hami-shared-region",
			ContainerPath: "/hami-shared-region",
			ReadOnly:      false,
		})
		resp.Mounts = mounts

		// Set NPU_MEM_QUOTA
		if len(memories) > 0 && memories[0] != nil {
			resp.Envs["NPU_MEM_QUOTA"] = strconv.FormatInt(*memories[0], 10)
			klog.V(4).InfoS("Memory quota set", "value", *memories[0])
		}

		// Set NPU_PRIORITY
		if len(cores) > 0 && cores[0] != nil {
			resp.Envs["NPU_PRIORITY"] = strconv.FormatInt(int64(*cores[0]), 10)
			klog.V(4).InfoS("Core priority set", "value", *cores[0])
		}

		// Set GLOBAL_SHM_PATH separated by device ID.
		if len(IDs) > 0 {
			resp.Envs["NPU_GLOBAL_SHM_PATH"] = fmt.Sprintf("/hami-shared-region/%d_global_registry", IDs[0])
			klog.V(5).Infof("Create %d_global_registry", IDs[0])
		} else {
			klog.Warningf("No device IDs allocated")
		}
	} else {
		if ascendVNPUSpec != "" {
			resp.Envs["ASCEND_VNPU_SPECS"] = ascendVNPUSpec
		}
	}
	return resp, nil
}

// popNextContainerDevices finds and erases the first non-empty containerDevices
// from podSingleDev. It mutates podSingleDev in place.
func (ps *PluginServer) popNextContainerDevices(podSingleDev device.PodSingleDevice) (device.ContainerDevices, error) {
	for i, ctrDevs := range podSingleDev {
		if len(ctrDevs) > 0 {
			podSingleDev[i] = device.ContainerDevices{}
			return ctrDevs, nil
		}
	}
	return nil, fmt.Errorf("no pending device allocation found")
}

// decodeDeviceAnnotations decodes the pod's device allocation annotation
// (registered as hami.io/<commonword>-devices-to-allocate in InRequestDevices)
// into a PodSingleDevice.
func (ps *PluginServer) decodeDeviceAnnotations(pod *v1.Pod) (device.PodSingleDevice, error) {
	pdevices, err := device.DecodePodDevices(device.InRequestDevices, pod.Annotations)
	if err != nil {
		return nil, err
	}
	pd, ok := pdevices[ps.commonWord]
	if !ok {
		return nil, fmt.Errorf("device %s not found in pod annotations", ps.commonWord)
	}
	return pd, nil
}

// buildRuntimeInfoLookup builds a UUID-to-RuntimeInfo lookup from the pod's allocAnno annotation.
func (ps *PluginServer) buildRuntimeInfoLookup(pod *v1.Pod) (map[string]RuntimeInfo, error) {
	anno, ok := pod.Annotations[ps.allocAnno]
	if !ok {
		return nil, fmt.Errorf("annotation %s not set", ps.allocAnno)
	}
	var rtInfo []RuntimeInfo
	if err := json.Unmarshal([]byte(anno), &rtInfo); err != nil {
		return nil, fmt.Errorf("annotation %s value %s invalid: %w", ps.allocAnno, anno, err)
	}
	lookup := make(map[string]RuntimeInfo, len(rtInfo))
	for _, info := range rtInfo {
		if info.UUID != "" {
			lookup[info.UUID] = info
		}
	}
	return lookup, nil
}

// patchErasedAnnotation patches the pod's device annotation with the given
// podSingleDev. It also updates pod.Annotations in place.
func (ps *PluginServer) patchErasedAnnotation(pod *v1.Pod, podSingleDev device.PodSingleDevice) error {
	klog.V(5).Infof("After erase annotation, remaining devices: %v", podSingleDev)
	newAnnoValue := device.EncodePodSingleDevice(podSingleDev)
	newAnnos := map[string]string{
		ps.toAllocDeviceAnno: newAnnoValue,
	}
	if err := util.PatchPodAnnotations(pod, newAnnos); err != nil {
		return err
	}
	pod.Annotations[ps.toAllocDeviceAnno] = newAnnoValue
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

// podAllocationTrySuccess checks if all containers of this pod have been
// allocated. If so, it sets bind-phase to "success" and releases the node
// lock; otherwise it returns without setting bind-phase or releasing the lock,
// waiting for the next Allocate call.
func (ps *PluginServer) podAllocationTrySuccess(pod *v1.Pod) {
	plugin.PodAllocationTrySuccess(ps.nodeName, ps.commonWord, NodeLockAscend, pod)
}

// podAllocationFailed sets bind-phase to "failed" and releases the node lock.
func (ps *PluginServer) podAllocationFailed(pod *v1.Pod) {
	plugin.PodAllocationFailed(ps.nodeName, pod, NodeLockAscend)
}
