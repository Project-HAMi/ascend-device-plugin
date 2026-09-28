package monitor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const enpuTestResource = "huawei.com/Ascend910C"

func enpuTestPod(uid, id, uuid string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: uid, Namespace: "test", UID: types.UID(uid), Annotations: map[string]string{
			"huawei.com/vnpu-mode":                 "enpu",
			"huawei.com/enpu-policy":               "best-effort",
			enpuTestResource:                       fmt.Sprintf(`[{"UUID":%q,"memory":20480,"core":30}]`, uuid),
			"hami.io/Ascend910C-devices-allocated": uuid + ",Ascend910C,20480,30:;",
		}},
		Spec:   corev1.PodSpec{Containers: []corev1.Container{{Name: "vllm", Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{enpuTestResource: resource.MustParse("1")}}}}},
		Status: corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{Name: "vllm", ContainerID: "containerd://" + id, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}},
	}
}

func enpuTestConfig(uuid string, physical, virtual int) string {
	return fmt.Sprintf("physical-npu-id=%d\nvirtual-npu-id=%d\naicore-quota=30\nmemory-request=20480\nmemory-limit=20480\nmemory-quota=20480\nshm-id=hami-enpu-%s\nscheduling-policy=3\n", physical, virtual, uuid)
}

func enpuWriteTestConfig(t *testing.T, root string, pod *corev1.Pod, manager bool, content string) string {
	t.Helper()
	dir := filepath.Join(root, string(pod.UID)+"_"+pod.Spec.Containers[0].Name)
	if manager {
		if filepath.Base(root) != "vcann-rt" {
			root = filepath.Join(root, "vcann-rt")
		}
		dir = filepath.Join(root, string(pod.UID), pod.Spec.Containers[0].Name)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "npu_info.config")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestListENPUAllocations(t *testing.T) {
	for _, mode := range []string{"enpu", " ubs-virt ", "VCANN-RT"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			pod := enpuTestPod("pod-1", strings.Repeat("A", 64), "die-0")
			pod.Annotations["huawei.com/vnpu-mode"] = mode
			enpuWriteTestConfig(t, root, pod, false, enpuTestConfig("die-0", 0, 2))
			got, err := listENPUAllocations([]*corev1.Pod{pod}, root, "")
			if err != nil || len(got) != 1 {
				t.Fatalf("got %v, %v", got, err)
			}
			a := got[0]
			if a.Namespace != "test" || a.PodName != "pod-1" || a.PodUID != "pod-1" || a.ContainerName != "vllm" || a.ContainerID != strings.Repeat("a", 64) || a.DeviceUUID != "die-0" || a.PhysicalID != 0 || a.VirtualID != 2 || a.Policy != "best-effort" || a.CoreQuota != 30 || a.MemoryRequest != 21474836480 || a.MemoryLimit != 21474836480 {
				t.Fatalf("incorrect allocation: %+v", a)
			}
		})
	}
}

