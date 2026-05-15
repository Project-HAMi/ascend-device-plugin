package server

import (
	"encoding/json"
	"fmt"
	"strconv"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/Project-HAMi/HAMi/pkg/device"
	"github.com/Project-HAMi/HAMi/pkg/device-plugin/nvidiadevice/nvinternal/plugin"
	"github.com/Project-HAMi/HAMi/pkg/util"
)

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

		// Set GLOBAL_SHM_PATH based on the first device ID.
		resp.Envs["NPU_GLOBAL_SHM_PATH"] = fmt.Sprintf("/hami-shared-region/%d_global_registry", IDs[0])
		klog.V(5).Infof("Create %d_global_registry", IDs[0])
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
