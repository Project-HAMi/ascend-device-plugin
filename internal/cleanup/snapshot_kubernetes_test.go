/*
 * Copyright 2026 The HAMi Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 */

package cleanup

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/Project-HAMi/ascend-device-plugin/internal/manager"
)

func TestKubernetesPodSnapshotMapsLivePodToCard(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "serve", UID: "pod-1"},
		Spec: corev1.PodSpec{
			NodeName: "node-1",
			Containers: []corev1.Container{{
				Name: "worker",
				Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{
					corev1.ResourceName("huawei.com/Ascend910B4"): resource.MustParse("1"),
				}},
			}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:  "worker",
				State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
			}},
		},
	}
	pod.Annotations = map[string]string{
		"huawei.com/Ascend910B4": `[{"UUID":"uuid-7","Temp":"vir05_1c_16g"}]`,
	}
	clientset := fake.NewSimpleClientset(pod)
	snapshot, err := NewKubernetesPodSnapshot(clientset, "node-1", fakeResolver{device: &manager.Device{UUID: "uuid-7", CardID: 7}}, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(snapshot.Stop)
	checkpoint := filepath.Join(t.TempDir(), "kubelet_internal_checkpoint")
	if err := os.WriteFile(checkpoint, []byte(`{"Data":{"PodDeviceEntries":[],"RegisteredDevices":{"huawei.com/Ascend910B4":["uuid-7"]}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	snapshot.checkpointPath = checkpoint
	waitForInformer(t, snapshot.podListerSynced)
	result, err := snapshot.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	claim, ok := result.ClaimsByCard[7]
	if !ok || claim.State != ClaimProtected {
		t.Fatalf("ClaimsByCard[7] = %+v, want protected claim", claim)
	}
}

func TestKubernetesPodSnapshotFailsClosedForUnknownCheckpointPod(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	snapshot, err := NewKubernetesPodSnapshot(clientset, "node-1", fakeResolver{}, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(snapshot.Stop)
	checkpoint := filepath.Join(t.TempDir(), "kubelet_internal_checkpoint")
	if err := os.WriteFile(checkpoint, []byte(`{"Data":{"PodDeviceEntries":[{"PodUID":"gone"}],"RegisteredDevices":{"huawei.com/Ascend910B4":["uuid-7"]}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	snapshot.checkpointPath = checkpoint
	waitForInformer(t, snapshot.podListerSynced)
	result, err := snapshot.Snapshot(context.Background())
	if err == nil || result.Ready || len(result.UnknownPodUIDs) != 1 {
		t.Fatalf("Snapshot() = result=%+v err=%v, want fail-closed unknown checkpoint Pod", result, err)
	}
}

type fakeResolver struct{ device *manager.Device }

func (r fakeResolver) GetDeviceByUUID(string) *manager.Device { return r.device }

func waitForInformer(t *testing.T, synced func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !synced() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !synced() {
		t.Fatal("pod informer did not sync")
	}
}