func TestENPULegacyConfigAndAnnotationFallback(t *testing.T) {
	root := t.TempDir()
	pod := enpuTestPod("pod-1", strings.Repeat("a", 64), "die-0")
	delete(pod.Annotations, "hami.io/Ascend910C-devices-allocated")
	delete(pod.Annotations, "huawei.com/enpu-policy")
	pod.Labels = map[string]string{"huawei.com/scheduler.softShareDev.policy": "fixed_share"}
	config := strings.ReplaceAll(enpuTestConfig("die-0", 0, 0), "memory-request=20480\nmemory-limit=20480\n", "")
	config = strings.ReplaceAll(config, "scheduling-policy=3", "scheduling-policy=1")
	enpuWriteTestConfig(t, root, pod, false, "# legacy config\n"+config)
	got, err := listENPUAllocations([]*corev1.Pod{pod}, root, "")
	if err != nil || len(got) != 1 || got[0].Policy != "fixed-share" || got[0].MemoryRequest != 20*(1<<30) {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestENPUManagerRootSelection(t *testing.T) {
	for _, leaf := range []string{"", "vcann-rt"} {
		t.Run(leaf, func(t *testing.T) {
			root, manager := t.TempDir(), filepath.Join(t.TempDir(), leaf)
			pod := enpuTestPod("pod-1", strings.Repeat("a", 64), "die-15")
			pod.Annotations["huawei.com/enpu-memory-limit"] = "65536"
			config := strings.ReplaceAll(enpuTestConfig("die-15", 15, 127), "memory-limit=20480", "memory-limit=65536")
			config = strings.ReplaceAll(config, "memory-quota=20480\n", "")
			config = strings.ReplaceAll(config, "shm-id=hami-enpu-", "shm-id=")
			enpuWriteTestConfig(t, manager, pod, true, config)
			enpuWriteTestConfig(t, root, pod, false, "broken normal config")
			got, err := listENPUAllocations([]*corev1.Pod{pod}, root, manager)
			if err != nil || len(got) != 1 || got[0].MemoryLimit != 64*(1<<30) || got[0].PhysicalID != 15 || got[0].VirtualID != 127 {
				t.Fatalf("got %v, %v", got, err)
			}
			enpuWriteTestConfig(t, manager, pod, true, strings.ReplaceAll(config, "virtual-npu-id=127", "virtual-npu-id=128"))
			if got, err := listENPUAllocations([]*corev1.Pod{pod}, root, manager); err == nil || len(got) != 0 {
				t.Fatalf("manager VID128 accepted: %v, %v", got, err)
			}
		})
	}
}

func TestENPUInvalidContainerPreservesValidPeer(t *testing.T) {
	cases := []struct {
		name   string
		change func(*corev1.Pod, string) string
	}{
		{"duplicate key", func(_ *corev1.Pod, c string) string { return c + "physical-npu-id=0\n" }},
		{"malformed line", func(_ *corev1.Pod, c string) string { return c + "malformed\n" }},
		{"negative device", func(_ *corev1.Pod, c string) string {
			return strings.ReplaceAll(c, "physical-npu-id=0", "physical-npu-id=-1")
		}},
		{"unsupported device", func(_ *corev1.Pod, c string) string {
			return strings.ReplaceAll(c, "physical-npu-id=0", "physical-npu-id=16")
		}},
		{"virtual bound", func(_ *corev1.Pod, c string) string {
			return strings.ReplaceAll(c, "virtual-npu-id=0", "virtual-npu-id=100")
		}},
		{"zero core best effort", func(_ *corev1.Pod, c string) string {
			return strings.ReplaceAll(c, "aicore-quota=30", "aicore-quota=0")
		}},
		{"fractional core", func(_ *corev1.Pod, c string) string {
			return strings.ReplaceAll(c, "aicore-quota=30", "aicore-quota=30.5")
		}},
		{"unknown policy", func(_ *corev1.Pod, c string) string {
			return strings.ReplaceAll(c, "scheduling-policy=3", "scheduling-policy=4")
		}},
		{"nonnumeric config policy", func(_ *corev1.Pod, c string) string {
			return strings.ReplaceAll(c, "scheduling-policy=3", "scheduling-policy=best-effort")
		}},
		{"memory float overflow", func(_ *corev1.Pod, c string) string {
			return strings.ReplaceAll(c, "memory-limit=20480", "memory-limit=1.7976931348623157e308")
		}},
		{"memory byte overflow", func(_ *corev1.Pod, c string) string {
			return strings.ReplaceAll(c, "memory-limit=20480", "memory-limit=17592186044416")
		}},
		{"negative memory", func(_ *corev1.Pod, c string) string {
			return strings.ReplaceAll(c, "memory-limit=20480", "memory-limit=-1")
		}},
		{"partial request limit", func(_ *corev1.Pod, c string) string { return strings.ReplaceAll(c, "memory-request=20480\n", "") }},
		{"quota drift", func(_ *corev1.Pod, c string) string {
			return strings.ReplaceAll(c, "memory-quota=20480", "memory-quota=40960")
		}},
		{"request greater limit", func(_ *corev1.Pod, c string) string {
			return strings.ReplaceAll(c, "memory-limit=20480", "memory-limit=10240")
		}},
		{"missing runtime annotation", func(p *corev1.Pod, c string) string { delete(p.Annotations, enpuTestResource); return c }},
		{"malformed runtime annotation", func(p *corev1.Pod, c string) string { p.Annotations[enpuTestResource] = "[broken]"; return c }},
		{"runtime UUID mismatch", func(p *corev1.Pod, c string) string {
			p.Annotations[enpuTestResource] = `[{"UUID":"another"}]`
			return c
		}},
		{"conflicting runtime UUID", func(p *corev1.Pod, c string) string {
			p.Annotations[enpuTestResource] = `[{"UUID":"die-0","core":20},{"UUID":"die-0","core":30}]`
			return c
		}},
		{"hardware template", func(p *corev1.Pod, c string) string {
			p.Annotations[enpuTestResource] = `[{"UUID":"die-0","temp":"vir01"}]`
			return c
		}},
		{"memory annotation drift", func(p *corev1.Pod, c string) string {
			p.Annotations[enpuTestResource] = `[{"UUID":"die-0","memory":10240,"core":30}]`
			return c
		}},
		{"core annotation drift", func(p *corev1.Pod, c string) string {
			p.Annotations[enpuTestResource] = `[{"UUID":"die-0","memory":20480,"core":20}]`
			return c
		}},
		{"limit annotation drift", func(p *corev1.Pod, c string) string {
			p.Annotations["huawei.com/enpu-memory-limit"] = "65536"
			return c
		}},
		{"policy annotation drift", func(p *corev1.Pod, c string) string { p.Annotations["huawei.com/enpu-policy"] = "elastic"; return c }},
		{"allocated multiple devices", func(p *corev1.Pod, c string) string {
			p.Annotations["hami.io/Ascend910C-devices-allocated"] = "die-0,Ascend910C,20480,30:die-1,Ascend910C,20480,30:;"
			return c
		}},
		{"allocated type mismatch", func(p *corev1.Pod, c string) string {
			p.Annotations["hami.io/Ascend910C-devices-allocated"] = "die-0,Ascend910B,20480,30:;"
			return c
		}},
		{"allocated count mismatch", func(p *corev1.Pod, c string) string {
			p.Annotations["hami.io/Ascend910C-devices-allocated"] = "die-0,Ascend910C,20480,30:;;"
			return c
		}},
		{"invalid short container ID", func(p *corev1.Pod, c string) string {
			p.Status.ContainerStatuses[0].ContainerID = "containerd://abcdef"
			return c
		}},
		{"unknown runtime", func(p *corev1.Pod, c string) string {
			p.Status.ContainerStatuses[0].ContainerID = "unknown://" + strings.Repeat("a", 64)
			return c
		}},
		{"duplicate container status", func(p *corev1.Pod, c string) string {
			p.Status.ContainerStatuses = append(p.Status.ContainerStatuses, p.Status.ContainerStatuses[0])
			return c
		}},
		{"shm UUID mismatch", func(_ *corev1.Pod, c string) string {
			return strings.ReplaceAll(c, "shm-id=hami-enpu-die-0", "shm-id=hami-enpu-other")
		}},
		{"shm traversal", func(_ *corev1.Pod, c string) string {
			return strings.ReplaceAll(c, "shm-id=hami-enpu-die-0", "shm-id=../bad")
		}},
		{"oversized file", func(_ *corev1.Pod, c string) string { return c + strings.Repeat("#", 65537) }},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			bad := enpuTestPod("bad", strings.Repeat("a", 64), "die-0")
			good := enpuTestPod("good", strings.Repeat("b", 64), "die-1")
			config := tt.change(bad, enpuTestConfig("die-0", 0, 0))
			enpuWriteTestConfig(t, root, bad, false, config)
			enpuWriteTestConfig(t, root, good, false, enpuTestConfig("die-1", 1, 0))
			got, err := listENPUAllocations([]*corev1.Pod{bad, good}, root, "")
			if err == nil || len(got) != 1 || got[0].PodUID != "good" {
				t.Fatalf("got %v, %v", got, err)
			}
		})
	}
}

