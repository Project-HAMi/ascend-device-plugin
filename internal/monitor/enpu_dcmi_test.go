package monitor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"ascend-common/devmanager/common"
	"ascend-common/devmanager/dcmi"
)

type enpuSDKDevice struct {
	logic, physical, card, device int32
	uuid                          string
}

type enpuFakeSDK struct {
	dcmi.DcDriverInterface
	t               *testing.T
	devices         []enpuSDKDevice
	fail            map[string]error
	calls           map[string]int
	processes       map[int32]*common.DevProcessInfo
	utilization     int32
	hbmInfo         *common.HbmInfo
	chipInfo        *common.ChipInfo
	product         string
	beforeProcesses func(int32)
	emptyUUID       bool
}

func newENPUFakeSDK(t *testing.T) *enpuFakeSDK {
	t.Helper()
	return &enpuFakeSDK{
		t:       t,
		devices: []enpuSDKDevice{{logic: 23, physical: 15, card: 7, device: 1, uuid: "die-15"}},
		fail:    map[string]error{}, calls: map[string]int{}, processes: map[int32]*common.DevProcessInfo{},
		utilization: 37, hbmInfo: &common.HbmInfo{MemorySize: 65536, Usage: 1024},
		product: "Ascend910C", chipInfo: &common.ChipInfo{Name: "Ascend910C"},
	}
}

func (f *enpuFakeSDK) logic(id int32) enpuSDKDevice {
	f.t.Helper()
	for _, d := range f.devices {
		if d.logic == id {
			return d
		}
	}
	f.t.Fatalf("SDK received unknown logical ID %d", id)
	return enpuSDKDevice{}
}

func (f *enpuFakeSDK) pair(operation string, card, device int32) enpuSDKDevice {
	f.t.Helper()
	f.calls[operation]++
	for _, d := range f.devices {
		if d.card == card && d.device == device {
			return d
		}
	}
	f.t.Fatalf("%s received incorrect card/device (%d,%d)", operation, card, device)
	return enpuSDKDevice{}
}

func (f *enpuFakeSDK) DcGetLogicIDList() (int32, []int32, error) {
	f.calls["list"]++
	ids := make([]int32, 0, len(f.devices))
	for _, d := range f.devices {
		ids = append(ids, d.logic)
	}
	return int32(len(ids)), ids, f.fail["list"]
}

func (f *enpuFakeSDK) DcGetCardIDDeviceID(id int32) (int32, int32, error) {
	f.calls["card"]++
	d := f.logic(id)
	return d.card, d.device, f.fail["card"]
}

func (f *enpuFakeSDK) DcGetPhysicIDFromLogicID(id int32) (int32, error) {
	f.calls["physical"]++
	return f.logic(id).physical, f.fail["physical"]
}

func (f *enpuFakeSDK) DcGetDieID(card, device int32, kind dcmi.DieType) (string, error) {
	d := f.pair("uuid", card, device)
	if kind != dcmi.VDIE {
		f.t.Fatalf("DIE identity type %v, want VDIE", kind)
	}
	if f.emptyUUID {
		return "", f.fail["uuid"]
	}
	return d.uuid, f.fail["uuid"]
}

func (f *enpuFakeSDK) DcGetProductType(card, device int32) (string, error) {
	f.pair("product", card, device)
	if f.fail["product"] != nil {
		return "", f.fail["product"]
	}
	return f.product, nil
}

func (f *enpuFakeSDK) DcGetChipInfo(card, device int32) (*common.ChipInfo, error) {
	f.pair("chip", card, device)
	return f.chipInfo, f.fail["chip"]
}

func (f *enpuFakeSDK) DcGetDeviceUtilizationRate(card, device int32, kind common.DeviceType) (int32, error) {
	f.pair("utilization", card, device)
	if kind != common.AICore {
		f.t.Fatalf("utilization type %v, want AICore", kind)
	}
	return f.utilization, f.fail["utilization"]
}

