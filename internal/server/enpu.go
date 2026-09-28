/*
 * Copyright 2026 The HAMi Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 */

package server

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/Project-HAMi/ascend-device-plugin/internal/manager"
)

const (
	defaultENPURuntimePath           = "/usr/local/enpu/vcann-rt/lib/libvruntime.so"
	defaultENPUMonitorPath           = "/usr/local/enpu/vcann-rt/tools/enpu-monitor"
	defaultENPUPreloadPath           = "/usr/local/enpu/vcann-rt/ld.so.preload"
	defaultENPUSystemdDetectVirtPath = "/usr/bin/systemd-detect-virt"
	defaultENPUConfigRoot            = "/var/lib/hami-enpu"
	enpuConfigContainerPath          = "/etc/enpu/vcann-rt/npu_info.config"
	enpuRuntimeContainerPath         = "/usr/local/enpu/vcann-rt/lib/libvruntime.so"
	enpuMonitorContainerPath         = "/usr/local/enpu/vcann-rt/tools/enpu-monitor"
	enpuPreloadContainerPath         = "/etc/ld.so.preload"

	enpuPolicyAnnotation = "huawei.com/enpu-policy"
	enpuPolicyKey        = "huawei.com/scheduler.softShareDev.policy"

	enpuMemoryRequestAnnotation = "huawei.com/enpu-memory-request"
	enpuMemoryLimitAnnotation   = "huawei.com/enpu-memory-limit"
)

func enpuHostPath(envName, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(envName)); value != "" {
		return value
	}
	return fallback
}

func sanitizeENPUName(value string) string {
	var b strings.Builder
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

func enpuShmID(uuid string, physicalID int32) string {
	name := sanitizeENPUName(uuid)
	if name == "" {
		name = strconv.FormatInt(int64(physicalID), 10)
	}
	name = "hami-enpu-" + name
	if len(name) > 120 {
		name = name[:120]
	}
	return name
}

func enpuPolicy(pod *v1.Pod, defaults ...string) (int, error) {
	value := ""
	if pod != nil {
		if pod.Annotations != nil {
			value = pod.Annotations[enpuPolicyAnnotation]
			if value == "" {
				value = pod.Annotations[enpuPolicyKey]
			}
		}
		if value == "" && pod.Labels != nil {
			value = pod.Labels[enpuPolicyKey]
		}
	}
	if value == "" {
		value = strings.TrimSpace(os.Getenv("ENPU_SCHEDULING_POLICY"))
	}
	if value == "" && len(defaults) > 0 {
		value = strings.TrimSpace(defaults[0])
	}
	if value == "" {
		return 2, nil
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "fixed-share", "fixed_share", "fixed":
		return 1, nil
	case "2", "elastic":
		return 2, nil
	case "3", "best-effort", "best_effort", "besteffort":
		return 3, nil
	default:
		return 0, fmt.Errorf("invalid ENPU scheduling policy %q; use fixed-share, elastic, or best-effort", value)
	}
}

func enpuQuota(info RuntimeInfo, dev *manager.Device) (int64, int32, error) {
	memory := int64(0)
	if info.Memory != nil {
		memory = *info.Memory
	}
	if memory <= 0 && dev != nil {
		memory = dev.Memory
	}
	if memory <= 0 {
		return 0, 0, fmt.Errorf("ENPU memory quota must be positive")
	}
	core := int32(100)
	if info.Core != nil && *info.Core > 0 {
		core = *info.Core
	}
	if core < 1 || core > 100 {
		return 0, 0, fmt.Errorf("ENPU aicore quota %d is outside 1..100", core)
	}
	return memory, core, nil
}

func enpuMemoryValue(pod *v1.Pod, key string, fallback int64) (int64, error) {
	if pod == nil || pod.Annotations == nil {
		return fallback, nil
	}
	raw := strings.TrimSpace(pod.Annotations[key])
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("ENPU annotation %s must be a positive integer in MB, got %q", key, raw)
	}
	return value, nil
}