func TestENPUConfigPathsAndLiveness(t *testing.T) {
	root := t.TempDir()
	pod := enpuTestPod("pod-1", strings.Repeat("a", 64), "die-0")
	got, err := listENPUAllocations([]*corev1.Pod{pod}, root, "")
	if err == nil || len(got) != 0 {
		t.Fatalf("missing config: %v, %v", got, err)
	}
	path := enpuWriteTestConfig(t, root, pod, false, enpuTestConfig("die-0", 0, 0))
	for _, phase := range []corev1.PodPhase{corev1.PodUnknown, corev1.PodSucceeded, corev1.PodFailed} {
		copy := pod.DeepCopy()
		copy.Status.Phase = phase
		if got, err := listENPUAllocations([]*corev1.Pod{copy}, root, ""); err != nil || len(got) != 0 {
			t.Fatalf("phase %s: %v, %v", phase, got, err)
		}
	}
	for _, change := range []func(*corev1.Pod){
		func(p *corev1.Pod) { p.DeletionTimestamp = &metav1.Time{} },
		func(p *corev1.Pod) { p.Status.ContainerStatuses[0].State.Running = nil },
		func(p *corev1.Pod) { p.Annotations["huawei.com/vnpu-mode"] = "hami-core" },
	} {
		copy := pod.DeepCopy()
		change(copy)
		if got, err := listENPUAllocations([]*corev1.Pod{copy}, root, ""); err != nil || len(got) != 0 {
			t.Fatalf("inactive got %v, %v", got, err)
		}
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("discovery changed config", err)
	}
	stale := enpuTestPod("stale", strings.Repeat("c", 64), "die-0")
	stalePath := enpuWriteTestConfig(t, root, stale, false, enpuTestConfig("die-0", 0, 0))
	if got, err := listENPUAllocations([]*corev1.Pod{nil, pod}, root, ""); err != nil || len(got) != 1 {
		t.Fatalf("stale config should be ignored: %v, %v", got, err)
	}
	if _, err := os.Stat(stalePath); err != nil {
		t.Fatal("stale config was deleted", err)
	}
	outside := filepath.Join(t.TempDir(), "config")
	if err := os.Rename(path, outside); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, path); err != nil {
		t.Fatal(err)
	}
	if got, err := listENPUAllocations([]*corev1.Pod{pod}, root, ""); err == nil || len(got) != 0 {
		t.Fatalf("symlink accepted: %v, %v", got, err)
	}
}