func (f *enpuFakeSDK) DcGetDevProcessInfo(card, device int32) (*common.DevProcessInfo, error) {
	d := f.pair("processes", card, device)
	if f.beforeProcesses != nil {
		f.beforeProcesses(d.physical)
	}
	if info, exists := f.processes[d.physical]; exists {
		return info, f.fail["processes"]
	}
	return &common.DevProcessInfo{}, f.fail["processes"]
}

func (f *enpuFakeSDK) hbm(card, device int32) (*common.HbmInfo, error) {
	f.pair("hbm", card, device)
	return f.hbmInfo, f.fail["hbm"]
}

func (f *enpuFakeSDK) DcGetMemoryInfo(int32, int32) (*common.MemoryInfo, error) {
	f.t.Fatal("ENPU collector must not query DDR memory")
	return nil, nil
}

func (f *enpuFakeSDK) DcGetHbmInfo(int32, int32) (*common.HbmInfo, error) {
	f.t.Fatal("ENPU collector must use the injected real HBM function, not SDK stub")
	return nil, nil
}

func enpuDCMIWriteProc(t *testing.T, root string, pid int32, container, start string) {
	t.Helper()
	dir := filepath.Join(root, strconv.Itoa(int(pid)))
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	stat := fmt.Sprintf("%d (a process (name)) S %s%s 0\n", pid, strings.Repeat("0 ", 18), start)
	if err := os.WriteFile(filepath.Join(dir, "stat"), []byte(stat), 0644); err != nil {
		t.Fatal(err)
	}
	cgroup := "0::/system.slice/host.service\n"
	if container != "" {
		cgroup = "0::/system.slice/cri-containerd-" + container + ".scope\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "cgroup"), []byte(cgroup), 0644); err != nil {
		t.Fatal(err)
	}
}

func enpuDCMIWriteProcView(t *testing.T, root string) {
	t.Helper()
	enpuDCMIWriteProc(t, root, 1, "", "1")
	if err := os.Symlink("1", filepath.Join(root, "self")); err != nil {
		t.Fatal(err)
	}
}

func TestReadENPUDeviceStatsMappingAndMemory(t *testing.T) {
	root := t.TempDir()
	first, second := strings.Repeat("a", 64), strings.Repeat("b", 64)
	for pid, container := range map[int32]string{101: first, 102: first, 103: second, 104: ""} {
		enpuDCMIWriteProc(t, root, pid, container, "1234")
	}
	f := newENPUFakeSDK(t)
	f.devices = append(f.devices, enpuSDKDevice{logic: 42, physical: 3, card: 1, device: 0, uuid: "die-3"})
	f.processes[15] = &common.DevProcessInfo{ProcNum: 4, DevProcArray: []common.DevProcInfo{{Pid: 101, MemUsage: 1.5}, {Pid: 102, MemUsage: 2.25}, {Pid: 103, MemUsage: 759.3125}, {Pid: 104, MemUsage: 8}}}
	got, err := readENPUDeviceStats(f, f.hbm, root, []enpuAllocation{{PhysicalID: 15, ContainerID: first}, {PhysicalID: 15, ContainerID: second}})
	if err != nil || len(got) != 2 {
		t.Fatalf("samples %v, err %v", got, err)
	}
	sample := got[0]
	if sample.LogicID != 23 || sample.PhysicalID != 15 || sample.UUID != "die-15" || sample.DeviceType != "Ascend910C" {
		t.Fatalf("wrong identity %+v", sample)
	}
	if sample.MemoryUsed == nil || *sample.MemoryUsed != 1024*1048576 || sample.Utilization == nil || *sample.Utilization != 37 {
		t.Fatalf("wrong host values %+v", sample)
	}
	if sample.MemoryErr != nil || len(sample.MemoryByContainer) != 2 || sample.MemoryByContainer[first] != 3.75*1048576 || sample.MemoryByContainer[second] != 759.3125*1048576 {
		t.Fatalf("wrong container bytes %+v", sample)
	}
	if got[1].PhysicalID != 3 || got[1].LogicID != 42 || got[1].MemoryByContainer != nil {
		t.Fatalf("wrong unrelated DIE sample %+v", got[1])
	}
	if f.calls["processes"] != 1 || f.calls["hbm"] != 2 || f.calls["utilization"] != 2 {
		t.Fatalf("unexpected SDK calls %v", f.calls)
	}
}

