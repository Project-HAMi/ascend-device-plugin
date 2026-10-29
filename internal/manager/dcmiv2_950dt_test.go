package manager

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"ascend-common/devmanager/common"
	"github.com/Project-HAMi/ascend-device-plugin/internal"
)

type fake950DTAPI struct {
	ids             []int32
	chips           map[int32]common.ChipInfo
	phys            map[int32]int32
	slots           map[int32][2]int32
	dies            map[int32][ascend950DTDieIDCount]uint32
	health          map[int32]uint32
	healthErr       map[int32]error
	hbm             map[int32]*common.HbmInfo
	countOverride   int32
	phyReverse      map[int32]int32
	slotReverse     map[[2]int32]int32
	deviceListCalls int
}

func newFake950DTAPI() *fake950DTAPI {
	f := &fake950DTAPI{
		ids:         []int32{0, 1},
		chips:       map[int32]common.ChipInfo{},
		phys:        map[int32]int32{0: 0, 1: 1},
		slots:       map[int32][2]int32{0: {0, 0}, 1: {1, 0}},
		dies:        map[int32][ascend950DTDieIDCount]uint32{0: {1, 2, 3, 4, 5}, 1: {1, 2, 3, 4, 6}},
		health:      map[int32]uint32{0: 0, 1: 0},
		healthErr:   map[int32]error{},
		hbm:         map[int32]*common.HbmInfo{},
		phyReverse:  map[int32]int32{},
		slotReverse: map[[2]int32]int32{},
	}
	for _, id := range f.ids {
		f.chips[id] = common.ChipInfo{Type: "Ascend", Name: ascend950DTName, NpuName: ascend950DTNPUName, AICoreCnt: 32}
		f.hbm[id] = &common.HbmInfo{MemorySize: 98304, Usage: 1024}
		f.phyReverse[f.phys[id]] = id
		f.slotReverse[f.slots[id]] = id
	}
	return f
}

func (f *fake950DTAPI) init() error { return nil }
func (f *fake950DTAPI) allDeviceCount() (int32, error) {
	if f.countOverride != 0 {
		return f.countOverride, nil
	}
	return int32(len(f.ids)), nil
}
func (f *fake950DTAPI) deviceList() ([]int32, error) {
	f.deviceListCalls++
	return append([]int32(nil), f.ids...), nil
}
func (f *fake950DTAPI) chipInfo(id int32) (common.ChipInfo, error) { return f.chips[id], nil }
func (f *fake950DTAPI) deviceHealth(id int32) (uint32, error) {
	if err := f.healthErr[id]; err != nil {
		return 0, err
	}
	return f.health[id], nil
}
func (f *fake950DTAPI) deviceDDIE(id int32) ([ascend950DTDieIDCount]uint32, error) {
	return f.dies[id], nil
}
func (f *fake950DTAPI) chipPhyID(id int32) (int32, error) { return f.phys[id], nil }
func (f *fake950DTAPI) deviceIDByChipPhyID(phy int32) (int32, error) {
	if id, ok := f.phyReverse[phy]; ok {
		return id, nil
	}
	return 0, fmt.Errorf("unknown physical ID %d", phy)
}
func (f *fake950DTAPI) slotChipID(id int32) (int32, int32, error) {
	value, ok := f.slots[id]
	if !ok {
		return 0, 0, fmt.Errorf("unknown logic ID %d", id)
	}
	return value[0], value[1], nil
}
func (f *fake950DTAPI) deviceIDBySlotChipID(slot, chip int32) (int32, error) {
	if id, ok := f.slotReverse[[2]int32{slot, chip}]; ok {
		return id, nil
	}
	return 0, fmt.Errorf("unknown slot/chip %d/%d", slot, chip)
}
func (f *fake950DTAPI) hbmInfo(id int32) (*common.HbmInfo, error) {
	info, ok := f.hbm[id]
	if !ok {
		return nil, errors.New("missing HBM info")
	}
	copy := *info
	return &copy, nil
}

func newFake950DTManager(t *testing.T, api *fake950DTAPI) *AscendManager {
	t.Helper()
	v2Mgr, err := newAscend950DTManagerWithAPI(api)
	if err != nil {
		t.Fatalf("newAscend950DTManagerWithAPI() error = %v", err)
	}
	return &AscendManager{v2Mgr: v2Mgr, devs: []*Device{}}
}

func config950DT(mode, templates string, memoryCapacity int, aiCore int) string {
	return fmt.Sprintf("vnpus:\n  hamiVnpuMode: %s\n  configs:\n  - chipName: Ascend950DT\n    commonWord: Ascend950DT\n    resourceName: huawei.com/Ascend950DT\n    memoryAllocatable: %d\n    memoryCapacity: %d\n    aiCore: %d\n    templates: %s\n", mode, memoryCapacity, memoryCapacity, aiCore, templates)
}

