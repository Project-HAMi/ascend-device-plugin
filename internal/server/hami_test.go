package server

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/Project-HAMi/HAMi/pkg/device"
	"github.com/Project-HAMi/ascend-device-plugin/internal/manager"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

type hamiPolicyFakeManager struct {
	*FakeManager
	policy string
}

func (m *hamiPolicyFakeManager) HamiCorePolicy() string { return m.policy }

func hamiTestInt64(value int64) *int64 { return &value }
func hamiTestInt32(value int32) *int32 { return &value }

func hamiTestServer(t *testing.T) (*PluginServer, map[string]*manager.Device) {
	t.Helper()
	original := hostHookPath
	hostHookPath = t.TempDir()
	t.Cleanup(func() { hostHookPath = original })
	devices := map[string]*manager.Device{
		"card-seven": {UUID: "card-seven", LogicID: 3, PhyID: 7, CardID: 4, Memory: 65536},
		"card-two":   {UUID: "card-two", LogicID: 1, PhyID: 2, CardID: 2, Memory: 32768},
	}
	mgr := &FakeManager{
		GetDeviceByUUIDFunc: func(uuid string) *manager.Device { return devices[uuid] },
		IsHamiVnpuCoreFunc:  func() bool { return true },
	}
	return &PluginServer{mgr: mgr, allocAnno: "huawei.com/Ascend910B3"}, devices
}

func hamiTestPod(uid string) *v1.Pod {
	return &v1.Pod{ObjectMeta: metav1.ObjectMeta{
		UID: types.UID(uid), Name: uid, Namespace: "test",
	}}
}

func hamiTestMount(t *testing.T, response *v1beta1.ContainerAllocateResponse, containerPath string) *v1beta1.Mount {
	t.Helper()
	var found *v1beta1.Mount
	for _, mount := range response.Mounts {
		if mount.ContainerPath == containerPath {
			if found != nil {
				t.Fatalf("multiple mounts target %q", containerPath)
			}
			found = mount
		}
	}
	if found == nil {
		t.Fatalf("missing mount for %q: %+v", containerPath, response.Mounts)
	}
	return found
}

func TestHamiCoreAllocateCanonicalEnvironmentAndMounts(t *testing.T) {
	ps, _ := hamiTestServer(t)
	pod := hamiTestPod("pod-a") // Node default selects hami-core; no Pod policy override.
	response, err := ps.buildContainerAllocateResponse(pod, "worker", device.ContainerDevices{
		{UUID: "card-seven"},
	}, map[string]RuntimeInfo{
		"card-seven": {Memory: hamiTestInt64(2048), Core: hamiTestInt32(31)},
	})
	if err != nil {
		t.Fatal(err)
	}
	wantEnvs := map[string]string{
		"ASCEND_VISIBLE_DEVICES": "7",
		"NPU_MEMORY_LIMIT_MB":    "2048",
		"NPU_COMPUTE_LIMIT":      "31",
		"NPU_SCHED_POLICY":       "spatial",
		"NPU_GLOBAL_SHM_PATH":    "/hami-shared-region/7_global_registry",
		"NPU_LOCAL_SHM_PATH":     "/hami-vnpu-shmem/vnpu_local_shmem",
	}
	if !reflect.DeepEqual(response.Envs, wantEnvs) {
		t.Fatalf("injected environment = %#v, want %#v", response.Envs, wantEnvs)
	}
	for _, legacy := range []string{"NPU_MEM_QUOTA", "NPU_PRIORITY", "NPU_FIXED_SHARE_RATIO"} {
		if _, exists := response.Envs[legacy]; exists {
			t.Errorf("migration-rejected legacy variable %s was injected", legacy)
		}
	}
	wantMounts := map[string]struct {
		host     string
		readOnly bool
	}{
		"/usr/local/bin/npu-smi":                {"/usr/local/bin/npu-smi", true},
		"/etc/ascend_install.info":              {"/etc/ascend_install.info", true},
		"/usr/local/Ascend/driver/lib64/driver": {"/usr/local/Ascend/driver/lib64/driver", true},
		"/usr/local/Ascend/driver/version.info": {"/usr/local/Ascend/driver/version.info", true},
		"/hami-vnpu-core":                       {hostHookPath, true},
		"/etc/ld.so.preload":                    {filepath.Join(hostHookPath, "ld.so.preload"), true},
		"/hami-shared-region":                   {"/usr/local/hami-shared-region", false},
		"/hami-vnpu-shmem":                      {filepath.Join(hostHookPath, "containers", "pod-a_worker"), false},
	}
	if len(response.Mounts) != len(wantMounts) {
		t.Fatalf("got %d mounts, want %d", len(response.Mounts), len(wantMounts))
	}
	for target, want := range wantMounts {
		mount := hamiTestMount(t, response, target)
		if mount.HostPath != want.host || mount.ReadOnly != want.readOnly {
			t.Errorf("mount %s = %+v, want host=%s readOnly=%v", target, mount, want.host, want.readOnly)
		}
	}
	localDir := hamiTestMount(t, response, "/hami-vnpu-shmem").HostPath
	info, err := os.Stat(localDir)
	if err != nil || !info.IsDir() {
		t.Fatalf("local SHM directory not created: info=%v, err=%v", info, err)
	}
	if info.Mode().Perm() != 0777 {
		t.Errorf("local SHM mode = %o, want 777", info.Mode().Perm())
	}
}