func TestReadENPUDeviceStatsNoAllocations(t *testing.T) {
	f := newENPUFakeSDK(t)
	got, err := readENPUDeviceStats(f, f.hbm, filepath.Join(t.TempDir(), "missing-proc"), nil)
	if err != nil || len(got) != 1 || got[0].MemoryUsed == nil || got[0].Utilization == nil {
		t.Fatalf("host-only collection failed: %v, %v", got, err)
	}
	if f.calls["processes"] != 0 || got[0].MemoryByContainer != nil {
		t.Fatalf("host-only collection queried process memory: %v", f.calls)
	}
}

func TestReadENPUDeviceStatsIdentityErrors(t *testing.T) {
	for _, stage := range []string{"list", "card", "physical", "uuid", "empty uuid"} {
		t.Run(stage, func(t *testing.T) {
			f := newENPUFakeSDK(t)
			if stage == "empty uuid" {
				f.emptyUUID = true
			} else {
				f.fail[stage] = errors.New("SDK query failed")
			}
			got, err := readENPUDeviceStats(f, f.hbm, t.TempDir(), nil)
			if err == nil || len(got) != 0 {
				t.Fatalf("invalid identity exported: %v, %v", got, err)
			}
			if f.calls["hbm"] != 0 || f.calls["processes"] != 0 {
				t.Fatalf("read metrics after failed identity: %v", f.calls)
			}
		})
	}
}

func TestReadENPUDeviceStatsUnavailableMetrics(t *testing.T) {
	cases := []struct {
		name                       string
		change                     func(*enpuFakeSDK)
		missingMemory, missingUtil bool
	}{
		{"HBM query error", func(f *enpuFakeSDK) { f.fail["hbm"] = errors.New("HBM unsupported") }, true, false},
		{"nil HBM", func(f *enpuFakeSDK) { f.hbmInfo = nil }, true, false},
		{"zero HBM capacity", func(f *enpuFakeSDK) { f.hbmInfo = &common.HbmInfo{} }, true, false},
		{"HBM usage exceeds capacity", func(f *enpuFakeSDK) { f.hbmInfo = &common.HbmInfo{MemorySize: 1, Usage: 2} }, true, false},
		{"utilization unsupported", func(f *enpuFakeSDK) { f.fail["utilization"] = errors.New("unsupported") }, false, true},
		{"negative utilization", func(f *enpuFakeSDK) { f.utilization = -1 }, false, true},
		{"utilization over 100", func(f *enpuFakeSDK) { f.utilization = 101 }, false, true},
		{"both queries fail", func(f *enpuFakeSDK) {
			f.fail["hbm"] = errors.New("HBM error")
			f.fail["utilization"] = errors.New("utilization error")
		}, true, true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			f := newENPUFakeSDK(t)
			tt.change(f)
			got, err := readENPUDeviceStats(f, f.hbm, t.TempDir(), nil)
			if err == nil || len(got) != 1 {
				t.Fatalf("samples %v, err %v", got, err)
			}
			if (got[0].MemoryUsed == nil) != tt.missingMemory || (got[0].Utilization == nil) != tt.missingUtil {
				t.Fatalf("unavailable metric converted to zero: %+v", got[0])
			}
		})
	}
}

