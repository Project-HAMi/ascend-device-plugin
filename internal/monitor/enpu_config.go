package monitor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

type enpuAllocation struct {
	Namespace, PodName, PodUID, ContainerName, ContainerID, DeviceUUID, Policy string
	PhysicalID, VirtualID                                                      int
	CoreQuota, MemoryRequest, MemoryLimit                                      float64
}

type enpuConfigAllocation struct {
	enpuAllocation
	shm string
}

type enpuRuntimeAllocation struct {
	UUID   string `json:"UUID"`
	Temp   string `json:"temp"`
	Memory *int64 `json:"memory"`
	Core   *int64 `json:"core"`
}

var enpuConfigName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)
var enpuContainerHex = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)

// listENPUAllocations keeps independent valid containers when another allocation fails.
func listENPUAllocations(pods []*corev1.Pod, configRoot, managerRoot string) ([]enpuAllocation, error) {
	var candidates []enpuConfigAllocation
	var errs []error
	for _, pod := range pods {
		if pod == nil || (pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodPending) || pod.DeletionTimestamp != nil {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(pod.Annotations["huawei.com/vnpu-mode"])) {
		case "enpu", "ubs-virt", "vcann-rt":
		default:
			continue
		}
		containers := enpuPodContainers(pod)
		statuses := append(append([]corev1.ContainerStatus{}, pod.Status.InitContainerStatuses...), pod.Status.ContainerStatuses...)
		for index, container := range containers {
			resource, err := enpuContainerResource(container)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s/%s/%s: %w", pod.Namespace, pod.Name, container.Name, err))
				continue
			}
			if resource == "" {
				continue
			}
			var status *corev1.ContainerStatus
			for i := range statuses {
				if statuses[i].Name == container.Name {
					if status != nil {
						err = fmt.Errorf("duplicate container status")
						break
					}
					status = &statuses[i]
				}
			}
			if err == nil && (status == nil || status.State.Running == nil) {
				continue
			}
			var allocation enpuConfigAllocation
			if err == nil {
				allocation, err = enpuReadAllocation(pod, index, resource, status.ContainerID, configRoot, managerRoot)
			}
			if err != nil {
				errs = append(errs, fmt.Errorf("%s/%s/%s: %w", pod.Namespace, pod.Name, container.Name, err))
				continue
			}
			candidates = append(candidates, allocation)
		}
	}
	invalid := make(map[int]bool)
	for i, a := range candidates {
		for j := 0; j < i; j++ {
			b := candidates[j]
			if a.ContainerID == b.ContainerID || (a.PodUID == b.PodUID && a.ContainerName == b.ContainerName) ||
				(a.PhysicalID == b.PhysicalID && (a.VirtualID == b.VirtualID || a.DeviceUUID != b.DeviceUUID || a.shm != b.shm)) ||
				(a.PhysicalID != b.PhysicalID && (a.DeviceUUID == b.DeviceUUID || a.shm == b.shm)) {
				invalid[i], invalid[j] = true, true
				errs = append(errs, fmt.Errorf("conflicting ENPU allocations %s/%s and %s/%s", a.PodUID, a.ContainerName, b.PodUID, b.ContainerName))
			}
		}
	}
	var result []enpuAllocation
	for i, a := range candidates {
		if !invalid[i] {
			result = append(result, a.enpuAllocation)
		}
	}
	return result, errors.Join(errs...)
}

func enpuContainerResource(container corev1.Container) (string, error) {
	resource := ""
	for _, resources := range []corev1.ResourceList{container.Resources.Requests, container.Resources.Limits} {
		for name, amount := range resources {
			key := string(name)
			if !strings.HasPrefix(key, "huawei.com/Ascend") || strings.HasSuffix(key, "-core") || strings.HasSuffix(key, "-memory") || amount.Sign() == 0 {
				continue
			}
			count, exact := amount.AsInt64()
			if !exact || count != 1 || (resource != "" && resource != key) {
				return "", fmt.Errorf("ENPU requires exactly one Ascend device resource")
			}
			resource = key
		}
	}
	return resource, nil
}

func enpuPodContainers(pod *corev1.Pod) []corev1.Container {
	return append(append([]corev1.Container{}, pod.Spec.InitContainers...), pod.Spec.Containers...)
}