func TestHamiCoreAllocatePolicyPrecedence(t *testing.T) {
	for _, test := range []struct {
		name          string
		managerPolicy *string
		podPolicy     string
		want          string
		wantErr       bool
	}{
		{name: "optional manager interface absent", want: "spatial"},
		{name: "empty manager default", managerPolicy: new(string), want: "spatial"},
		{name: "manager fixed", managerPolicy: hamiTestString("fixed"), want: "fixed"},
		{name: "manager dynamic", managerPolicy: hamiTestString("dynamic"), want: "dynamic"},
		{name: "manager overdraft", managerPolicy: hamiTestString("overdraft"), want: "overdraft"},
		{name: "manager spatial", managerPolicy: hamiTestString("spatial"), want: "spatial"},
		{name: "pod fixed", managerPolicy: hamiTestString("spatial"), podPolicy: "fixed", want: "fixed"},
		{name: "pod dynamic", managerPolicy: hamiTestString("spatial"), podPolicy: "dynamic", want: "dynamic"},
		{name: "pod overdraft", managerPolicy: hamiTestString("spatial"), podPolicy: "overdraft", want: "overdraft"},
		{name: "pod spatial", managerPolicy: hamiTestString("overdraft"), podPolicy: "spatial", want: "spatial"},
		{name: "normalizes pod policy", podPolicy: "  SpAtIaL  ", want: "spatial"},
		{name: "blank pod inherits manager", managerPolicy: hamiTestString("overdraft"), podPolicy: " \t", want: "overdraft"},
		{name: "invalid pod fails instead of fallback", managerPolicy: hamiTestString("spatial"), podPolicy: "elastic", wantErr: true},
		{name: "invalid manager fails", managerPolicy: hamiTestString("fixed-share"), wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			ps, _ := hamiTestServer(t)
			if test.managerPolicy != nil {
				ps.mgr = &hamiPolicyFakeManager{FakeManager: ps.mgr.(*FakeManager), policy: *test.managerPolicy}
			}
			pod := hamiTestPod("policy-pod")
			pod.Annotations = map[string]string{
				VNPUModeAnnotation:       VNPUModeHamiCore,
				HamiCorePolicyAnnotation: test.podPolicy,
			}
			response, err := ps.buildContainerAllocateResponse(pod, "worker", device.ContainerDevices{
				{UUID: "card-seven"},
			}, map[string]RuntimeInfo{"card-seven": {}})
			if test.wantErr {
				if err == nil || response != nil {
					t.Fatalf("invalid policy returned response=%+v, err=%v", response, err)
				}
				if !strings.Contains(err.Error(), HamiCorePolicyAnnotation) {
					t.Errorf("policy error does not identify Pod setting: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := response.Envs["NPU_SCHED_POLICY"]; got != test.want {
				t.Errorf("policy = %q, want %q", got, test.want)
			}
		})
	}
}

func hamiTestString(value string) *string { return &value }

func TestHamiCoreAllocateQuotaValidation(t *testing.T) {
	maxMemoryMiB := int64(math.MaxUint64 / (1024 * 1024))
	for _, test := range []struct {
		name       string
		lookup     map[string]RuntimeInfo
		secondCard bool
		deviceMem  *int64
		wantMemory int64
		wantCore   int32
		wantErr    string
	}{
		{name: "missing core defaults to whole card", lookup: map[string]RuntimeInfo{"card-seven": {Memory: hamiTestInt64(1024)}}, wantMemory: 1024, wantCore: 100},
		{name: "zero core defaults to whole card", lookup: map[string]RuntimeInfo{"card-seven": {Core: hamiTestInt32(0)}}, wantMemory: 65536, wantCore: 100},
		{name: "minimum core", lookup: map[string]RuntimeInfo{"card-seven": {Core: hamiTestInt32(1)}}, wantMemory: 65536, wantCore: 1},
		{name: "maximum core", lookup: map[string]RuntimeInfo{"card-seven": {Core: hamiTestInt32(100)}}, wantMemory: 65536, wantCore: 100},
		{name: "negative core", lookup: map[string]RuntimeInfo{"card-seven": {Core: hamiTestInt32(-1)}}, wantErr: "outside 1..100"},
		{name: "core exceeds percentage base", lookup: map[string]RuntimeInfo{"card-seven": {Core: hamiTestInt32(101)}}, wantErr: "outside 1..100"},
		{name: "missing memory falls back to device", lookup: map[string]RuntimeInfo{"card-seven": {}}, wantMemory: 65536, wantCore: 100},
		{name: "zero memory falls back to device", lookup: map[string]RuntimeInfo{"card-seven": {Memory: hamiTestInt64(0)}}, wantMemory: 65536, wantCore: 100},
		{name: "negative memory", lookup: map[string]RuntimeInfo{"card-seven": {Memory: hamiTestInt64(-1)}}, wantErr: "must not be negative"},
		{name: "missing runtime allocation", lookup: map[string]RuntimeInfo{}, wantErr: "missing HAMi runtime allocation"},
		{name: "nil runtime lookup", wantErr: "missing HAMi runtime allocation"},
		{name: "missing memory and no device capacity", lookup: map[string]RuntimeInfo{"card-seven": {}}, deviceMem: hamiTestInt64(0), wantErr: "positive HAMi memory quota"},
		{name: "negative fallback device capacity", lookup: map[string]RuntimeInfo{"card-seven": {}}, deviceMem: hamiTestInt64(-1), wantErr: "positive HAMi memory quota"},
		{name: "maximum representable byte count", lookup: map[string]RuntimeInfo{"card-seven": {Memory: hamiTestInt64(maxMemoryMiB)}}, wantMemory: maxMemoryMiB, wantCore: 100},
		{name: "single card byte overflow", lookup: map[string]RuntimeInfo{"card-seven": {Memory: hamiTestInt64(maxMemoryMiB + 1)}}, wantErr: "runtime byte count"},
		{name: "different per-card memory sums", secondCard: true, lookup: map[string]RuntimeInfo{
			"card-seven": {Memory: hamiTestInt64(1024), Core: hamiTestInt32(33)},
			"card-two":   {Memory: hamiTestInt64(2048), Core: hamiTestInt32(33)},
		}, wantMemory: 3072, wantCore: 33},
		{name: "fallback memory sums per selected device", secondCard: true, lookup: map[string]RuntimeInfo{
			"card-seven": {}, "card-two": {},
		}, wantMemory: 98304, wantCore: 100},
		{name: "mixed missing and zero cores share full quota", secondCard: true, lookup: map[string]RuntimeInfo{
			"card-seven": {}, "card-two": {Core: hamiTestInt32(0)},
		}, wantMemory: 98304, wantCore: 100},
		{name: "different per-card compute rejected", secondCard: true, lookup: map[string]RuntimeInfo{
			"card-seven": {Core: hamiTestInt32(10)}, "card-two": {Core: hamiTestInt32(30)},
		}, wantErr: "same compute quota"},
		{name: "missing second runtime allocation", secondCard: true, lookup: map[string]RuntimeInfo{
			"card-seven": {},
		}, wantErr: "missing HAMi runtime allocation for card-two"},
		{name: "combined byte overflow", secondCard: true, lookup: map[string]RuntimeInfo{
			"card-seven": {Memory: hamiTestInt64(maxMemoryMiB)}, "card-two": {Memory: hamiTestInt64(1)},
		}, wantErr: "runtime byte count"},
		{name: "combined signed quota overflow", secondCard: true, lookup: map[string]RuntimeInfo{
			"card-seven": {Memory: hamiTestInt64(1)}, "card-two": {Memory: hamiTestInt64(math.MaxInt64)},
		}, wantErr: "overflows int64"},
	} {
		t.Run(test.name, func(t *testing.T) {
			ps, devices := hamiTestServer(t)
			if test.deviceMem != nil {
				devices["card-seven"].Memory = *test.deviceMem
			}
			allocated := device.ContainerDevices{{UUID: "card-seven"}}
			visible := "7"
			if test.secondCard {
				allocated = append(allocated, device.ContainerDevice{UUID: "card-two"})
				visible = "7,2"
			}
			response, err := ps.buildContainerAllocateResponse(hamiTestPod("quota-pod"), "worker", allocated, test.lookup)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) || response != nil {
					t.Fatalf("got response=%+v, err=%v; want no response and error containing %q", response, err, test.wantErr)
				}
				if _, statErr := os.Stat(filepath.Join(hostHookPath, "containers")); !os.IsNotExist(statErr) {
					t.Errorf("invalid quota created local SHM state: stat err=%v", statErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			for key, want := range map[string]string{
				"NPU_MEMORY_LIMIT_MB":    strconv.FormatInt(test.wantMemory, 10),
				"NPU_COMPUTE_LIMIT":      strconv.FormatInt(int64(test.wantCore), 10),
				"ASCEND_VISIBLE_DEVICES": visible,
				"NPU_GLOBAL_SHM_PATH":    "/hami-shared-region/7_global_registry",
			} {
				if got := response.Envs[key]; got != want {
					t.Errorf("%s = %q, want %q", key, got, want)
				}
			}
		})
	}
}

func TestHamiCoreAllocateLocalSHMIsolation(t *testing.T) {
	ps, _ := hamiTestServer(t)
	allocate := func(uid, container string) string {
		t.Helper()
		response, err := ps.buildContainerAllocateResponse(hamiTestPod(uid), container,
			device.ContainerDevices{{UUID: "card-seven"}}, map[string]RuntimeInfo{"card-seven": {}})
		if err != nil {
			t.Fatal(err)
		}
		return hamiTestMount(t, response, "/hami-vnpu-shmem").HostPath
	}
	directories := []string{allocate("pod-a", "worker"), allocate("pod-a", "sidecar"), allocate("pod-b", "worker")}
	seen := make(map[string]bool)
	for _, dir := range directories {
		if seen[dir] {
			t.Fatalf("different Pod/container allocations share host SHM directory %s", dir)
		}
		seen[dir] = true
		if err := os.WriteFile(filepath.Join(dir, "vnpu_local_shmem"), []byte("old state"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if got := allocate("pod-a", "worker"); got != directories[0] {
		t.Fatalf("reallocation moved local SHM from %q to %q", directories[0], got)
	}
	if _, err := os.Stat(filepath.Join(directories[0], "vnpu_local_shmem")); !os.IsNotExist(err) {
		t.Errorf("reallocation retained stale local SHM: %v", err)
	}
	for _, dir := range directories[1:] {
		data, err := os.ReadFile(filepath.Join(dir, "vnpu_local_shmem"))
		if err != nil || string(data) != "old state" {
			t.Errorf("reallocation damaged another allocation %s: data=%q err=%v", dir, data, err)
		}
	}
}

func TestHamiCoreAllocateLocalSHMSetupFailure(t *testing.T) {
	ps, _ := hamiTestServer(t)
	// A regular file blocks directory construction even when tests run as root.
	if err := os.WriteFile(filepath.Join(hostHookPath, "containers"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	response, err := ps.buildContainerAllocateResponse(hamiTestPod("blocked-pod"), "worker",
		device.ContainerDevices{{UUID: "card-seven"}}, map[string]RuntimeInfo{"card-seven": {}})
	if err == nil || response != nil || !strings.Contains(err.Error(), "HAMi local SHM directory") {
		t.Fatalf("unusable SHM directory returned response=%+v err=%v", response, err)
	}
	data, readErr := os.ReadFile(filepath.Join(hostHookPath, "containers"))
	if readErr != nil || string(data) != "keep" {
		t.Errorf("failed allocation modified obstructing file: data=%q err=%v", data, readErr)
	}
}

func TestHamiCoreAllocateRejectsRelativeHookPath(t *testing.T) {
	ps, _ := hamiTestServer(t)
	hostHookPath = "relative-core-assets"
	response, err := ps.buildContainerAllocateResponse(hamiTestPod("relative-path-pod"), "worker",
		device.ContainerDevices{{UUID: "card-seven"}}, map[string]RuntimeInfo{"card-seven": {}})
	if err == nil || response != nil || !strings.Contains(err.Error(), "HOOK_PATH must be absolute") {
		t.Fatalf("relative hook path returned response=%+v err=%v", response, err)
	}
}