func TestFormatDDIEIsStableAndCanonical(t *testing.T) {
	got := formatDDIE([ascend950DTDieIDCount]uint32{0x7777777b, 0x20660000, 0xcf0a2f1a, 0xe1d4d90a, 0x7e0b6a43})
	want := "DDIE-7777777b-20660000-cf0a2f1a-e1d4d90a-7e0b6a43"
	if got != want {
		t.Fatalf("formatDDIE() = %q, want %q", got, want)
	}
}

func TestAscend950DTDiscoveryRejectsUnsafeIdentityAndMappings(t *testing.T) {
	tests := []struct {
		name   string
		modify func(*fake950DTAPI)
		want   string
	}{
		{"duplicate DDIE", func(f *fake950DTAPI) { f.dies[1] = f.dies[0] }, "duplicate DDIE"},
		{"wrong physical reverse map", func(f *fake950DTAPI) { f.phyReverse[1] = 0 }, "unsafe logic/physical mapping"},
		{"wrong slot reverse map", func(f *fake950DTAPI) { f.slotReverse[[2]int32{1, 0}] = 0 }, "unsafe logic/slot mapping"},
		{"different chip family", func(f *fake950DTAPI) { info := f.chips[1]; info.Name = "other"; f.chips[1] = info }, "not Ascend950DT"},
		{"count/list mismatch", func(f *fake950DTAPI) { f.countOverride = 3 }, "count/list mismatch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := newFake950DTAPI()
			tt.modify(api)
			_, err := newAscend950DTManagerWithAPI(api)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("newAscend950DTManagerWithAPI() error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestAscend950DTAdditionalSKUWithMatchingConfiguration(t *testing.T) {
	api := newFake950DTAPI()
	for _, id := range api.ids {
		chip := api.chips[id]
		chip.NpuName = "9581"
		api.chips[id] = chip
		api.hbm[id].MemorySize = 147456
	}
	am := newFake950DTManager(t, api)
	if err := am.LoadConfig(writeConfig(t, config950DT("template", "[]", 147456, 32))); err != nil {
		t.Fatalf("LoadConfig() with simulated 9581: %v", err)
	}
	if err := am.UpdateDevice(); err != nil {
		t.Fatalf("UpdateDevice() with simulated 9581: %v", err)
	}
}

func TestAscend950DTLoadConfigEnforcesWholeCardAndCapacity(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		memory    uint64
		wantError string
	}{
		{"whole-card config", config950DT("template", "[]", 98304, 32), 98304, ""},
		{"slicing mode", config950DT("hami-core", "[]", 98304, 32), 98304, "full-card allocation only"},
		{"vNPU templates", config950DT("template", "[{name: vir01, memory: 8192}]", 98304, 32), 98304, "templates must be empty"},
		{"HBM mismatch", config950DT("template", "[]", 65536, 32), 98304, "does not match configured memoryCapacity"},
		{"AICore mismatch", config950DT("template", "[]", 98304, 20), 98304, "does not match DCMI v2 chip info"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := newFake950DTAPI()
			for _, id := range api.ids {
				info := *api.hbm[id]
				info.MemorySize = tt.memory
				api.hbm[id] = &info
			}
			am := newFake950DTManager(t, api)
			err := am.LoadConfig(writeConfig(t, tt.content))
			if err == nil {
				err = am.ValidateWholeCardMode()
			}
			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("LoadConfig()/ValidateWholeCardMode() error = %v", err)
				}
				if got := am.VDeviceCount(); got != 1 {
					t.Fatalf("VDeviceCount() = %d, want 1", got)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("LoadConfig() error = %v, want substring %q", err, tt.wantError)
			}
		})
	}
}

func TestAscend950DTWholeCardModeUsesEffectiveNodeMode(t *testing.T) {
	for _, globalMode := range []string{"hami-core", "enpu"} {
		t.Run(globalMode, func(t *testing.T) {
			t.Run("node template override", func(t *testing.T) {
				am := newFake950DTManager(t, newFake950DTAPI())
				if err := am.LoadConfig(writeConfig(t, config950DT(globalMode, "[]", 98304, 32))); err != nil {
					t.Fatalf("LoadConfig() error = %v", err)
				}
				node := "nodes:\n- name: test-node\n  hamiVnpuMode: template\n"
				if err := am.LoadNodeConfig(writeConfig(t, node), "test-node"); err != nil {
					t.Fatalf("LoadNodeConfig() error = %v", err)
				}
				if err := am.ValidateWholeCardMode(); err != nil {
					t.Fatalf("ValidateWholeCardMode() error = %v, want nil", err)
				}
			})

			t.Run("without node override", func(t *testing.T) {
				am := newFake950DTManager(t, newFake950DTAPI())
				if err := am.LoadConfig(writeConfig(t, config950DT(globalMode, "[]", 98304, 32))); err != nil {
					t.Fatalf("LoadConfig() error = %v", err)
				}
				if err := am.ValidateWholeCardMode(); err == nil || !strings.Contains(err.Error(), "full-card allocation only") {
					t.Fatalf("ValidateWholeCardMode() error = %v, want full-card allocation only", err)
				}
			})
		})
	}
}