func TestENPUDuplicateMappingsRejectBoth(t *testing.T) {
	for _, kind := range []string{"slot", "container", "physical", "uuid"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			a := enpuTestPod("a", strings.Repeat("a", 64), "die-0")
			b := enpuTestPod("b", strings.Repeat("b", 64), "die-0")
			good := enpuTestPod("good", strings.Repeat("c", 64), "die-2")
			configB := enpuTestConfig("die-0", 0, 0)
			switch kind {
			case "container":
				b.Status.ContainerStatuses[0].ContainerID = a.Status.ContainerStatuses[0].ContainerID
				configB = enpuTestConfig("die-0", 0, 1)
			case "physical":
				b = enpuTestPod("b", strings.Repeat("b", 64), "die-1")
				configB = enpuTestConfig("die-1", 0, 1)
			case "uuid":
				configB = enpuTestConfig("die-0", 1, 0)
			}
			enpuWriteTestConfig(t, root, a, false, enpuTestConfig("die-0", 0, 0))
			enpuWriteTestConfig(t, root, b, false, configB)
			enpuWriteTestConfig(t, root, good, false, enpuTestConfig("die-2", 2, 0))
			got, err := listENPUAllocations([]*corev1.Pod{a, b, good}, root, "")
			if err == nil || len(got) != 1 || got[0].PodUID != "good" {
				t.Fatalf("got %v, %v", got, err)
			}
		})
	}
}

func TestENPUMultipleContainersUseOwnAllocation(t *testing.T) {
	root := t.TempDir()
	pod := enpuTestPod("pod-1", strings.Repeat("a", 64), "die-0")
	second := pod.Spec.Containers[0].DeepCopy()
	second.Name = "second"
	pod.Spec.Containers = append(pod.Spec.Containers, *second)
	status := pod.Status.ContainerStatuses[0]
	status.Name = "second"
	status.ContainerID = "docker://" + strings.Repeat("b", 64)
	pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, status)
	pod.Annotations[enpuTestResource] = `[{"UUID":"die-0","memory":20480,"core":30},{"UUID":"die-1","memory":20480,"core":30}]`
	pod.Annotations["hami.io/Ascend910C-devices-allocated"] = "die-0,Ascend910C,20480,30:;die-1,Ascend910C,20480,30:;"
	enpuWriteTestConfig(t, root, pod, false, enpuTestConfig("die-0", 0, 0))
	copy := pod.DeepCopy()
	copy.Spec.Containers = []corev1.Container{*second}
	enpuWriteTestConfig(t, root, copy, false, enpuTestConfig("die-1", 1, 0))
	got, err := listENPUAllocations([]*corev1.Pod{pod}, root, "")
	if err != nil || len(got) != 2 || got[0].DeviceUUID != "die-0" || got[1].DeviceUUID != "die-1" || got[1].ContainerName != "second" {
		t.Fatalf("got %v, %v", got, err)
	}
	delete(pod.Annotations, "hami.io/Ascend910C-devices-allocated")
	if got, err := listENPUAllocations([]*corev1.Pod{pod}, root, ""); err == nil || len(got) != 0 {
		t.Fatalf("ambiguous mapping accepted: %v, %v", got, err)
	}
}