func TestReadENPUDeviceStatsValidZeroAndProductFallback(t *testing.T) {
	f := newENPUFakeSDK(t)
	f.hbmInfo.Usage = 0
	f.utilization = 0
	f.fail["product"] = errors.New("unknown product")
	got, err := readENPUDeviceStats(f, f.hbm, t.TempDir(), nil)
	if err != nil || len(got) != 1 || got[0].DeviceType != "Ascend910C" || got[0].MemoryUsed == nil || *got[0].MemoryUsed != 0 || got[0].Utilization == nil || *got[0].Utilization != 0 {
		t.Fatalf("valid zero lost: %v, %v", got, err)
	}
}

func TestReadENPUDeviceStatsProductFallback(t *testing.T) {
	cases := []struct {
		name      string
		change    func(*enpuFakeSDK)
		want      string
		chipCalls int
		fail      bool
	}{
		{"product supported", func(f *enpuFakeSDK) { f.chipInfo = nil }, "Ascend910C", 0, false},
		{"product query error", func(f *enpuFakeSDK) { f.fail["product"] = errors.New("-8255"); f.chipInfo.Name = "Ascend910_9392" }, "Ascend910_9392", 1, false},
		{"empty product", func(f *enpuFakeSDK) { f.product = "" }, "Ascend910C", 1, false},
		{"whitespace product", func(f *enpuFakeSDK) { f.product = " \t"; f.chipInfo.Name = " Ascend910C " }, "Ascend910C", 1, false},
		{"both query errors", func(f *enpuFakeSDK) {
			f.fail["product"] = errors.New("-8255")
			f.fail["chip"] = errors.New("chip unsupported")
		}, "", 1, true},
		{"nil chip", func(f *enpuFakeSDK) { f.product = ""; f.chipInfo = nil }, "", 1, true},
		{"empty chip name", func(f *enpuFakeSDK) { f.fail["product"] = errors.New("-8255"); f.chipInfo.Name = "" }, "", 1, true},
		{"whitespace chip name", func(f *enpuFakeSDK) { f.product = ""; f.chipInfo.Name = " \t" }, "", 1, true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			f := newENPUFakeSDK(t)
			tt.change(f)
			got, err := readENPUDeviceStats(f, f.hbm, t.TempDir(), nil)
			if (err != nil) != tt.fail || len(got) != 1 || got[0].DeviceType != tt.want {
				t.Fatalf("device type sample %v, err %v", got, err)
			}
			if f.calls["chip"] != tt.chipCalls || got[0].MemoryUsed == nil || got[0].Utilization == nil {
				t.Fatalf("fallback calls or independent metrics incorrect: %+v, %v", got[0], f.calls)
			}
		})
	}
}

func TestReadENPUDeviceStatsProcessErrors(t *testing.T) {
	for _, kind := range []string{"query error", "nil process list", "missing proc root", "missing cgroup", "malformed cgroup", "missing stat", "malformed stat", "PID appeared", "PID reused"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			id := strings.Repeat("a", 64)
			enpuDCMIWriteProc(t, root, 100, id, "1234")
			f := newENPUFakeSDK(t)
			f.processes[15] = &common.DevProcessInfo{ProcNum: 1, DevProcArray: []common.DevProcInfo{{Pid: 100, MemUsage: 247.3125}}}
			switch kind {
			case "query error":
				f.fail["processes"] = errors.New("process query unsupported")
			case "nil process list":
				f.processes[15] = nil
			case "missing proc root":
				root = filepath.Join(root, "missing")
			case "missing cgroup":
				if err := os.Remove(filepath.Join(root, "100", "cgroup")); err != nil {
					t.Fatal(err)
				}
			case "malformed cgroup":
				if err := os.WriteFile(filepath.Join(root, "100", "cgroup"), []byte("invalid cgroup\n"), 0644); err != nil {
					t.Fatal(err)
				}
			case "missing stat":
				f.beforeProcesses = func(int32) {
					if err := os.Remove(filepath.Join(root, "100", "stat")); err != nil {
						t.Fatal(err)
					}
				}
			case "malformed stat":
				f.beforeProcesses = func(int32) {
					if err := os.WriteFile(filepath.Join(root, "100", "stat"), []byte("invalid stat\n"), 0644); err != nil {
						t.Fatal(err)
					}
				}
			case "PID appeared":
				f.beforeProcesses = func(int32) { enpuDCMIWriteProc(t, root, 101, id, "1234") }
				f.processes[15].DevProcArray[0].Pid = 101
			case "PID reused":
				f.beforeProcesses = func(int32) { enpuDCMIWriteProc(t, root, 100, id, "5678") }
			}
			got, err := readENPUDeviceStats(f, f.hbm, root, []enpuAllocation{{PhysicalID: 15}})
			if err == nil || len(got) != 1 || got[0].MemoryErr == nil || got[0].MemoryByContainer != nil {
				t.Fatalf("failed attribution exported as zero: %v, %v", got, err)
			}
			if got[0].MemoryUsed == nil || got[0].Utilization == nil {
				t.Fatalf("process failure erased independent host metrics: %+v", got[0])
			}
			if kind == "missing proc root" && f.calls["processes"] != 0 {
				t.Fatal("queried PID list without initial process snapshot")
			}
		})
	}
}

