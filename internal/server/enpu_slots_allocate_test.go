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
	"strconv"
	"strings"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/Project-HAMi/HAMi/pkg/device"
	"github.com/Project-HAMi/HAMi/pkg/util"
	"github.com/Project-HAMi/HAMi/pkg/util/client"
	"github.com/Project-HAMi/ascend-device-plugin/internal/manager"
)

func TestAllocateENPUSlotsFollowSelectedPhysicalDevice(t *testing.T) {
	t.Setenv("ENPU_CONFIG_ROOT", t.TempDir())
	t.Setenv("ENPU_MANAGER_URL", "")
	t.Setenv("ENPU_SCHEDULING_POLICY", "elastic")
	t.Setenv("ENPU_EXPOSE_ALL_DEVICES", "false")
	t.Cleanup(setupInRequestDevices(testCommonWord))

	selected := &manager.Device{UUID: "selected-die", PhyID: 5, Memory: 65536}
	ps := &PluginServer{
		commonWord:        testCommonWord,
		nodeName:          "test-node",
		toAllocDeviceAnno: "hami.io/Ascend910-devices-to-allocate",
		allocAnno:         "huawei.com/Ascend910",
		mgr: &FakeManager{
			CommonWordFunc: func() string { return testCommonWord },
			GetDeviceByUUIDFunc: func(uuid string) *manager.Device {
				if uuid == selected.UUID {
					return selected
				}
				return nil
			},
		},
	}
	memory, core := int64(16384), int32(20)
	runtimeInfo, err := json.Marshal([]RuntimeInfo{{UUID: selected.UUID, Memory: &memory, Core: &core}})
	if err != nil {
		t.Fatal(err)
	}
	devices := device.EncodePodSingleDevice(device.PodSingleDevice{{cd(selected.UUID, testCommonWord, int32(memory), core)}})
	slots := make(map[int]bool)
	var previous *v1.Pod
	for i, requestedID := range []string{"die-a-3", "die-b-3"} {
		_, pod, cleanup := setupAllocateEnv(ps.nodeName, fmt.Sprintf("pod-%d", i), "default", 1, nil)
		t.Cleanup(cleanup)
		if previous != nil {
			if _, err := client.KubeClient.CoreV1().Pods(previous.Namespace).Create(context.Background(), previous, metav1.CreateOptions{}); err != nil {
				t.Fatal(err)
			}
		}
		pod.UID = types.UID(fmt.Sprintf("pod-uid-%d", i))
		pod.Annotations = map[string]string{
			VNPUModeAnnotation:                    VNPUModeENPU,
			ps.toAllocDeviceAnno:                  devices,
			ps.allocAnno:                          string(runtimeInfo),
			util.BindTimeAnnotations:              "2024-01-01T00:00:00Z",
			util.DeviceBindPhase:                  util.DeviceBindAllocating,
			"hami.io/Ascend910-devices-allocated": devices,
		}
		if _, err := client.KubeClient.CoreV1().Pods(pod.Namespace).Update(context.Background(), pod, metav1.UpdateOptions{}); err != nil {
			t.Fatal(err)
		}
		resp, err := ps.Allocate(context.Background(), &v1beta1.AllocateRequest{
			ContainerRequests: []*v1beta1.ContainerAllocateRequest{{DevicesIds: []string{requestedID}}},
		})
		if err != nil {
			t.Fatalf("Allocate(%q): %v", requestedID, err)
		}
		if len(resp.ContainerResponses) != 1 {
			t.Fatalf("Allocate(%q) returned %d containers", requestedID, len(resp.ContainerResponses))
		}
		container := resp.ContainerResponses[0]
		if got := container.Envs["ASCEND_VISIBLE_DEVICES"]; got != "5" {
			t.Fatalf("Allocate(%q) visible device = %q, want selected physical ID 5", requestedID, got)
		}
		configPath := ""
		for _, mount := range container.Mounts {
			if mount.ContainerPath == enpuConfigContainerPath {
				configPath = mount.HostPath
			}
		}
		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("read Allocate(%q) config: %v", requestedID, err)
		}
		config := make(map[string]string)
		for _, line := range strings.Split(string(data), "\n") {
			if key, value, ok := strings.Cut(line, "="); ok {
				config[key] = value
			}
		}
		if config["physical-npu-id"] != "5" {
			t.Fatalf("Allocate(%q) config selected wrong physical device: %s", requestedID, data)
		}
		slot, err := strconv.Atoi(config["virtual-npu-id"])
		if err != nil || slot < 0 || slot >= 100 || slots[slot] {
			t.Fatalf("Allocate(%q) invalid or duplicate virtual slot %q: %v", requestedID, config["virtual-npu-id"], err)
		}
		slots[slot] = true
		previous, err = client.KubeClient.CoreV1().Pods(pod.Namespace).Get(context.Background(), pod.Name, metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
	}
}