func enpuReadAllocation(pod *corev1.Pod, index int, resource, containerID, configRoot, managerRoot string) (enpuConfigAllocation, error) {
	a := enpuConfigAllocation{}
	uid, name := string(pod.UID), enpuPodContainers(pod)[index].Name
	if pod.Namespace == "" || pod.Name == "" || !enpuConfigName.MatchString(uid) || strings.Contains(uid, "_") || !enpuConfigName.MatchString(name) {
		return a, fmt.Errorf("invalid Pod/container identity")
	}
	runtime, id, ok := strings.Cut(containerID, "://")
	if !ok || (runtime != "containerd" && runtime != "docker" && runtime != "cri-o" && runtime != "crio") || !enpuContainerHex.MatchString(id) {
		return a, fmt.Errorf("invalid runtime container ID")
	}
	root, relative := configRoot, filepath.Join(uid+"_"+name, "npu_info.config")
	if managerRoot != "" {
		root = filepath.Clean(managerRoot)
		if filepath.Base(root) != "vcann-rt" {
			root = filepath.Join(root, "vcann-rt")
		}
		relative = filepath.Join(uid, name, "npu_info.config")
	}
	if root == "" {
		return a, fmt.Errorf("empty ENPU config root")
	}
	values, err := enpuReadConfig(root, relative)
	if err != nil {
		return a, err
	}
	physical, err := enpuConfigUint(values["physical-npu-id"], 0, 15)
	if err != nil {
		return a, fmt.Errorf("physical-npu-id: %w", err)
	}
	virtualMax := uint64(99)
	if managerRoot != "" {
		virtualMax = 127
	}
	virtual, err := enpuConfigUint(values["virtual-npu-id"], 0, virtualMax)
	if err != nil {
		return a, fmt.Errorf("virtual-npu-id: %w", err)
	}
	core, err := enpuConfigUint(values["aicore-quota"], 1, 100)
	if err != nil {
		return a, fmt.Errorf("aicore-quota: %w", err)
	}
	policyCode, err := enpuConfigUint(values["scheduling-policy"], 1, 3)
	if err != nil {
		return a, fmt.Errorf("scheduling-policy: %w", err)
	}
	policy, _ := enpuPolicyName(strconv.FormatUint(policyCode, 10))
	shm := values["shm-id"]
	if shm == "" || len(shm) > 127 || strings.Contains(shm, "/") || strings.Contains(shm, "..") || strings.ContainsAny(shm, "\x00\r\n\t ") {
		return a, fmt.Errorf("invalid shm-id")
	}
	request, limit := values["memory-request"], values["memory-limit"]
	if request == "" && limit == "" {
		request, limit = values["memory-quota"], values["memory-quota"]
	}
	requestMiB, err := enpuConfigUint(request, 1, ^uint64(0)>>20)
	if err != nil {
		return a, fmt.Errorf("memory-request: %w", err)
	}
	limitMiB, err := enpuConfigUint(limit, requestMiB, ^uint64(0)>>20)
	if err != nil {
		return a, fmt.Errorf("memory-limit: %w", err)
	}
	if raw, exists := values["memory-quota"]; exists {
		quota, quotaErr := enpuConfigUint(raw, 1, ^uint64(0)>>20)
		if quotaErr != nil || quota != requestMiB {
			return a, fmt.Errorf("memory-quota disagrees with request")
		}
	}
	if policy == "fixed-share" && requestMiB != limitMiB {
		return a, fmt.Errorf("fixed-share memory request and limit differ")
	}
	device, err := enpuAnnotatedDevice(pod, index, resource)
	if err != nil {
		return a, err
	}
	if device.Memory != nil && (*device.Memory <= 0 || uint64(*device.Memory) != requestMiB) {
		return a, fmt.Errorf("device annotation memory disagrees with config")
	}
	if device.Core != nil {
		configured := *device.Core
		if configured == 0 {
			configured = 100
		}
		if configured < 1 || uint64(configured) != core {
			return a, fmt.Errorf("device annotation core disagrees with config")
		}
	}
	if managerRoot == "" {
		expected := "hami-enpu-" + device.UUID
		if len(expected) > 120 {
			expected = expected[:120]
		}
		if shm != expected {
			return a, fmt.Errorf("shm-id disagrees with allocated UUID")
		}
	} else if shm != device.UUID {
		return a, fmt.Errorf("manager shm-id disagrees with allocated UUID")
	}
	for key, want := range map[string]uint64{"huawei.com/enpu-memory-request": requestMiB, "huawei.com/enpu-memory-limit": limitMiB} {
		raw := strings.TrimSpace(pod.Annotations[key])
		if raw == "" {
			if key == "huawei.com/enpu-memory-limit" && limitMiB != requestMiB {
				return a, fmt.Errorf("memory limit annotation missing")
			}
			continue
		}
		value, parseErr := enpuConfigUint(raw, 1, ^uint64(0)>>20)
		if parseErr != nil || value != want {
			return a, fmt.Errorf("%s disagrees with config", key)
		}
	}
	policyAnnotation := pod.Annotations["huawei.com/enpu-policy"]
	if policyAnnotation == "" {
		policyAnnotation = pod.Annotations["huawei.com/scheduler.softShareDev.policy"]
	}
	if policyAnnotation == "" {
		policyAnnotation = pod.Labels["huawei.com/scheduler.softShareDev.policy"]
	}
	if policyAnnotation != "" {
		value, parseErr := enpuPolicyName(policyAnnotation)
		if parseErr != nil || value != policy {
			return a, fmt.Errorf("policy annotation disagrees with config")
		}
	}
	a.enpuAllocation = enpuAllocation{Namespace: pod.Namespace, PodName: pod.Name, PodUID: uid, ContainerName: name, ContainerID: strings.ToLower(id), DeviceUUID: device.UUID, PhysicalID: int(physical), VirtualID: int(virtual), CoreQuota: float64(core), MemoryRequest: float64(requestMiB << 20), MemoryLimit: float64(limitMiB << 20), Policy: policy}
	a.shm = shm
	return a, nil
}