func TestReadENPUDeviceStatsExitedProcess(t *testing.T) {
	for _, phase := range []string{"before snapshot", "after snapshot"} {
		t.Run(phase, func(t *testing.T) {
			root := t.TempDir()
			enpuDCMIWriteProcView(t, root)
			live, exited := strings.Repeat("a", 64), strings.Repeat("b", 64)
			enpuDCMIWriteProc(t, root, 100, exited, "1234")
			enpuDCMIWriteProc(t, root, 101, live, "1235")
			enpuDCMIWriteProc(t, root, 102, live, "1236")
			f := newENPUFakeSDK(t)
			f.processes[15] = &common.DevProcessInfo{ProcNum: 3, DevProcArray: []common.DevProcInfo{
				{Pid: 101, MemUsage: 1.5}, {Pid: 100, MemUsage: 247.3125}, {Pid: 102, MemUsage: 2.25},
			}}
			exit := func(int32) {
				if err := os.RemoveAll(filepath.Join(root, "100")); err != nil {
					t.Fatal(err)
				}
			}
			if phase == "before snapshot" {
				exit(15)
			} else {
				f.beforeProcesses = exit
			}
			got, err := readENPUDeviceStats(f, f.hbm, root, []enpuAllocation{{PhysicalID: 15, ContainerID: live}})
			if err != nil || len(got) != 1 || got[0].MemoryErr != nil {
				t.Fatalf("normal exit failed the DIE sample: %v, %v", got, err)
			}
			if len(got[0].MemoryByContainer) != 1 || got[0].MemoryByContainer[live] != 3.75*common.UnitMB {
				t.Fatalf("live memory was lost or stale memory retained: %v", got[0].MemoryByContainer)
			}
		})
	}
}

func TestENPUContainerMemoryExitAfterCgroupRead(t *testing.T) {
	root := t.TempDir()
	enpuDCMIWriteProcView(t, root)
	id := strings.Repeat("a", 64)
	enpuDCMIWriteProc(t, root, 100, id, "1234")
	enpuDCMIWriteProc(t, root, 101, id, "1235")
	starts, err := enpuProcessStarts(root)
	if err != nil {
		t.Fatal(err)
	}
	cgroup := filepath.Join(root, "100", "cgroup")
	if err := os.Remove(cgroup); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(cgroup, 0600); err != nil {
		t.Fatal(err)
	}
	written := make(chan error, 1)
	go func() {
		f, err := os.OpenFile(cgroup, os.O_WRONLY, 0600)
		if err != nil {
			written <- err
			return
		}
		defer func() {
			_ = f.Close()
		}()
		if err := os.RemoveAll(filepath.Join(root, "100")); err != nil {
			written <- err
			return
		}
		_, err = fmt.Fprintf(f, "0::/system.slice/docker-%s.scope\n", id)
		written <- err
	}()
	info := &common.DevProcessInfo{ProcNum: 2, DevProcArray: []common.DevProcInfo{{Pid: 100, MemUsage: 8}, {Pid: 101, MemUsage: 2}}}
	got, err := enpuContainerMemory(info, root, starts)
	if writeErr := <-written; writeErr != nil {
		t.Fatal(writeErr)
	}
	if err != nil || len(got) != 1 || got[id] != 2*common.UnitMB {
		t.Fatalf("exit between cgroup and stat reads lost live memory: %v, %v", got, err)
	}
}

