package server

import (
	"encoding/json"
	"fmt"
	"os"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/Project-HAMi/HAMi/pkg/device"
	"github.com/Project-HAMi/HAMi/pkg/device-plugin/nvidiadevice/nvinternal/plugin"
	"github.com/Project-HAMi/HAMi/pkg/util"
)

var hostHookPath string

func init() {
	hostHookPath = os.Getenv("HOOK_PATH")
	if hostHookPath == "" {
		hostHookPath = "/usr/local/hami-vnpu-core"
	}
}

// buildContainerAllocateResponse builds the allocate response for a single container.
func (ps *PluginServer) buildContainerAllocateResponse(pod *v1.Pod, ctrName string, containerDevs device.ContainerDevices, rtInfoLookup map[string]RuntimeInfo) (*v1beta1.ContainerAllocateResponse, error) {
	resp := &v1beta1.ContainerAllocateResponse{}

	var (
		IDs            []int32
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
		return ps.buildHamiCoreAllocateResponse(resp, pod, ctrName, containerDevs, rtInfoLookup)
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
