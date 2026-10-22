package server

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Project-HAMi/HAMi/pkg/device"
	"github.com/Project-HAMi/ascend-device-plugin/internal"
	v1 "k8s.io/api/core/v1"
	"k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

// HamiCoreHostPath is shared by runtime installation, Allocate and monitoring.
func HamiCoreHostPath() string { return hostHookPath }

const HamiCorePolicyAnnotation = "huawei.com/npu-sched-policy"

// Keep the optional capability separate from Manager so other backends and
// manager implementations do not need to provide HAMi core settings.
type hamiCorePolicyManager interface {
	HamiCorePolicy() string
}

func (ps *PluginServer) hamiCorePolicy(pod *v1.Pod) (string, error) {
	value := pod.Annotations[HamiCorePolicyAnnotation]
	if strings.TrimSpace(value) == "" {
		if manager, ok := ps.mgr.(hamiCorePolicyManager); ok {
			value = manager.HamiCorePolicy()
		}
	}
	policy, err := internal.ResolveHamiCorePolicy(value)
	if err != nil {
		return "", fmt.Errorf("%s: %w", HamiCorePolicyAnnotation, err)
	}
	return policy, nil
}

// The core applies its scalar memory and compute settings independently to
// every selected card, so heterogeneous per-card quotas cannot be represented.
func (ps *PluginServer) hamiCoreQuota(devices device.ContainerDevices, lookup map[string]RuntimeInfo) (int64, int32, error) {
	var memoryLimit int64
	var computeLimit int32
	for _, allocation := range devices {
		dev := ps.mgr.GetDeviceByUUID(allocation.UUID)
		if dev == nil {
			return 0, 0, fmt.Errorf("unknown uuid: %s", allocation.UUID)
		}
		info, ok := lookup[allocation.UUID]
		if !ok {
			return 0, 0, fmt.Errorf("missing HAMi runtime allocation for %s", allocation.UUID)
		}
		memory := dev.Memory
		if info.Memory != nil {
			if *info.Memory < 0 {
				return 0, 0, fmt.Errorf("HAMi memory quota for %s must not be negative", allocation.UUID)
			}
			if *info.Memory > 0 {
				memory = *info.Memory
			}
		}
		if memory <= 0 {
			return 0, 0, fmt.Errorf("cannot determine positive HAMi memory quota for %s", allocation.UUID)
		}
		// The runtime converts MiB to a u64 byte count.
		if uint64(memory) > math.MaxUint64/(1024*1024) {
			return 0, 0, fmt.Errorf("HAMi memory quota for %s overflows runtime byte count", allocation.UUID)
		}
		if memoryLimit != 0 && memoryLimit != memory {
			return 0, 0, fmt.Errorf("HAMi core requires the same memory quota on every device: got %d and %d MiB", memoryLimit, memory)
		}
		memoryLimit = memory
		compute := int32(100)
		if info.Core != nil && *info.Core != 0 {
			compute = *info.Core
		}
		if compute < 1 || compute > 100 {
			return 0, 0, fmt.Errorf("HAMi compute quota %d for %s is outside 1..100", compute, allocation.UUID)
		}
		if computeLimit != 0 && computeLimit != compute {
			return 0, 0, fmt.Errorf("HAMi core requires the same compute quota on every device: got %d and %d", computeLimit, compute)
		}
		computeLimit = compute
	}
	if len(devices) == 0 {
		return 0, 0, fmt.Errorf("HAMi core allocation has no devices")
	}
	return memoryLimit, computeLimit, nil
}

func (ps *PluginServer) buildHamiCoreAllocateResponse(resp *v1beta1.ContainerAllocateResponse, pod *v1.Pod, ctrName string, devices device.ContainerDevices, lookup map[string]RuntimeInfo) (*v1beta1.ContainerAllocateResponse, error) {
	policy, err := ps.hamiCorePolicy(pod)
	if err != nil {
		return nil, err
	}
	memory, compute, err := ps.hamiCoreQuota(devices, lookup)
	if err != nil {
		return nil, err
	}
	if !filepath.IsAbs(hostHookPath) {
		return nil, fmt.Errorf("HOOK_PATH must be absolute, got %q", hostHookPath)
	}

	for _, path := range []string{
		"/usr/local/bin/npu-smi", "/etc/ascend_install.info",
		"/usr/local/Ascend/driver/lib64/driver", "/usr/local/Ascend/driver/version.info",
	} {
		resp.Mounts = append(resp.Mounts, &v1beta1.Mount{HostPath: path, ContainerPath: path, ReadOnly: true})
	}
	resp.Mounts = append(resp.Mounts,
		&v1beta1.Mount{HostPath: hostHookPath, ContainerPath: "/hami-vnpu-core", ReadOnly: true},
		&v1beta1.Mount{HostPath: filepath.Join(hostHookPath, "ld.so.preload"), ContainerPath: "/etc/ld.so.preload", ReadOnly: true},
		&v1beta1.Mount{HostPath: "/usr/local/hami-shared-region", ContainerPath: "/hami-shared-region", ReadOnly: false},
	)
	resp.Envs["NPU_MEMORY_LIMIT_MB"] = strconv.FormatInt(memory, 10)
	resp.Envs["NPU_COMPUTE_LIMIT"] = strconv.FormatInt(int64(compute), 10)
	resp.Envs["NPU_SCHED_POLICY"] = policy
	// The core appends the physical device ID; overlapping allocations must use
	// the same base regardless of their selected device order.
	resp.Envs["NPU_GLOBAL_SHM_PATH"] = "/hami-shared-region/global_registry"

	containerShmemDir := filepath.Join(hostHookPath, "containers", fmt.Sprintf("%s_%s", pod.UID, ctrName))
	if err := os.RemoveAll(containerShmemDir); err != nil {
		return nil, fmt.Errorf("remove old HAMi local SHM directory: %w", err)
	}
	if err := os.MkdirAll(containerShmemDir, 0777); err != nil {
		return nil, fmt.Errorf("create HAMi local SHM directory: %w", err)
	}
	if err := os.Chmod(containerShmemDir, 0777); err != nil {
		return nil, fmt.Errorf("chmod HAMi local SHM directory: %w", err)
	}
	resp.Mounts = append(resp.Mounts, &v1beta1.Mount{HostPath: containerShmemDir, ContainerPath: "/hami-vnpu-shmem", ReadOnly: false})
	resp.Envs["NPU_LOCAL_SHM_PATH"] = "/hami-vnpu-shmem/vnpu_local_shmem"
	return resp, nil
}
