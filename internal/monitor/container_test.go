package monitor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

func coreMonitorPod(t *testing.T, uid string, containers []string, uuids []string) *corev1.Pod {
	t.Helper()
	const deviceResource = "huawei.com/Ascend910"
	devices := make([]map[string]string, len(uuids))
	for i, uuid := range uuids {
		devices[i] = map[string]string{"UUID": uuid}
	}
	annotation, err := json.Marshal(devices)
	if err != nil {
		t.Fatal(err)
	}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		UID: types.UID(uid), Namespace: "test", Name: uid,
		Annotations: map[string]string{deviceResource: string(annotation)},
	}, Status: corev1.PodStatus{Phase: corev1.PodRunning}}
	for _, name := range containers {
		pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{Name: name,
			Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{
				corev1.ResourceName(deviceResource): resource.MustParse("2"),
			}},
		})
	}
	return pod
}

func coreMonitorLister(t *testing.T, root string, pods ...*corev1.Pod) (*ContainerLister, cache.Indexer) {
	t.Helper()
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, pod := range pods {
		if err := indexer.Add(pod); err != nil {
			t.Fatal(err)
		}
	}
	return &ContainerLister{containersPath: root, podLister: corelisters.NewPodLister(indexer)}, indexer
}

func TestCorePerPhysicalDeviceDomains(t *testing.T) {
	root := t.TempDir()
	// Neither DCMI enumeration nor pod annotation ordering matches physical IDs.
	var devices []DeviceStat
	var uuids []string
	for _, physical := range []int{7, 3, 5, 0, 2, 1, 6, 4} {
		uuid := fmt.Sprintf("die-%d", physical)
		devices = append(devices, DeviceStat{Index: 20 + physical, PhysicalID: physical, UUID: uuid})
		uuids = append(uuids, uuid)
		writeCoreShmem(t, filepath.Join(root, "pod_model", fmt.Sprintf("%s.device-%d", localShmemName, physical)),
			uint64(6000+physical), true, map[int]coreProcessFixture{
				0: {pid: 100, active: 1, memory: [hbmDevices]uint64{uint64(100 + physical), 90000, 80000}},
				1: {pid: 101, active: 1, memory: [hbmDevices]uint64{uint64(10 + physical)}},
			})
	}
	// A leftover legacy file must not add usage or override a domain's limit.
	writeCoreShmem(t, filepath.Join(root, "pod_model", localShmemName), 99999, true,
		map[int]coreProcessFixture{0: {pid: 1, active: 1, memory: [hbmDevices]uint64{99999}}})
	lister, _ := coreMonitorLister(t, root, coreMonitorPod(t, "pod", []string{"model"}, uuids))
	entries, err := lister.ListContainers(devices)
	if err != nil || len(entries) != 1 || len(entries[0].Devices) != 8 {
		t.Fatalf("entries = %+v, error = %v", entries, err)
	}
	for _, got := range entries[0].Devices {
		want := ContainerDeviceStats{Index: got.Index, UUID: fmt.Sprintf("die-%d", got.Index),
			MemoryUsed: uint64(110 + 2*got.Index), MemoryLimit: uint64(6000 + got.Index)}
		if got != want {
			t.Errorf("device = %+v, want %+v", got, want)
		}
	}
}

func TestCoreMissingDomainsAndContainerIsolation(t *testing.T) {
	root := t.TempDir()
	devices := []DeviceStat{{PhysicalID: 7, UUID: "die-7"}, {PhysicalID: 3, UUID: "die-3"}}
	pod := coreMonitorPod(t, "pod", []string{"first", "second"}, []string{"die-3", "die-7"})
	lister, _ := coreMonitorLister(t, root, pod)
	first := filepath.Join(root, "pod_first")
	second := filepath.Join(root, "pod_second")
	for _, fixture := range []struct {
		dir, suffix string
		used        uint64
	}{
		{first, "7", 700}, {second, "3", 300},
	} {
		writeCoreShmem(t, filepath.Join(fixture.dir, localShmemName+".device-"+fixture.suffix), 6000, true,
			map[int]coreProcessFixture{0: {pid: 1, active: 1, memory: [hbmDevices]uint64{fixture.used}}})
	}
	entries, err := lister.ListContainers(devices)
	if err != nil || len(entries) != 2 || len(entries[0].Devices) != 1 || len(entries[1].Devices) != 1 {
		t.Fatalf("pod-wide UUID list leaked into containers: %+v, %v", entries, err)
	}
	if entries[0].Devices[0].UUID != "die-7" || entries[1].Devices[0].UUID != "die-3" {
		t.Fatalf("wrong container attribution: %+v", entries)
	}
	// An uninitialized domain suppresses both itself and stale legacy fallback.
	writeCoreShmem(t, filepath.Join(first, localShmemName+".device-7"), 9999, false, nil)
	writeCoreShmem(t, filepath.Join(first, localShmemName), 9999, true,
		map[int]coreProcessFixture{0: {pid: 1, active: 1, memory: [hbmDevices]uint64{9999}}})
	if entries, err = lister.ListContainers(devices); err != nil || len(entries) != 1 || entries[0].ContainerName != "second" {
		t.Fatalf("uninitialized domain retained metrics: %+v, %v", entries, err)
	}
	if err := os.Remove(filepath.Join(second, localShmemName+".device-3")); err != nil {
		t.Fatal(err)
	}
	if entries, err = lister.ListContainers(devices); err != nil || len(entries) != 0 {
		t.Fatalf("removed domain retained metrics: %+v, %v", entries, err)
	}
}

func TestCoreLegacyShmemAndPodCleanup(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "pod_model")
	path := filepath.Join(dir, localShmemName)
	writeCoreShmem(t, path, 5000, true, map[int]coreProcessFixture{
		0: {pid: 100, active: 1, memory: [hbmDevices]uint64{700, 300}},
	})
	pod := coreMonitorPod(t, "pod", []string{"model"}, []string{"die-7", "die-3"})
	lister, indexer := coreMonitorLister(t, root, pod)
	entries, err := lister.ListContainers(nil)
	want := []ContainerDeviceStats{{Index: 0, UUID: "die-7", MemoryUsed: 700, MemoryLimit: 5000},
		{Index: 1, UUID: "die-3", MemoryUsed: 300, MemoryLimit: 5000}}
	if err != nil || len(entries) != 1 || !reflect.DeepEqual(entries[0].Devices, want) {
		t.Fatalf("legacy entries = %+v, %v, want %+v", entries, err, want)
	}
	for _, phase := range []corev1.PodPhase{corev1.PodSucceeded, corev1.PodFailed} {
		writeCoreShmem(t, path, 5000, true, nil)
		pod.Status.Phase = phase
		if err := indexer.Update(pod); err != nil {
			t.Fatal(err)
		}
		if entries, err := lister.ListContainers(nil); err != nil || len(entries) != 0 {
			t.Fatalf("terminal pod retained metrics: %+v, %v", entries, err)
		}
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("stale directory retained: %v", err)
		}
	}
	writeCoreShmem(t, path, 5000, true, nil)
	if err := indexer.Delete(pod); err != nil {
		t.Fatal(err)
	}
	if entries, err := lister.ListContainers(nil); err != nil || len(entries) != 0 {
		t.Fatalf("deleted pod retained metrics: %+v, %v", entries, err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("deleted pod directory retained: %v", err)
	}
}
