/*
 * Copyright 2026 The HAMi Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package manager

import (
	"fmt"
	"math"
	"sort"

	"ascend-common/devmanager/common"

	"github.com/Project-HAMi/ascend-device-plugin/internal"
	"k8s.io/klog/v2"
)

const (
	ascend950DTName       = "Ascend950DT"
	ascend950DTNPUName    = "9582"
	ascend950DTCommonWord = "Ascend950DT"
	ascend950DTResource   = "huawei.com/Ascend950DT"
	ascend950DTDieIDCount = 5
)

// dcmiV2API is intentionally limited to read-only APIs needed for whole-card
// discovery and health. 950DT does not use DCMI virtual-device operations.
type dcmiV2API interface {
	init() error
	allDeviceCount() (int32, error)
	deviceList() ([]int32, error)
	chipInfo(int32) (common.ChipInfo, error)
	deviceHealth(int32) (uint32, error)
	deviceDDIE(int32) ([ascend950DTDieIDCount]uint32, error)
	chipPhyID(int32) (int32, error)
	deviceIDByChipPhyID(int32) (int32, error)
	slotChipID(int32) (int32, int32, error)
	deviceIDBySlotChipID(int32, int32) (int32, error)
	hbmInfo(int32) (*common.HbmInfo, error)
}

type ascend950DTDeviceInfo struct {
	logicID  int32
	phyID    int32
	cardID   int32
	deviceID int32
	uuid     string
	chipInfo common.ChipInfo
}

type ascend950DTManager struct {
	api       dcmiV2API
	chipInfo  common.ChipInfo
	deviceIDs []int32
}

func newAscend950DTManager() (*ascend950DTManager, error) {
	api, err := newDcmiV2Client()
	if err != nil {
		return nil, err
	}
	return newAscend950DTManagerWithAPI(api)
}

func newAscend950DTManagerWithAPI(api dcmiV2API) (*ascend950DTManager, error) {
	if api == nil {
		return nil, fmt.Errorf("DCMI v2 API is nil")
	}
	if err := api.init(); err != nil {
		return nil, fmt.Errorf("dcmiv2_init failed: %w", err)
	}
	mgr := &ascend950DTManager{api: api}
	devices, err := mgr.readDeviceInfo()
	if err != nil {
		return nil, err
	}
	if len(devices) == 0 {
		return nil, fmt.Errorf("DCMI v2 reported no Ascend950DT devices")
	}
	mgr.chipInfo = devices[0].chipInfo
	mgr.deviceIDs = make([]int32, 0, len(devices))
	for _, dev := range devices {
		mgr.deviceIDs = append(mgr.deviceIDs, dev.logicID)
	}
	return mgr, nil
}

func isAscend950DT(info common.ChipInfo) bool {
	return info.Type == "Ascend" && info.Name == ascend950DTName
}

func formatDDIE(values [ascend950DTDieIDCount]uint32) string {
	return fmt.Sprintf("DDIE-%08x-%08x-%08x-%08x-%08x",
		values[0], values[1], values[2], values[3], values[4])
}

// readDeviceInfo validates the full mapping before any device can be exposed
// to kubelet. DCMI v2 device IDs are logical IDs; HAMi allocation uses the
// chip physical ID as ASCEND_VISIBLE_DEVICES.
func (m *ascend950DTManager) readDeviceInfo() ([]ascend950DTDeviceInfo, error) {
	count, err := m.api.allDeviceCount()
	if err != nil {
		return nil, fmt.Errorf("dcmiv2_get_all_device_count failed: %w", err)
	}
	ids, err := m.api.deviceList()
	if err != nil {
		return nil, fmt.Errorf("dcmiv2_get_device_list failed: %w", err)
	}
	if count <= 0 || int(count) != len(ids) {
		return nil, fmt.Errorf("DCMI v2 device count/list mismatch: count=%d list=%d", count, len(ids))
	}

	seenLogic := make(map[int32]bool, len(ids))
	seenPhy := make(map[int32]bool, len(ids))
	seenSlotChip := make(map[[2]int32]bool, len(ids))
	seenUUID := make(map[string]bool, len(ids))
	devices := make([]ascend950DTDeviceInfo, 0, len(ids))
	for _, logicID := range ids {
		if logicID < 0 || seenLogic[logicID] {
			return nil, fmt.Errorf("invalid or duplicate DCMI v2 logic ID %d", logicID)
		}
		seenLogic[logicID] = true

		chipInfo, err := m.api.chipInfo(logicID)
		if err != nil {
			return nil, fmt.Errorf("get chip info for logic ID %d: %w", logicID, err)
		}
		if !isAscend950DT(chipInfo) {
			return nil, fmt.Errorf("logic ID %d is not Ascend950DT: type=%q name=%q npu_name=%q",
				logicID, chipInfo.Type, chipInfo.Name, chipInfo.NpuName)
		}
		if chipInfo.AICoreCnt <= 0 {
			return nil, fmt.Errorf("logic ID %d returned invalid AICore count %d", logicID, chipInfo.AICoreCnt)
		}

		phyID, err := m.api.chipPhyID(logicID)
		if err != nil {
			return nil, fmt.Errorf("get chip physical ID for logic ID %d: %w", logicID, err)
		}
		if phyID < 0 || seenPhy[phyID] {
			return nil, fmt.Errorf("invalid or duplicate physical ID %d for logic ID %d", phyID, logicID)
		}
		seenPhy[phyID] = true
		logicByPhy, err := m.api.deviceIDByChipPhyID(phyID)
		if err != nil || logicByPhy != logicID {
			return nil, fmt.Errorf("unsafe logic/physical mapping: logic=%d phy=%d reverse=%d err=%v",
				logicID, phyID, logicByPhy, err)
		}

		slotID, chipID, err := m.api.slotChipID(logicID)
		if err != nil {
			return nil, fmt.Errorf("get slot/chip mapping for logic ID %d: %w", logicID, err)
		}
		if slotID < 0 || chipID < 0 || seenSlotChip[[2]int32{slotID, chipID}] {
			return nil, fmt.Errorf("invalid or duplicate slot/chip mapping for logic ID %d: slot=%d chip=%d",
				logicID, slotID, chipID)
		}
		seenSlotChip[[2]int32{slotID, chipID}] = true
		logicBySlot, err := m.api.deviceIDBySlotChipID(slotID, chipID)
		if err != nil || logicBySlot != logicID {
			return nil, fmt.Errorf("unsafe logic/slot mapping: logic=%d slot=%d chip=%d reverse=%d err=%v",
				logicID, slotID, chipID, logicBySlot, err)
		}

		dieID, err := m.api.deviceDDIE(logicID)
		if err != nil {
			return nil, fmt.Errorf("get DDIE for logic ID %d: %w", logicID, err)
		}
		allZero := true
		for _, value := range dieID {
			if value != 0 {
				allZero = false
				break
			}
		}
		uuid := formatDDIE(dieID)
		if allZero || seenUUID[uuid] {
			return nil, fmt.Errorf("invalid or duplicate DDIE for logic ID %d: %s", logicID, uuid)
		}
		seenUUID[uuid] = true

		devices = append(devices, ascend950DTDeviceInfo{
			logicID: logicID, phyID: phyID, cardID: slotID, deviceID: chipID,
			uuid: uuid, chipInfo: chipInfo,
		})
	}
	sort.Slice(devices, func(i, j int) bool { return devices[i].logicID < devices[j].logicID })
	return devices, nil
}

func (m *ascend950DTManager) validateConfig(config internal.VNPUConfig) error {
	if config.CommonWord != ascend950DTCommonWord || config.ResourceName != ascend950DTResource {
		return fmt.Errorf("Ascend950DT requires commonWord=%q and resourceName=%q",
			ascend950DTCommonWord, ascend950DTResource)
	}
	if len(config.Templates) != 0 {
		return fmt.Errorf("Ascend950DT supports whole-card allocation only; vNPU templates must be empty")
	}
	if config.MemoryCapacity <= 0 || config.MemoryAllocatable <= 0 || config.MemoryAllocatable > config.MemoryCapacity {
		return fmt.Errorf("Ascend950DT requires 0 < memoryAllocatable <= memoryCapacity, got %d/%d",
			config.MemoryAllocatable, config.MemoryCapacity)
	}
	if config.AICore != int32(m.chipInfo.AICoreCnt) {
		return fmt.Errorf("Ascend950DT aiCore config %d does not match DCMI v2 chip info %d",
			config.AICore, m.chipInfo.AICoreCnt)
	}
	devices, err := m.readDeviceInfo()
	if err != nil {
		return err
	}
	for _, dev := range devices {
		if config.AICore != int32(dev.chipInfo.AICoreCnt) {
			return fmt.Errorf("Ascend950DT logic ID %d aiCore config %d does not match DCMI v2 chip info %d", dev.logicID, config.AICore, dev.chipInfo.AICoreCnt)
		}
		hbm, err := m.api.hbmInfo(dev.logicID)
		if err != nil {
			return fmt.Errorf("get HBM info for logic ID %d: %w", dev.logicID, err)
		}
		if hbm == nil || hbm.MemorySize == 0 || hbm.MemorySize > math.MaxInt32 || hbm.Usage > hbm.MemorySize {
			return fmt.Errorf("invalid HBM info for logic ID %d: %+v", dev.logicID, hbm)
		}
		if int64(hbm.MemorySize) != config.MemoryCapacity {
			return fmt.Errorf("Ascend950DT logic ID %d HBM capacity %d MiB does not match configured memoryCapacity %d MiB",
				dev.logicID, hbm.MemorySize, config.MemoryCapacity)
		}
	}
	return nil
}

func (m *ascend950DTManager) discover(config internal.VNPUConfig) ([]*Device, []int32, error) {
	devices, err := m.readDeviceInfo()
	if err != nil {
		return nil, nil, err
	}
	newDevices := make([]*Device, 0, len(devices))
	ids := make([]int32, 0, len(devices))
	for _, dev := range devices {
		hbm, err := m.api.hbmInfo(dev.logicID)
		if err != nil {
			return nil, nil, fmt.Errorf("get HBM info for logic ID %d: %w", dev.logicID, err)
		}
		if hbm == nil || hbm.MemorySize == 0 || hbm.MemorySize > math.MaxInt32 || hbm.Usage > hbm.MemorySize || int64(hbm.MemorySize) != config.MemoryCapacity {
			return nil, nil, fmt.Errorf("unsafe HBM capacity for logic ID %d: got %+v, configured capacity %d MiB",
				dev.logicID, hbm, config.MemoryCapacity)
		}
		health, healthErr := m.api.deviceHealth(dev.logicID)
		healthy := healthErr == nil && health == 0
		if healthErr != nil {
			klog.Errorf("dcmiv2_get_device_health failed for logic ID %d; marking device unhealthy: %v", dev.logicID, healthErr)
		}
		newDevices = append(newDevices, &Device{
			UUID: dev.uuid, LogicID: dev.logicID, PhyID: dev.phyID,
			CardID: dev.cardID, DeviceID: dev.deviceID,
			Memory: config.MemoryAllocatable, AICore: config.AICore, Health: healthy,
		})
		ids = append(ids, dev.logicID)
	}
	return newDevices, ids, nil
}
