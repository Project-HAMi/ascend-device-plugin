/*
 * Copyright 2026 The HAMi Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 */

package server

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func slotTestContents(physical int32) func(int) string {
	return func(slot int) string {
		return fmt.Sprintf("physical-npu-id=%d\nvirtual-npu-id=%d\naicore-quota=20\nmemory-request=16384\nmemory-limit=16384\nmemory-quota=16384\nshm-id=hami-enpu-die-%d\nscheduling-policy=2\n", physical, slot, physical)
	}
}

func TestENPUSlotsPersistAndReuse(t *testing.T) {
	root := t.TempDir()
	first, err := reserveENPUConfig(root, "pod-a", "vllm", 0, slotTestContents(0), nil)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(first)
	if err != nil {
		t.Fatal(err)
	}
	second, err := reserveENPUConfig(root, "pod-b", "vllm", 0, slotTestContents(0), nil)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("distinct containers received the same config")
	}
	for range 2 {
		retry, err := reserveENPUConfig(root, "pod-a", "vllm", 0, slotTestContents(0), nil)
		if err != nil || retry != first {
			t.Fatalf("retry = %s, %v; want %s", retry, err, first)
		}
	}
	after, err := os.Stat(first)
	if err != nil || !os.SameFile(before, after) {
		t.Fatalf("retry replaced a config mounted by the running container: %v", err)
	}
	if _, err := reserveENPUConfig(root, "pod-c", "vllm", 1, slotTestContents(1), nil); err != nil {
		t.Fatal(err)
	}
	slots, err := readENPUSlots(root)
	if err != nil || len(slots) != 3 {
		t.Fatalf("persisted slots = %+v, %v", slots, err)
	}
	for i, want := range []int{0, 1, 0} {
		if slots[i].virtual != want {
			t.Fatalf("slot %d = %d, want %d", i, slots[i].virtual, want)
		}
	}
	if _, err := reserveENPUConfig(root, "pod-a", "vllm", 1, slotTestContents(1), nil); err == nil {
		t.Fatal("retry silently changed physical DIE")
	}
	changed := func(slot int) string {
		return strings.ReplaceAll(slotTestContents(0)(slot), "aicore-quota=20", "aicore-quota=50")
	}
	if _, err := reserveENPUConfig(root, "pod-a", "vllm", 0, changed, nil); err == nil {
		t.Fatal("retry silently changed running quota")
	}
	drift := func(slot int) string {
		return strings.ReplaceAll(slotTestContents(2)(slot), "shm-id=hami-enpu-die-2", "shm-id=hami-enpu-die-0")
	}
	if _, err := reserveENPUConfig(root, "pod-d", "vllm", 2, drift, nil); err == nil {
		t.Fatal("shared-memory identity silently moved to a different physical DIE")
	}
}

func TestENPUSlotsConcurrentAndExhausted(t *testing.T) {
	root := t.TempDir()
	var wg sync.WaitGroup
	for i := range 100 {
		wg.Go(func() {
			if _, err := reserveENPUConfig(root, fmt.Sprintf("pod-%03d", i), "vllm", 0, slotTestContents(0), nil); err != nil {
				t.Errorf("allocation %d: %v", i, err)
			}
		})
	}
	wg.Wait()
	slots, err := readENPUSlots(root)
	if err != nil || len(slots) != 100 {
		t.Fatalf("slots=%d, error=%v", len(slots), err)
	}
	seen := make(map[int]bool)
	for _, slot := range slots {
		if seen[slot.virtual] {
			t.Fatalf("duplicate virtual ID %d", slot.virtual)
		}
		seen[slot.virtual] = true
	}
	if _, err := reserveENPUConfig(root, "overflow", "vllm", 0, slotTestContents(0), nil); err == nil || !strings.Contains(err.Error(), "all 100") {
		t.Fatalf("exhausted allocation = %v", err)
	}
}