func TestReadENPUDeviceStatsLostProcView(t *testing.T) {
	for _, emptyMountpoint := range []bool{false, true} {
		t.Run(fmt.Sprintf("empty_mountpoint=%t", emptyMountpoint), func(t *testing.T) {
			root := t.TempDir()
			enpuDCMIWriteProcView(t, root)
			enpuDCMIWriteProc(t, root, 100, strings.Repeat("a", 64), "1234")
			f := newENPUFakeSDK(t)
			f.processes[15] = &common.DevProcessInfo{ProcNum: 1, DevProcArray: []common.DevProcInfo{{Pid: 100, MemUsage: 8}}}
			f.beforeProcesses = func(int32) {
				if err := os.RemoveAll(root); err != nil {
					t.Fatal(err)
				}
				if emptyMountpoint {
					if err := os.Mkdir(root, 0755); err != nil {
						t.Fatal(err)
					}
				}
			}
			got, err := readENPUDeviceStats(f, f.hbm, root, []enpuAllocation{{PhysicalID: 15}})
			if err == nil || len(got) != 1 || got[0].MemoryErr == nil || got[0].MemoryByContainer != nil {
				t.Fatalf("unavailable proc view exported as zero: %v, %v", got, err)
			}
		})
	}
}

func TestENPUProcessMissingErrors(t *testing.T) {
	for _, tc := range []struct {
		err  error
		gone bool
	}{
		{syscall.ENOENT, true},
		{syscall.ESRCH, true},
		{syscall.EACCES, false},
		{syscall.EPERM, false},
		{syscall.ENOTDIR, false},
		{errors.New("malformed process data"), false},
	} {
		t.Run(tc.err.Error(), func(t *testing.T) {
			err := fmt.Errorf("process read: %w", &os.PathError{Op: "read", Path: "proc/100/stat", Err: tc.err})
			if got := enpuProcessMissing(err); got != tc.gone {
				t.Fatalf("missing process classification for %v: %t", err, got)
			}
		})
	}
}

func TestReadENPUDeviceStatsOneProcessSnapshot(t *testing.T) {
	root := t.TempDir()
	id := strings.Repeat("a", 64)
	enpuDCMIWriteProc(t, root, 100, id, "1234")
	f := newENPUFakeSDK(t)
	f.devices = append(f.devices, enpuSDKDevice{logic: 42, physical: 3, card: 1, device: 0, uuid: "die-3"})
	f.processes[15] = &common.DevProcessInfo{ProcNum: 1, DevProcArray: []common.DevProcInfo{{Pid: 100, MemUsage: 1}}}
	f.processes[3] = &common.DevProcessInfo{ProcNum: 1, DevProcArray: []common.DevProcInfo{{Pid: 101, MemUsage: 2}}}
	f.beforeProcesses = func(physical int32) {
		if physical == 15 {
			enpuDCMIWriteProc(t, root, 101, id, "5678")
		}
	}
	got, err := readENPUDeviceStats(f, f.hbm, root, []enpuAllocation{{PhysicalID: 15}, {PhysicalID: 3}})
	if err == nil || len(got) != 2 || got[0].MemoryErr != nil || got[1].MemoryErr == nil || got[1].MemoryByContainer != nil {
		t.Fatalf("expected one snapshot before all DIE queries: %v, %v", got, err)
	}
	if got[0].MemoryByContainer[id] != 1048576 || f.calls["processes"] != 2 {
		t.Fatalf("independent valid sample was lost: %v", got)
	}
}
