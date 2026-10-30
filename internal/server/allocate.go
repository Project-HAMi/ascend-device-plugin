package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/Project-HAMi/HAMi/pkg/device"
	"github.com/Project-HAMi/HAMi/pkg/device-plugin/nvidiadevice/nvinternal/plugin"
	"github.com/Project-HAMi/HAMi/pkg/util"
	"github.com/Project-HAMi/ascend-device-plugin/internal"
)

var hostHookPath string

func init() {
	hostHookPath = os.Getenv("HOOK_PATH")
	if hostHookPath == "" {
		hostHookPath = "/usr/local/hami-vnpu-core"
	}
}

func roHostMount(path string) *v1beta1.Mount {
	return &v1beta1.Mount{HostPath: path, ContainerPath: path, ReadOnly: true}
}

func hostDirExists(path string) bool {
	candidates := []string{path}
	const ascendRoot = "/usr/local/Ascend/"
	if strings.HasPrefix(path, ascendRoot) {
		// The device plugin sees the host Ascend tree at /host/ascend.
		candidates = append([]string{"/host/ascend/" + strings.TrimPrefix(path, ascendRoot)}, candidates...)
	}
	for _, candidate := range candidates {
		st, err := os.Stat(candidate)
		if err == nil && st.IsDir() {
			return true
		}
	}
	return false
}

// hamiCoreHostMounts mounts the driver files the workload needs.
// npu-smi on Ascend 950 is /usr/local/sbin/npu-smi; a missing host path
// fails the pod, so only an existing candidate is used.
// CANN 9 on this machine lives under /usr/local/Ascend/cann, not
// ascend-toolkit/latest. Those directories are mounted only when present.
func hamiCoreHostMounts() []*v1beta1.Mount {
	smi := "/usr/local/bin/npu-smi"
	if found, err := resolveNpuSmi(); err == nil && found != "" {
		smi = found
	}
	mounts := []*v1beta1.Mount{
		roHostMount(smi),
		roHostMount("/etc/ascend_install.info"),
		roHostMount("/usr/local/Ascend/driver/lib64/driver"),
		roHostMount("/usr/local/Ascend/driver/version.info"),
	}
	for _, path := range []string{
		"/usr/local/Ascend/driver/lib64/common",
		"/usr/local/Ascend/cann",
		"/usr/local/Ascend/ascend-toolkit/latest",
	} {
		if hostDirExists(path) {
			mounts = append(mounts, roHostMount(path))
		}
	}
	return mounts
}

func ascendDockerRuntimePresent() bool {
	for _, path := range []string{
		"/usr/local/Ascend/Ascend-Docker-Runtime/ascend-docker-runtime",
		"/host/ascend/Ascend-Docker-Runtime/ascend-docker-runtime",
	} {
		st, err := os.Stat(path)
		if err == nil && !st.IsDir() {
			return true
		}
	}
	return false
}

func ascend950DeviceSpecs(phyID int32) []*v1beta1.DeviceSpec {
	var specs []*v1beta1.DeviceSpec
	add := func(path string) {
		specs = append(specs, &v1beta1.DeviceSpec{
			ContainerPath: path,
			HostPath:      path,
			Permissions:   "rwm",
		})
	}
	add(fmt.Sprintf("/dev/davinci%d", phyID))
	add("/dev/davinci_manager")
	for _, dir := range []string{"/dev/uburma", "/dev/ummu"} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			add(filepath.Join(dir, entry.Name()))
		}
	}
	return specs
}

func ascendRuntimeLibPath() string {
	var dirs []string
	for _, path := range []string{
		"/usr/local/Ascend/cann/lib64",
		"/usr/local/Ascend/cann/aarch64-linux/lib64",
		"/usr/local/Ascend/ascend-toolkit/latest/lib64",
		"/usr/local/Ascend/ascend-toolkit/latest/aarch64-linux/lib64",
		"/usr/local/Ascend/driver/lib64/driver",
		"/usr/local/Ascend/driver/lib64/common",
	} {
		if hostDirExists(path) {
			dirs = append(dirs, path)
		}
	}
	return strings.Join(dirs, ":")
}