func enpuReadConfig(root, relative string) (map[string]string, error) {
	directory, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("open ENPU config root: %w", err)
	}
	defer func() {
		_ = directory.Close()
	}()
	for partial := relative; partial != "."; partial = filepath.Dir(partial) {
		info, statErr := directory.Lstat(partial)
		if statErr != nil {
			return nil, fmt.Errorf("stat ENPU config: %w", statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("ENPU config path contains symlink")
		}
	}
	file, err := directory.Open(relative)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("ENPU config is not a regular file")
	}
	data, err := io.ReadAll(io.LimitReader(file, 65537))
	if err != nil {
		return nil, err
	}
	if len(data) > 65536 {
		return nil, fmt.Errorf("ENPU config exceeds 64 KiB")
	}
	values := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if _, duplicate := values[key]; !ok || key == "" || value == "" || duplicate || strings.ContainsRune(line, '\x00') {
			return nil, fmt.Errorf("invalid or duplicate ENPU config entry")
		}
		values[key] = value
	}
	return values, nil
}

func enpuConfigUint(raw string, min, max uint64) (uint64, error) {
	if raw == "" || strings.IndexFunc(raw, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
		return 0, fmt.Errorf("expected decimal integer")
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || value < min || value > max {
		return 0, fmt.Errorf("integer outside %d..%d", min, max)
	}
	return value, nil
}

func enpuPolicyName(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "fixed-share", "fixed_share", "fixed":
		return "fixed-share", nil
	case "2", "elastic":
		return "elastic", nil
	case "3", "best-effort", "best_effort", "besteffort":
		return "best-effort", nil
	default:
		return "", fmt.Errorf("unknown ENPU policy")
	}
}

func enpuAnnotatedDevice(pod *corev1.Pod, index int, resource string) (enpuRuntimeAllocation, error) {
	var devices []enpuRuntimeAllocation
	if err := json.Unmarshal([]byte(pod.Annotations[resource]), &devices); err != nil {
		return enpuRuntimeAllocation{}, fmt.Errorf("invalid %s annotation: %w", resource, err)
	}
	byUUID := map[string]enpuRuntimeAllocation{}
	for _, device := range devices {
		if !enpuConfigName.MatchString(device.UUID) || device.Temp != "" {
			return enpuRuntimeAllocation{}, fmt.Errorf("invalid runtime allocation identity")
		}
		if previous, duplicate := byUUID[device.UUID]; duplicate && !reflect.DeepEqual(previous, device) {
			return enpuRuntimeAllocation{}, fmt.Errorf("conflicting runtime allocation UUID")
		}
		byUUID[device.UUID] = device
	}
	deviceType := strings.TrimPrefix(resource, "huawei.com/")
	allocated, exists := pod.Annotations["hami.io/"+deviceType+"-devices-allocated"]
	if !exists {
		count := 0
		for _, container := range enpuPodContainers(pod) {
			candidate, err := enpuContainerResource(container)
			if err != nil {
				return enpuRuntimeAllocation{}, err
			}
			if candidate == resource {
				count++
			}
		}
		if count == 1 && len(byUUID) == 1 {
			return devices[0], nil
		}
		return enpuRuntimeAllocation{}, fmt.Errorf("ambiguous container device annotation")
	}
	parts := strings.Split(allocated, ";")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	if len(parts) != len(enpuPodContainers(pod)) {
		return enpuRuntimeAllocation{}, fmt.Errorf("allocated annotation container count mismatch")
	}
	entry := strings.TrimSuffix(parts[index], ":")
	fields := strings.Split(entry, ",")
	if (len(fields) != 4 && len(fields) != 5) || fields[1] != deviceType || strings.Contains(entry, ":") || (len(fields) == 5 && fields[4] != "1") {
		return enpuRuntimeAllocation{}, fmt.Errorf("invalid single-device allocation annotation")
	}
	device, found := byUUID[fields[0]]
	if !found {
		return enpuRuntimeAllocation{}, fmt.Errorf("allocated UUID missing from runtime annotation")
	}
	memory, err := enpuConfigUint(fields[2], 1, 2147483647)
	if err != nil {
		return enpuRuntimeAllocation{}, fmt.Errorf("allocated memory: %w", err)
	}
	core, err := enpuConfigUint(fields[3], 0, 100)
	if err != nil {
		return enpuRuntimeAllocation{}, fmt.Errorf("allocated core: %w", err)
	}
	if core == 0 {
		core = 100
	}
	if device.Memory != nil && (*device.Memory <= 0 || uint64(*device.Memory) != memory) {
		return enpuRuntimeAllocation{}, fmt.Errorf("conflicting memory annotations")
	}
	if device.Core != nil {
		value := *device.Core
		if value == 0 {
			value = 100
		}
		if value < 1 || uint64(value) != core {
			return enpuRuntimeAllocation{}, fmt.Errorf("conflicting core annotations")
		}
	}
	memValue, coreValue := int64(memory), int64(core)
	device.Memory, device.Core = &memValue, &coreValue
	return device, nil
}
