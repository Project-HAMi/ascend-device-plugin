/*
 * Copyright 2026 The HAMi Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 */

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/Project-HAMi/HAMi/pkg/util/client"
)

type enpuSlot struct {
	uid, path, contents, shm string
	physical, virtual        int
}

// reserveENPUConfig serializes persistent slot allocation across plugin processes.
func reserveENPUConfig(root, uid, container string, physical int32, contents func(int) string, livePods func() (map[string]bool, error)) (string, error) {
	if physical < 0 {
		return "", fmt.Errorf("invalid ENPU physical ID %d", physical)
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		return "", err
	}
	lock, err := os.OpenFile(filepath.Join(root, ".allocation.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return "", err
	}
	defer func() { _ = lock.Close() }()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return "", err
	}
	defer func() { _ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN) }()
	slots, err := readENPUSlots(root)
	if err != nil {
		return "", err
	}
	var active map[string]bool
	if len(slots) > 0 && livePods != nil {
		active, err = livePods()
		if err != nil {
			klog.Warningf("Retaining ENPU slots: cannot verify allocation cleanup: %v", err)
			active = nil
		}
	}
	configPath := filepath.Join(root, uid+"_"+container, "npu_info.config")
	used := make(map[int]string)
	var existing *enpuSlot
	for i := range slots {
		slot := &slots[i]
		if active != nil && !active[slot.uid] && slot.uid != uid {
			if err := os.Remove(slot.path); err != nil {
				return "", fmt.Errorf("release ENPU slot %s: %w", slot.path, err)
			}
			_ = os.Remove(filepath.Dir(slot.path))
			continue
		}
		if slot.path == configPath {
			existing = slot
			if slot.physical != int(physical) || slot.contents != contents(slot.virtual) {
				return "", fmt.Errorf("existing ENPU allocation %s differs from the requested DIE or quotas", configPath)
			}
		}
		sameShm := strings.Contains(contents(slot.virtual), "\nshm-id="+slot.shm+"\n")
		if sameShm && slot.physical != int(physical) {
			return "", fmt.Errorf("ENPU shared-memory identity %s is already assigned to physical DIE %d", slot.shm, slot.physical)
		}
		if slot.physical != int(physical) {
			continue
		}
		if !sameShm {
			return "", fmt.Errorf("ENPU physical DIE %d has a conflicting shared-memory identity in %s", physical, slot.path)
		}
		if previous, found := used[slot.virtual]; found {
			return "", fmt.Errorf("duplicate ENPU virtual ID %d on DIE %d in %s and %s; stop the affected workloads before removing their configs", slot.virtual, physical, previous, slot.path)
		}
		used[slot.virtual] = slot.path
	}
	if existing != nil {
		return configPath, nil
	}
	for slot := range 100 {
		if _, occupied := used[slot]; occupied {
			continue
		}
		if err := writeENPUConfigAtomic(configPath, contents(slot)); err != nil {
			return "", err
		}
		if err := syncENPUDirectory(root); err != nil {
			return "", err
		}
		return configPath, nil
	}
	return "", fmt.Errorf("all 100 ENPU virtual IDs on physical DIE %d are reserved", physical)
}

func readENPUSlots(root string) ([]enpuSlot, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var slots []enpuSlot
	for _, entry := range entries {
		uid, container, managed := strings.Cut(entry.Name(), "_")
		if !managed || uid == "" || container == "" || !entry.IsDir() {
			continue
		}
		path := filepath.Join(root, entry.Name(), "npu_info.config")
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		values := make(map[string]string)
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			key, value, ok := strings.Cut(line, "=")
			key, value = strings.TrimSpace(key), strings.TrimSpace(value)
			if _, duplicate := values[key]; !ok || duplicate {
				return nil, fmt.Errorf("invalid ENPU config %s", path)
			}
			values[key] = value
		}
		physical, phyErr := strconv.Atoi(values["physical-npu-id"])
		virtual, virtErr := strconv.Atoi(values["virtual-npu-id"])
		if phyErr != nil || virtErr != nil || physical < 0 || virtual < 0 || virtual >= 100 || values["shm-id"] == "" {
			return nil, fmt.Errorf("invalid ENPU identity in %s; preserve existing workloads and repair the config before allocating", path)
		}
		slots = append(slots, enpuSlot{uid: uid, path: path, contents: string(data), shm: values["shm-id"], physical: physical, virtual: virtual})
	}
	return slots, nil
}

func writeENPUConfigAtomic(path, contents string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".npu-info-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	defer func() { _ = tmp.Close() }()
	if err := tmp.Chmod(0644); err != nil {
		return err
	}
	if _, err := tmp.WriteString(contents); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	return syncENPUDirectory(filepath.Dir(path))
}

func syncENPUDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = dir.Close() }()
	return dir.Sync()
}

// enpuLivePodUIDs retains slots until both Kubernetes and kubelet release the Pod.
func enpuLivePodUIDs(node string) (map[string]bool, error) {
	kube := client.GetClient()
	if kube == nil || node == "" {
		return nil, fmt.Errorf("node Kubernetes client is unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pods, err := kube.CoreV1().Pods("").List(ctx, metav1.ListOptions{FieldSelector: "spec.nodeName=" + node})
	if err != nil {
		return nil, err
	}
	checkpoint, err := os.ReadFile("/var/lib/kubelet/device-plugins/kubelet_internal_checkpoint")
	if err != nil {
		return nil, err
	}
	active, err := enpuCheckpointPodUIDs(checkpoint)
	if err != nil {
		return nil, err
	}
	for _, pod := range pods.Items {
		active[string(pod.UID)] = true
	}
	return active, nil
}

func enpuCheckpointPodUIDs(contents []byte) (map[string]bool, error) {
	var checkpoint struct {
		Data *struct {
			PodDeviceEntries  json.RawMessage
			RegisteredDevices map[string][]string
		}
	}
	if err := json.Unmarshal(contents, &checkpoint); err != nil {
		return nil, fmt.Errorf("read kubelet allocation checkpoint: %w", err)
	}
	if checkpoint.Data == nil || checkpoint.Data.RegisteredDevices == nil || len(checkpoint.Data.PodDeviceEntries) == 0 {
		return nil, fmt.Errorf("kubelet allocation checkpoint is uninitialized or unsupported")
	}
	ascendRegistered := false
	for resource, ids := range checkpoint.Data.RegisteredDevices {
		if strings.HasPrefix(resource, "huawei.com/Ascend") && len(ids) > 0 {
			ascendRegistered = true
		}
	}
	if !ascendRegistered {
		return nil, fmt.Errorf("kubelet allocation checkpoint has no registered Ascend devices")
	}
	active := make(map[string]bool)
	var entries []struct{ PodUID string }
	if err := json.Unmarshal(checkpoint.Data.PodDeviceEntries, &entries); err != nil {
		return nil, fmt.Errorf("read kubelet Pod allocations: %w", err)
	}
	for _, entry := range entries {
		if entry.PodUID == "" {
			return nil, fmt.Errorf("kubelet allocation checkpoint contains an empty Pod UID")
		}
		active[entry.PodUID] = true
	}
	return active, nil
}