// buildContainerAllocateResponse builds the allocate response for a single container.
func (ps *PluginServer) buildContainerAllocateResponse(pod *v1.Pod, ctrName string, containerDevs device.ContainerDevices, rtInfoLookup map[string]RuntimeInfo) (*v1beta1.ContainerAllocateResponse, error) {
	resp := &v1beta1.ContainerAllocateResponse{}
	is950 := internal.ChipFamily(ps.mgr.CommonWord()) != ""
	if is950 {
		if pod.Annotations[VNPUModeAnnotation] != VNPUModeHamiCore || !ps.mgr.IsHamiVnpuCore() || len(containerDevs) != 1 {
			return nil, fmt.Errorf("experimental 950 requires explicit hami-core on a hami-core node and exactly one device per container")
		}
		info := rtInfoLookup[containerDevs[0].UUID]
		if info.Memory == nil || *info.Memory <= 0 || info.Core == nil || *info.Core <= 0 || *info.Core > 100 {
			return nil, fmt.Errorf("experimental 950 requires positive memory and core quota (1-100)")
		}
	}

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
	effectiveHamiCore := vnpuMode == VNPUModeHamiCore || (vnpuMode == "" && ps.mgr.IsHamiVnpuCore())
	effectiveENPU := podUsesENPU(pod)
	klog.V(4).Infof("Pod %s vnpu mode: %q, effectiveHamiCore: %v, effectiveENPU: %v", pod.Name, vnpuMode, effectiveHamiCore, effectiveENPU)
	if effectiveENPU {
		if len(containerDevs) != 1 {
			return nil, fmt.Errorf("ENPU mode supports exactly one physical NPU per container, got %d", len(containerDevs))
		}
		dev := ps.mgr.GetDeviceByUUID(containerDevs[0].UUID)
		if dev == nil {
			return nil, fmt.Errorf("unknown uuid: %s", containerDevs[0].UUID)
		}
		if err := ps.ensureENPUSingleDieMode(); err != nil {
			return nil, err
		}
		policy, err := enpuPolicy(pod, managerENPUPolicy(ps.mgr))
		if err != nil {
			return nil, err
		}
		info := rtInfoLookup[containerDevs[0].UUID]
		memory, core, err := enpuQuota(info, dev)
		if err != nil {
			return nil, err
		}
		request, limit, err := enpuMemoryRequestLimit(pod, memory, policy)
		if err != nil {
			return nil, err
		}
		configPath := ""
		if enpuManagerEndpoint() != "" {
			allocation, managerErr := allocateENPUManager(pod, ctrName, dev, request, limit, core, policy)
			if managerErr != nil {
				return nil, managerErr
			}
			configPath = managerConfigPath(allocation)
		} else {
			configPath, err = ps.writeENPUConfig(pod, ctrName, dev, info, policy)
			if err != nil {
				return nil, err
			}
		}
		resp.Mounts = enpuMounts(configPath)
		resp.Devices = enpuDeviceSpecs(ps.mgr.GetDevices())
		return resp, nil
	}
	if effectiveHamiCore {
		if is950 {
			d := ps.mgr.GetDeviceByUUID(containerDevs[0].UUID)
			if d.LogicID < 0 || *memories[0] > d.Memory {
				return nil, fmt.Errorf("invalid 950 device mapping or memory quota exceeds device capacity")
			}
			// Ascend 950 uses DCMI v2, whose process-memory API is addressed by logic ID.
			resp.Envs["NPU_DCMI_DEVICE"] = fmt.Sprintf("v2:%d", d.LogicID)
			resp.Envs["NPU_FAIL_CLOSED"] = "1"
		}
		// 1. Handle volume mount injection
		mounts := hamiCoreHostMounts()

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
		if is950 {
			// 950 core quotas are fixed time shares, not weighted best-effort tokens.
			resp.Envs["NPU_FIXED_SHARE_RATIO"] = "1"
			// The Ascend OCI runtime injects these nodes itself. Without that
			// runtime, kubelet still injects the paths returned here.
			if !ascendDockerRuntimePresent() {
				resp.Devices = ascend950DeviceSpecs(IDs[0])
			}
		}
		if libPath := ascendRuntimeLibPath(); libPath != "" {
			resp.Envs["LD_LIBRARY_PATH"] = libPath
		}

		// Set GLOBAL_SHM_PATH based on the first device ID.
		resp.Envs["NPU_GLOBAL_SHM_PATH"] = fmt.Sprintf("/hami-shared-region/%d_global_registry", IDs[0])
		klog.V(5).Infof("Create %d_global_registry", IDs[0])

		// Per-container local shmem dir (like NVIDIA vgpu/containers/{podUID}_{ctrName})
		containerShmemDir := fmt.Sprintf("%s/containers/%s_%s", hostHookPath, pod.UID, ctrName)
		_ = os.RemoveAll(containerShmemDir)
		if err := os.MkdirAll(containerShmemDir, 0777); err != nil {
			klog.Warningf("MkdirAll %s: %v", containerShmemDir, err)
		}
		if err := os.Chmod(containerShmemDir, 0777); err != nil {
			klog.Warningf("Chmod %s: %v", containerShmemDir, err)
		}
		resp.Mounts = append(resp.Mounts, &v1beta1.Mount{
			HostPath:      containerShmemDir,
			ContainerPath: "/hami-vnpu-shmem",
			ReadOnly:      false,
		})
		resp.Envs["NPU_LOCAL_SHM_PATH"] = "/hami-vnpu-shmem/vnpu_local_shmem"
		klog.V(4).Infof("Local shmem for %s/%s: host=%s", pod.UID, ctrName, containerShmemDir)
	} else {
		if ascendVNPUSpec != "" {
			resp.Envs["ASCEND_VNPU_SPECS"] = ascendVNPUSpec
		}
	}
	return resp, nil
}

// popNextContainerDevices finds and erases the first non-empty containerDevices
// from podSingleDev and returns the corresponding container name.
// The annotation order is: init containers first, then regular containers.
func (ps *PluginServer) popNextContainerDevices(pod *v1.Pod, podSingleDev device.PodSingleDevice) (device.ContainerDevices, string, error) {
	initCount := len(pod.Spec.InitContainers)
	for i, ctrDevs := range podSingleDev {
		if len(ctrDevs) > 0 {
			podSingleDev[i] = device.ContainerDevices{}
			var ctrName string
			if i < initCount {
				ctrName = pod.Spec.InitContainers[i].Name
			} else {
				regularIdx := i - initCount
				if regularIdx < len(pod.Spec.Containers) {
					ctrName = pod.Spec.Containers[regularIdx].Name
				}
			}
			return ctrDevs, ctrName, nil
		}
	}
	return nil, "", fmt.Errorf("no pending device allocation found")
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