func TestENPUConfigUintBounds(t *testing.T) {
	for _, value := range []string{"0", "-1", "NaN", "Inf", "1.0", "1e10", "18446744073709551616", "17592186044416"} {
		if _, err := enpuConfigUint(value, 1, ^uint64(0)>>20); err == nil {
			t.Errorf("accepted %q", value)
		}
	}
	value, err := enpuConfigUint("17592186044415", 1, ^uint64(0)>>20)
	if err != nil || value<<20 != 18446744073708503040 {
		t.Fatalf("bad upper bound %d, %v", value, err)
	}
}

func TestENPUZeroAnnotationCoreUsesDefault(t *testing.T) {
	root := t.TempDir()
	pod := enpuTestPod("pod-1", strings.Repeat("a", 64), "die-0")
	pod.Annotations[enpuTestResource] = `[{"UUID":"die-0","memory":20480,"core":0}]`
	pod.Annotations["hami.io/Ascend910C-devices-allocated"] = "die-0,Ascend910C,20480,0:;"
	config := strings.ReplaceAll(enpuTestConfig("die-0", 0, 0), "aicore-quota=30", "aicore-quota=100")
	enpuWriteTestConfig(t, root, pod, false, config)
	got, err := listENPUAllocations([]*corev1.Pod{pod}, root, "")
	if err != nil || len(got) != 1 || got[0].CoreQuota != 100 {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestENPUSharedDIEContainersAndInitIndex(t *testing.T) {
	root := t.TempDir()
	pod := enpuTestPod("pod-1", strings.Repeat("a", 64), "die-0")
	pod.Spec.InitContainers = []corev1.Container{{Name: "setup"}}
	second := pod.Spec.Containers[0].DeepCopy()
	second.Name = "second"
	pod.Spec.Containers = append(pod.Spec.Containers, *second)
	status := pod.Status.ContainerStatuses[0]
	status.Name = "second"
	status.ContainerID = "containerd://" + strings.Repeat("b", 64)
	pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, status)
	pod.Annotations[enpuTestResource] = `[{"UUID":"die-0","memory":20480,"core":30},{"UUID":"die-0","memory":20480,"core":30}]`
	pod.Annotations["hami.io/Ascend910C-devices-allocated"] = ";die-0,Ascend910C,20480,30:;die-0,Ascend910C,20480,30:;"
	enpuWriteTestConfig(t, root, pod, false, enpuTestConfig("die-0", 0, 0))
	copy := pod.DeepCopy()
	copy.Spec.Containers = []corev1.Container{*second}
	enpuWriteTestConfig(t, root, copy, false, enpuTestConfig("die-0", 0, 1))
	got, err := listENPUAllocations([]*corev1.Pod{pod}, root, "")
	if err != nil || len(got) != 2 || got[0].VirtualID != 0 || got[1].VirtualID != 1 || got[0].DeviceUUID != got[1].DeviceUUID {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestENPURunningInitSidecar(t *testing.T) {
	root := t.TempDir()
	pod := enpuTestPod("pod-1", strings.Repeat("a", 64), "die-0")
	pod.Spec.InitContainers = pod.Spec.Containers
	pod.Spec.Containers = []corev1.Container{{Name: "application"}}
	pod.Status.InitContainerStatuses = pod.Status.ContainerStatuses
	pod.Status.ContainerStatuses = nil
	pod.Annotations["hami.io/Ascend910C-devices-allocated"] = "die-0,Ascend910C,20480,30:;;"
	writer := pod.DeepCopy()
	writer.Spec.Containers = writer.Spec.InitContainers
	enpuWriteTestConfig(t, root, writer, false, enpuTestConfig("die-0", 0, 0))
	for _, phase := range []corev1.PodPhase{corev1.PodPending, corev1.PodRunning} {
		pod.Status.Phase = phase
		got, err := listENPUAllocations([]*corev1.Pod{pod}, root, "")
		if err != nil || len(got) != 1 || got[0].ContainerName != "vllm" {
			t.Fatalf("phase %s: got %v, %v", phase, got, err)
		}
	}
}

func TestENPUPodObjectNotMutated(t *testing.T) {
	root := t.TempDir()
	pod := enpuTestPod("pod-1", strings.Repeat("a", 64), "die-0")
	before, _ := json.Marshal(pod)
	enpuWriteTestConfig(t, root, pod, false, enpuTestConfig("die-0", 0, 0))
	_, _ = listENPUAllocations([]*corev1.Pod{pod}, root, "")
	after, _ := json.Marshal(pod)
	if string(before) != string(after) {
		t.Fatal("discovery mutated Pod")
	}
}