func enpuMemoryRequestLimit(pod *v1.Pod, memory int64, policy int) (int64, int64, error) {
	request, err := enpuMemoryValue(pod, enpuMemoryRequestAnnotation, memory)
	if err != nil {
		return 0, 0, err
	}
	if request != memory {
		return 0, 0, fmt.Errorf("ENPU memory request %d MB must equal HAMi allocated memory %d MB; omit %s to use the scheduled memory resource", request, memory, enpuMemoryRequestAnnotation)
	}
	limit, err := enpuMemoryValue(pod, enpuMemoryLimitAnnotation, memory)
	if err != nil {
		return 0, 0, err
	}
	if request > limit {
		return 0, 0, fmt.Errorf("ENPU memory request %d MB exceeds memory limit %d MB", request, limit)
	}
	if policy == 1 && request != limit {
		return 0, 0, fmt.Errorf("ENPU fixed-share policy requires memory request and limit to be equal")
	}
	return request, limit, nil
}

// enpuPhysicalID returns the physical DIE ID used by vCANN-RT.
func enpuPhysicalID(dev *manager.Device) int32 {
	if dev == nil {
		return -1
	}
	return dev.PhyID
}

// ensureENPUSingleDieMode verifies A3 single-DIE mode without changing it.
func (ps *PluginServer) ensureENPUSingleDieMode() error {
	if ps.mgr.CommonWord() != "Ascend910C" {
		return nil
	}
	const query = "npu-smi info -t multi-die-policy"
	out, err := runNpuSmi("info", "-t", "multi-die-policy")
	if err != nil {
		return fmt.Errorf("check ENPU Ascend910C single-DIE prerequisite: %s failed: %w: %s", query, err, strings.TrimSpace(string(out)))
	}
	independent, err := enpuSingleDieMode(out)
	if err != nil {
		return fmt.Errorf("check ENPU Ascend910C single-DIE prerequisite: %s: %w", query, err)
	}
	if !independent {
		return fmt.Errorf("ENPU on Ascend910C requires INDEP_POLICY (single-DIE mode), but the node uses UNION_POLICY; the node administrator must configure 'npu-smi set -t multi-die-policy -d 1' before retrying; device-plugin does not change this global setting")
	}
	return nil
}

// enpuSingleDieMode parses the driver policy and rejects missing or ambiguous fields.
func enpuSingleDieMode(output []byte) (bool, error) {
	found := false
	independent := false
	for _, line := range strings.Split(string(output), "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok || !strings.EqualFold(strings.TrimSpace(key), "Multi_die policy") {
			continue
		}
		if found {
			return false, fmt.Errorf("multiple Multi_die policy fields in npu-smi output")
		}
		found = true
		switch strings.TrimSpace(value) {
		case "INDEP_POLICY":
			independent = true
		case "UNION_POLICY":
			independent = false
		default:
			return false, fmt.Errorf("unrecognized Multi_die policy %q; expected INDEP_POLICY or UNION_POLICY", strings.TrimSpace(value))
		}
	}
	if !found {
		return false, fmt.Errorf("multi-die policy field missing; verify that the installed driver supports this query")
	}
	return independent, nil
}