func TestAscend950DTHealthQueryFailureIsUnhealthyAndCleanupIsSkipped(t *testing.T) {
	api := newFake950DTAPI()
	api.healthErr[1] = errors.New("health unavailable")
	am := newFake950DTManager(t, api)
	if err := am.LoadConfig(writeConfig(t, config950DT("template", "[]", 98304, 32))); err != nil {
		t.Fatal(err)
	}
	if err := am.UpdateDevice(); err != nil {
		t.Fatalf("UpdateDevice() error = %v", err)
	}
	if !am.GetDeviceByUUID(formatDDIE(api.dies[0])).Health {
		t.Fatal("device with successful healthy query should be healthy")
	}
	if am.GetDeviceByUUID(formatDDIE(api.dies[1])).Health {
		t.Fatal("device with failed health query must be unhealthy")
	}
	ids := am.GetUnHealthIDs()
	if len(ids) != 1 || ids[0] != 1 {
		t.Fatalf("GetUnHealthIDs() = %v, want [1]", ids)
	}
	callsBeforeCleanup := api.deviceListCalls
	if err := am.CleanupIdleVNPUs(); err != nil {
		t.Fatalf("CleanupIdleVNPUs() error = %v", err)
	}
	if api.deviceListCalls != callsBeforeCleanup {
		t.Fatal("CleanupIdleVNPUs() queried DCMI; 950DT cleanup must be skipped")
	}
}

func TestAscend950DTDiscoveryFailurePreservesDevicesUntilRecovery(t *testing.T) {
	api := newFake950DTAPI()
	am := newFake950DTManager(t, api)
	if err := am.LoadConfig(writeConfig(t, config950DT("template", "[]", 98304, 32))); err != nil {
		t.Fatal(err)
	}
	if err := am.UpdateDevice(); err != nil {
		t.Fatal(err)
	}
	original := am.GetDevices()
	if len(original) != 2 || !original[0].Health || !original[1].Health {
		t.Fatalf("initial devices = %+v", original)
	}
	saved := api.hbm[0]
	delete(api.hbm, 0)
	if err := am.UpdateDevice(); err != nil {
		t.Fatal(err)
	}
	failed := am.GetDevices()
	if len(failed) != 2 || failed[0].UUID != original[0].UUID || failed[1].UUID != original[1].UUID || failed[0].Health || failed[1].Health {
		t.Fatalf("failed discovery devices = %+v", failed)
	}
	if !original[0].Health || !original[1].Health {
		t.Fatal("previous device objects were mutated")
	}
	api.hbm[0] = saved
	if err := am.UpdateDevice(); err != nil {
		t.Fatal(err)
	}
	recovered := am.GetDevices()
	if len(recovered) != 2 || recovered[0].UUID != original[0].UUID || recovered[1].UUID != original[1].UUID || !recovered[0].Health || !recovered[1].Health {
		t.Fatalf("recovered devices = %+v", recovered)
	}
}

func TestAscend950DTUpdateDeviceAppliesNodeFiltersBeforePublishing(t *testing.T) {
	for _, tc := range []struct {
		name   string
		filter internal.FilterDevices
	}{
		{name: "physical index", filter: internal.FilterDevices{Index: []int32{0}}},
		{name: "DDIE UUID", filter: internal.FilterDevices{UUID: []string{formatDDIE([ascend950DTDieIDCount]uint32{1, 2, 3, 4, 5})}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			am := newFake950DTManager(t, newFake950DTAPI())
			if err := am.LoadConfig(writeConfig(t, config950DT("template", "[]", 98304, 32))); err != nil {
				t.Fatal(err)
			}
			am.nodeConfig = &internal.NodeConfig{FilterDevices: tc.filter}
			if err := am.UpdateDevice(); err != nil {
				t.Fatalf("UpdateDevice() error = %v", err)
			}
			devices := am.GetDevices()
			if len(devices) != 1 || devices[0].LogicID != 1 {
				t.Fatalf("GetDevices() = %+v, want only logic ID 1", devices)
			}
			if ids := am.GetIDs(); len(ids) != 1 || ids[0] != 1 {
				t.Fatalf("GetIDs() = %v, want [1]", ids)
			}
		})
	}
}

func TestAscend950DTNodeConfigRejectsSlicing(t *testing.T) {
	for _, tc := range []struct {
		name, yaml, want string
	}{
		{"mode", "hamiVnpuMode: enpu", "full-card allocation only"},
		{"device count", "vDeviceCount: 2", "supports one whole-card device"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			am := newFake950DTManager(t, newFake950DTAPI())
			if err := am.LoadConfig(writeConfig(t, config950DT("template", "[]", 98304, 32))); err != nil {
				t.Fatal(err)
			}
			node := "nodes:\n- name: test-node\n  " + strings.ReplaceAll(tc.yaml, "\n", "\n  ") + "\n"
			err := am.LoadNodeConfig(writeConfig(t, node), "test-node")
			if err == nil && tc.name == "mode" {
				err = am.ValidateWholeCardMode()
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("LoadNodeConfig()/ValidateWholeCardMode() error = %v, want substring %q", err, tc.want)
			}
		})
	}
}