func TestENPUSlotsAcrossProcesses(t *testing.T) {
	if root := os.Getenv("HAMI_TEST_ENPU_SLOT_ROOT"); root != "" {
		for i := range 5 {
			uid := fmt.Sprintf("%s-%d", os.Getenv("HAMI_TEST_ENPU_SLOT_OWNER"), i)
			if _, err := reserveENPUConfig(root, uid, "vllm", 0, slotTestContents(0), nil); err != nil {
				t.Fatal(err)
			}
		}
		return
	}
	root := t.TempDir()
	run := func(owner string) {
		cmd := exec.Command(os.Args[0], "-test.run=^TestENPUSlotsAcrossProcesses$")
		cmd.Env = append(os.Environ(), "HAMI_TEST_ENPU_SLOT_ROOT="+root, "HAMI_TEST_ENPU_SLOT_OWNER="+owner)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("allocator process %s: %v\n%s", owner, err, out)
		}
	}
	var wg sync.WaitGroup
	for _, owner := range []string{"first", "second"} {
		wg.Go(func() { run(owner) })
	}
	wg.Wait()
	run("first")
	slots, err := readENPUSlots(root)
	if err != nil || len(slots) != 10 {
		t.Fatalf("cross-process reservations = %d, %v; want 10", len(slots), err)
	}
	seen := make(map[int]bool)
	for _, slot := range slots {
		if seen[slot.virtual] {
			t.Fatalf("processes shared virtual ID %d", slot.virtual)
		}
		seen[slot.virtual] = true
	}
}

func TestENPUSlotsCleanupRequiresVerifiedLiveness(t *testing.T) {
	for _, state := range []string{"released", "still allocated", "unavailable"} {
		t.Run(state, func(t *testing.T) {
			root := t.TempDir()
			old, err := reserveENPUConfig(root, "old-pod", "vllm", 0, slotTestContents(0), nil)
			if err != nil {
				t.Fatal(err)
			}
			live := func() (map[string]bool, error) {
				if state == "unavailable" {
					return nil, errors.New("checkpoint unavailable")
				}
				return map[string]bool{"new-pod": true, "old-pod": state == "still allocated"}, nil
			}
			path, err := reserveENPUConfig(root, "new-pod", "vllm", 0, slotTestContents(0), live)
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			want := 0
			if state != "released" {
				want = 1
			}
			if err != nil || string(data) != slotTestContents(0)(want) {
				t.Fatalf("new allocation %q, %v; want slot %d", data, err, want)
			}
			_, err = os.Stat(old)
			if state != "released" && err != nil || state == "released" && !os.IsNotExist(err) {
				t.Fatalf("old config retention incorrect: %v", err)
			}
		})
	}
}

func TestENPUSlotsPreserveLegacyAndRejectCollisions(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "legacy-pod_vllm", "npu_info.config")
	if err := writeENPUConfigAtomic(legacy, slotTestContents(0)(3)); err != nil {
		t.Fatal(err)
	}
	if _, err := reserveENPUConfig(root, "new-pod", "vllm", 0, slotTestContents(0), nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(legacy)
	if err != nil || string(data) != slotTestContents(0)(3) {
		t.Fatalf("legacy config changed: %q, %v", data, err)
	}
	duplicate := filepath.Join(root, "duplicate-pod_vllm", "npu_info.config")
	if err := writeENPUConfigAtomic(duplicate, slotTestContents(0)(3)); err != nil {
		t.Fatal(err)
	}
	if _, err := reserveENPUConfig(root, "third-pod", "vllm", 0, slotTestContents(0), nil); err == nil || !strings.Contains(err.Error(), "duplicate ENPU") {
		t.Fatalf("existing collision accepted: %v", err)
	}
}

func TestENPUCheckpointLiveness(t *testing.T) {
	for _, tc := range []struct {
		name, data string
		valid      bool
	}{
		{"allocated", `{"Data":{"PodDeviceEntries":[{"PodUID":"old-pod"}],"RegisteredDevices":{"huawei.com/Ascend910C":["die-a-3"]}}}`, true},
		{"empty", `{"Data":{"PodDeviceEntries":[],"RegisteredDevices":{"huawei.com/Ascend910C":["die-a-3"]}}}`, true},
		{"missing data", `{}`, false},
		{"missing entries", `{"Data":{"RegisteredDevices":{"huawei.com/Ascend910C":["die-a-3"]}}}`, false},
		{"unregistered", `{"Data":{"RegisteredDevices":{}}}`, false},
		{"missing UID", `{"Data":{"PodDeviceEntries":[{}],"RegisteredDevices":{"huawei.com/Ascend910C":["die-a-3"]}}}`, false},
		{"truncated", `{"Data":`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			active, err := enpuCheckpointPodUIDs([]byte(tc.data))
			if (err == nil) != tc.valid {
				t.Fatalf("checkpoint accepted=%v, want %v: %v", err == nil, tc.valid, err)
			}
			if tc.name == "allocated" && !active["old-pod"] {
				t.Fatal("kubelet-owned Pod would be reclaimed")
			}
		})
	}
}