// enpuExposeAllDevices reports whether full device visibility is explicitly enabled.
func enpuExposeAllDevices() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("ENPU_EXPOSE_ALL_DEVICES"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func enpuDeviceSpecs(devices []*manager.Device) []*v1beta1.DeviceSpec {
	if !enpuExposeAllDevices() {
		return nil
	}
	seen := make(map[int32]struct{}, len(devices))
	result := make([]*v1beta1.DeviceSpec, 0, len(devices))
	for _, dev := range devices {
		if dev == nil || dev.PhyID < 0 {
			continue
		}
		if _, ok := seen[dev.PhyID]; ok {
			continue
		}
		seen[dev.PhyID] = struct{}{}
		path := fmt.Sprintf("/dev/davinci%d", dev.PhyID)
		result = append(result, &v1beta1.DeviceSpec{
			ContainerPath: path,
			HostPath:      path,
			Permissions:   "rwm",
		})
	}
	return result
}

func (ps *PluginServer) writeENPUConfig(pod *v1.Pod, ctrName string, dev *manager.Device, info RuntimeInfo, policy int) (string, error) {
	if dev == nil {
		return "", fmt.Errorf("ENPU device is nil")
	}
	memory, core, err := enpuQuota(info, dev)
	if err != nil {
		return "", err
	}
	request, limit, err := enpuMemoryRequestLimit(pod, memory, policy)
	if err != nil {
		return "", err
	}
	if pod == nil || pod.UID == "" || ctrName == "" {
		return "", fmt.Errorf("ENPU allocation requires a Pod UID and container name")
	}
	uid, container := string(pod.UID), ctrName
	if sanitizeENPUName(uid) != uid || strings.Contains(uid, "_") || sanitizeENPUName(container) != container {
		return "", fmt.Errorf("invalid ENPU allocation identity %q/%q", uid, container)
	}
	root := enpuHostPath("ENPU_CONFIG_ROOT", defaultENPUConfigRoot)
	physicalID := enpuPhysicalID(dev)
	contents := func(slot int) string {
		return fmt.Sprintf("physical-npu-id=%d\nvirtual-npu-id=%d\naicore-quota=%d\nmemory-request=%d\nmemory-limit=%d\nmemory-quota=%d\nshm-id=%s\nscheduling-policy=%d\n",
			physicalID, slot, core, request, limit, memory, enpuShmID(dev.UUID, physicalID), policy)
	}
	configPath, err := reserveENPUConfig(root, uid, container, physicalID, contents, func() (map[string]bool, error) {
		return enpuLivePodUIDs(ps.nodeName)
	})
	if err != nil {
		return "", err
	}
	klog.V(4).Infof("reserved ENPU config for %s/%s at %s: physical=%d core=%d memoryRequest=%d memoryLimit=%d policy=%d",
		uid, container, configPath, physicalID, core, request, limit, policy)
	return configPath, nil
}

func enpuMounts(configPath string) []*v1beta1.Mount {
	npuSmiHostPath := enpuHostPath("NPU_SMI_PATH", "/usr/local/sbin")
	npuSmiContainerPath := "/usr/local/sbin"
	if info, err := os.Stat(npuSmiHostPath); err == nil && !info.IsDir() {
		npuSmiContainerPath = "/usr/local/sbin/npu-smi"
	}
	mounts := []*v1beta1.Mount{
		{HostPath: "/dev/shm", ContainerPath: "/dev/shm", ReadOnly: false},
		{HostPath: npuSmiHostPath, ContainerPath: npuSmiContainerPath, ReadOnly: true},
		{HostPath: "/etc/ascend_install.info", ContainerPath: "/etc/ascend_install.info", ReadOnly: true},
		{HostPath: "/usr/local/Ascend/driver", ContainerPath: "/usr/local/Ascend/driver", ReadOnly: true},
		{HostPath: enpuHostPath("ENPU_RUNTIME_PATH", defaultENPURuntimePath), ContainerPath: enpuRuntimeContainerPath, ReadOnly: true},
		{HostPath: enpuHostPath("ENPU_MONITOR_PATH", defaultENPUMonitorPath), ContainerPath: enpuMonitorContainerPath, ReadOnly: true},
		{HostPath: enpuHostPath("ENPU_PRELOAD_PATH", defaultENPUPreloadPath), ContainerPath: enpuPreloadContainerPath, ReadOnly: true},
		{HostPath: configPath, ContainerPath: enpuConfigContainerPath, ReadOnly: true},
	}
	if path := strings.TrimSpace(os.Getenv("ENPU_SYSTEMD_DETECT_VIRT_PATH")); path != "" {
		mounts = append(mounts, &v1beta1.Mount{HostPath: path, ContainerPath: defaultENPUSystemdDetectVirtPath, ReadOnly: true})
	}
	return mounts
}
