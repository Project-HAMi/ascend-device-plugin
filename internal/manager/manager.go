/*
 * Copyright 2024 The HAMi Authors.
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
	"sort"
	"strings"
	"sync"

	"ascend-common/devmanager"
	"ascend-common/devmanager/common"
	"ascend-common/devmanager/dcmi"

	"github.com/Project-HAMi/ascend-device-plugin/internal"
	"k8s.io/klog/v2"
)

type Device struct {
	UUID     string
	LogicID  int32
	PhyID    int32
	CardID   int32
	DeviceID int32
	Memory   int64
	AICore   int32
	Health   bool
}

// Manager defines the interface that PluginServer depends on.
// AscendManager implements this interface.
type Manager interface {
	CommonWord() string
	ResourceName() string
	VDeviceCount() int
	UpdateDevice() error
	GetDevices() []*Device
	GetDeviceByUUID(UUID string) *Device
	GetUnHealthIDs() []int32
	CleanupIdleVNPUs() error
	IsHamiVnpuCore() bool
	DeviceCoreScaling() float64
}

type AscendManager struct {
	mu           sync.RWMutex
	mgr          devmanager.DeviceInterface
	v2Mgr        *ascend950DTManager
	config       internal.VNPUConfig
	globalConfig internal.Config
	devs         []*Device
	nodeConfig   *internal.NodeConfig
}

func NewAscendManager() (*AscendManager, error) {
	mgr, err := devmanager.AutoInit("", 30)
	if err != nil {
		v2Mgr, v2Err := newAscend950DTManager()
		if v2Err != nil {
			return nil, fmt.Errorf("failed to auto-init device manager: %v; Ascend950DT DCMI v2 detection failed: %w", err, v2Err)
		}
		return &AscendManager{v2Mgr: v2Mgr, devs: []*Device{}}, nil
	}
	return &AscendManager{
		mgr:  mgr,
		devs: []*Device{},
	}, nil
}

func (am *AscendManager) LoadNodeConfig(nodePath string, nodeName string) error {
	nodeConfigList, err := internal.LoadNodeConfig(nodePath)
	if err != nil {
		klog.Warningf("Failed to load node config from %s: %v", nodePath, err)
		return err
	}

	for _, n := range nodeConfigList.Nodes {
		if n.Name == nodeName {
			if am.v2Mgr != nil {
				if n.VDeviceCount < 0 || n.VDeviceCount > 1 {
					return fmt.Errorf("Ascend950DT supports one whole-card device per physical card; node %s vDeviceCount=%d", nodeName, n.VDeviceCount)
				}
			}
			am.nodeConfig = &n
			klog.Infof("Successfully matched node config for %s: %+v", nodeName, n)
			return nil
		}
	}

	klog.Infof("No specific config found for node %s, will use default settings", nodeName)
	return nil
}

func (am *AscendManager) shouldIgnoreDevice(uuid string, index int32) bool {
	if !am.shouldCheckIgnored() {
		return false
	}

	for _, ignoredUUID := range am.nodeConfig.FilterDevices.UUID {
		if uuid != "" && uuid == ignoredUUID {
			return true
		}
	}
	for _, ignoredIndex := range am.nodeConfig.FilterDevices.Index {
		if index == ignoredIndex {
			return true
		}
	}
	return false
}

func (am *AscendManager) shouldCheckIgnored() bool {
	return am.nodeConfig != nil && !am.nodeConfig.FilterDevices.IsEmpty()
}

func (am *AscendManager) LoadConfig(path string) error {
	config, err := internal.LoadConfig(path)
	if err != nil {
		return fmt.Errorf("failed to load config from %s: %w", path, err)
	}
	var chipInfo common.ChipInfo
	if am.v2Mgr != nil {
		chipInfo = am.v2Mgr.chipInfo
	} else {
		chipInfo, err = am.mgr.GetValidChipInfo()
		if err != nil {
			return fmt.Errorf("failed to get valid chip info: %w", err)
		}
	}
	if chipInfo.Type != "Ascend" {
		return fmt.Errorf("chip type is not Ascend")
	}
	idx := -1
	for i, vnpu := range config.VNPUs.Configs {
		if vnpu.ChipName == chipInfo.Name {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("can not find vnpu config for chip %s", chipInfo.Name)
	}
	am.config = config.VNPUs.Configs[idx]
	am.globalConfig = *config
	if am.v2Mgr != nil {
		if err := am.v2Mgr.validateConfig(am.config); err != nil {
			return err
		}
	}
	sort.Slice(am.config.Templates, func(i, j int) bool {
		return am.config.Templates[i].Memory < am.config.Templates[j].Memory
	})
	klog.Infof("load config: %v", am.config)
	return nil
}

// ValidateWholeCardMode checks the effective vNPU mode after global and node
// configuration have both been loaded.
func (am *AscendManager) ValidateWholeCardMode() error {
	if am.v2Mgr == nil {
		return nil
	}
	mode, err := am.vnpuMode()
	if err != nil {
		return fmt.Errorf("resolve Ascend950DT mode: %w", err)
	}
	if mode != internal.VNPUModeTemplate {
		return fmt.Errorf("Ascend950DT supports full-card allocation only; hamiVnpuMode=%q", mode)
	}
	return nil
}

func (am *AscendManager) CommonWord() string {
	return am.config.CommonWord
}

func (am *AscendManager) ResourceName() string {
	return am.config.ResourceName
}

func (am *AscendManager) VDeviceCount() int {
	if am.v2Mgr != nil {
		return 1
	}
	count := 0
	if am.nodeConfig != nil && am.nodeConfig.VDeviceCount > 0 {
		count = am.nodeConfig.VDeviceCount
	} else if am.IsHamiVnpuCore() || am.IsEnpu() {
		count = am.globalConfig.VNPUs.DeviceSplitCount
		if count <= 0 {
			count = 10
		}
	}
	if count > 0 {
		if am.IsEnpu() && count > 100 {
			klog.Warningf("device count %d exceeds ENPU's 100 virtual-NPU slots; using 100", count)
			return 100
		}
		return count
	}
	if len(am.config.Templates) == 0 {
		return 1
	}
	return int(am.config.MemoryAllocatable / am.config.Templates[0].Memory)
}

func (am *AscendManager) UpdateDevice() error {
	if am.v2Mgr != nil {
		devices, _, err := am.v2Mgr.discover(am.config)
		if err != nil {
			klog.Errorf("Ascend950DT discovery failed; clearing advertised devices: %v", err)
			devices = []*Device{}
		} else if am.shouldCheckIgnored() {
			available := make([]*Device, 0, len(devices))
			for _, dev := range devices {
				if am.shouldIgnoreDevice(dev.UUID, dev.CardID) {
					klog.V(4).Infof("ignore Ascend950DT device matched filterDevices uuid=%s index=%d logicID=%d phyID=%d deviceID=%d",
						dev.UUID, dev.CardID, dev.LogicID, dev.PhyID, dev.DeviceID)
					continue
				}
				available = append(available, dev)
			}
			devices = available
		}
		am.mu.Lock()
		am.devs = devices
		am.mu.Unlock()
		return nil
	}
	_, IDs, err := am.mgr.GetDeviceList()
	if err != nil {
		klog.Errorf("failed to get device list: %v", err)
		return err
	}

	newDevs := make([]*Device, 0, len(IDs))
	for _, ID := range IDs {
		phyID, err := am.mgr.GetPhysicIDFromLogicID(ID)
		if err != nil {
			klog.Errorf("failed to get physic id from logic id: %v", err)
			return err
		}
		cardID, deviceID, err := am.mgr.GetCardIDDeviceID(ID)
		if err != nil {
			klog.Errorf("failed to get card id from device id: %v", err)
			return err
		}
		uuid, err := am.mgr.GetDieID(ID, dcmi.VDIE)
		if err != nil {
			klog.Errorf("failed to get uuid from device id: %v", err)
			return err
		}
		if am.shouldIgnoreDevice(uuid, cardID) {
			klog.V(4).Infof("ignore device matched filterDevices uuid=%s index=%d logicID=%d phyID=%d deviceID=%d", uuid, cardID, ID, phyID, deviceID)
			continue
		}
		health, err := am.mgr.GetDeviceHealth(ID)
		if err != nil {
			klog.Errorf("failed to get device health: %v", err)
			return err
		}
		newDevs = append(newDevs, &Device{
			UUID:     uuid,
			LogicID:  ID,
			PhyID:    phyID,
			CardID:   cardID,
			DeviceID: deviceID,
			Memory:   am.config.MemoryAllocatable,
			AICore:   am.config.AICore,
			Health:   health == 0,
		})
	}
	am.mu.Lock()
	am.devs = newDevs
	am.mu.Unlock()
	return nil
}

func (am *AscendManager) GetDevices() []*Device {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.devs
}

func (am *AscendManager) GetDeviceByUUID(UUID string) *Device {
	am.mu.RLock()
	defer am.mu.RUnlock()
	for _, dev := range am.devs {
		if dev.UUID == UUID {
			return dev
		}
	}
	return nil
}

func (am *AscendManager) GetIDs() []int32 {
	if am.v2Mgr != nil {
		am.mu.RLock()
		defer am.mu.RUnlock()
		ids := make([]int32, 0, len(am.devs))
		for _, dev := range am.devs {
			if !am.shouldIgnoreDevice(dev.UUID, dev.CardID) {
				ids = append(ids, dev.LogicID)
			}
		}
		return ids
	}
	_, IDs, err := am.mgr.GetDeviceList()
	if err != nil {
		klog.Errorf("failed to get device list: %v", err)
		return nil
	}
	if !am.shouldCheckIgnored() {
		return IDs
	}

	availableIDs := make([]int32, 0, len(IDs))
	for _, id := range IDs {
		cardID, _, err := am.mgr.GetCardIDDeviceID(id)
		if err != nil {
			klog.Warningf("failed to get card/device ID for logic ID %d: %v", id, err)
			continue
		}
		uuid := ""
		if am.nodeConfig.FilterDevices.HasUUID() {
			uuid, err = am.mgr.GetDieID(id, dcmi.VDIE)
			if err != nil {
				klog.Warningf("failed to get uuid for logic ID %d: %v", id, err)
				continue
			}
		}
		if !am.shouldIgnoreDevice(uuid, cardID) {
			availableIDs = append(availableIDs, id)
		}
	}
	return availableIDs
}

func (am *AscendManager) GetUnHealthIDs() []int32 {
	if am.v2Mgr != nil {
		am.mu.RLock()
		devices := append([]*Device(nil), am.devs...)
		am.mu.RUnlock()
		var unhealthy []int32
		for _, dev := range devices {
			if am.shouldIgnoreDevice(dev.UUID, dev.CardID) {
				continue
			}
			health, err := am.v2Mgr.api.deviceHealth(dev.LogicID)
			if err != nil || health != 0 {
				if err != nil {
					klog.Errorf("dcmiv2_get_device_health failed for logic ID %d; reporting unhealthy: %v", dev.LogicID, err)
				}
				unhealthy = append(unhealthy, dev.LogicID)
			}
		}
		return unhealthy
	}
	_, IDs, err := am.mgr.GetDeviceList()
	if err != nil {
		return nil
	}
	filterDevicesEnabled := am.shouldCheckIgnored()
	var unhealthy []int32
	for _, d := range IDs {
		if filterDevicesEnabled {
			cardID, _, err := am.mgr.GetCardIDDeviceID(d)
			if err != nil {
				klog.Warningf("failed to get card/device ID for logic ID %d: %v", d, err)
				continue
			}
			uuid := ""
			if am.nodeConfig.FilterDevices.HasUUID() {
				uuid, err = am.mgr.GetDieID(d, dcmi.VDIE)
				if err != nil {
					klog.Warningf("failed to get uuid for logic ID %d: %v", d, err)
					continue
				}
			}
			if am.shouldIgnoreDevice(uuid, cardID) {
				continue
			}
		}
		healthCode, err := am.mgr.GetDeviceHealth(d)
		if err != nil {
			klog.Warningf("failed to get device health for %d: %v", d, err)
			continue
		}
		if healthCode != 0 {
			unhealthy = append(unhealthy, d)
		}
	}
	return unhealthy
}

func (am *AscendManager) CleanupIdleVNPUs() error {
	if am.v2Mgr != nil {
		klog.V(2).Info("Skipping vNPU cleanup for whole-card-only Ascend950DT")
		return nil
	}
	klog.Info("Starting cleanup of idle vNPUs...")

	_, IDs, err := am.mgr.GetDeviceList()
	if err != nil {
		return fmt.Errorf("failed to get device list: %w", err)
	}
	klog.Infof("Found %d devices to check for idle vNPUs,%+v", len(IDs), IDs)

	totalCleaned := 0
	for _, logicID := range IDs {
		cardID, deviceID, err := am.mgr.GetCardIDDeviceID(logicID)
		if err != nil {
			klog.Warningf("failed to get card/device ID for logic ID %d: %v", logicID, err)
			continue
		}
		uuid := ""
		if am.shouldCheckIgnored() && am.nodeConfig.FilterDevices.HasUUID() {
			uuid, err = am.mgr.GetDieID(logicID, dcmi.VDIE)
			if err != nil {
				klog.Warningf("failed to get uuid for logic ID %d: %v", logicID, err)
				continue
			}
		}
		if am.shouldIgnoreDevice(uuid, cardID) {
			klog.V(4).Infof("skip cleanup on ignored device uuid=%s index=%d logicID=%d deviceID=%d", uuid, cardID, logicID, deviceID)
			continue
		}
		// Obtain all vNPU information on this device
		vDevInfos, err := am.mgr.GetVirtualDeviceInfo(logicID)
		if err != nil {
			klog.Infof("no vNPU found on device %d or query failed: %v", logicID, err)
			continue
		}

		klog.V(1).Infof("Device logicID=%d, cardID=%d,deviceID=%d has %d vNPUs", logicID, cardID, deviceID, len(vDevInfos.VDevInfo))

		for _, vDev := range vDevInfos.VDevInfo {
			klog.V(1).Infof("vNPU CardId=%d, VDevID(Vnpu ID)=%d,template=%s,IsContainerUsed=%d", cardID, vDev.VDevID, vDev.QueryInfo.Name, vDev.QueryInfo.IsContainerUsed)

			if vDev.QueryInfo.IsContainerUsed == 0 {
				klog.V(1).Infof("Found idle vNPU: cardID=%d, deviceID=%d, vnpuID=%d, status=%d, template=%s,IsContainerUsed=%d",
					cardID, deviceID, vDev.VDevID, vDev.QueryInfo.Status, vDev.QueryInfo.Name, vDev.QueryInfo.IsContainerUsed)

				err := am.mgr.DestroyVirtualDevice(logicID, uint32(vDev.VDevID))
				if err != nil {
					klog.Errorf("failed to destroy vNPU %d on device %d: %v", vDev.VDevID, logicID, err)
				} else {
					klog.Infof("Successfully destroyed idle vNPU: vnpuID=%d", vDev.VDevID)
					totalCleaned++
				}
			} else {
				klog.Infof("Skipping active vNPU: cardID=%d, deviceID=%d, vnpuID=%d, status=%d, template=%s,IsContainerUsed=%d",
					cardID, deviceID, vDev.VDevID, vDev.QueryInfo.Status, vDev.QueryInfo.Name, vDev.QueryInfo.IsContainerUsed)
			}
		}
	}

	klog.Infof("Cleanup completed, destroyed %d idle vNPUs", totalCleaned)
	return nil
}

func (am *AscendManager) GetNodeConfig() *internal.NodeConfig {
	return am.nodeConfig
}

func (am *AscendManager) IsHamiVnpuCore() bool {
	if am.v2Mgr != nil {
		return false
	}
	mode, err := am.vnpuMode()
	return err == nil && mode == internal.VNPUModeHamiCore
}

// IsEnpu returns whether ENPU is enabled, preferring the node setting.
func (am *AscendManager) IsEnpu() bool {
	if am.v2Mgr != nil {
		return false
	}
	mode, err := am.vnpuMode()
	return err == nil && mode == internal.VNPUModeENPU
}

// IsFullCardOnly reports the 950DT backend's allocation constraint to the
// server so pod-level runtime overrides cannot enable a slicing mode.
func (am *AscendManager) IsFullCardOnly() bool {
	return am.v2Mgr != nil
}

func (am *AscendManager) vnpuMode() (string, error) {
	mode, err := am.globalConfig.VNPUs.Mode()
	if err != nil || am.nodeConfig == nil {
		return mode, err
	}
	return am.nodeConfig.Mode(mode)
}

// EnpuPolicy returns the default ENPU scheduling policy.
func (am *AscendManager) EnpuPolicy() string {
	return am.globalConfig.VNPUs.EnpuPolicy
}

// HamiCorePolicy returns the runtime policy, preferring a nonempty node override.
// Empty values are resolved to the default by the allocating server.
func (am *AscendManager) HamiCorePolicy() string {
	if am.nodeConfig != nil && strings.TrimSpace(am.nodeConfig.HamiCorePolicy) != "" {
		return am.nodeConfig.HamiCorePolicy
	}
	return am.globalConfig.VNPUs.HamiCorePolicy
}

// DeviceCoreScaling returns the hami-core oversell ratio in effect, preferring
// a per-node override over the global value. An override of 0 reads as "not
// set", matching the omitempty encoding of the field. Every other override is
// returned as it was written, an invalid one included, so that AdvertisedDevcore
// reports it and falls back to the percentage base instead of the node silently
// inheriting the global ratio.
func (am *AscendManager) DeviceCoreScaling() float64 {
	if am.nodeConfig != nil && am.nodeConfig.DeviceCoreScaling != 0 {
		return am.nodeConfig.DeviceCoreScaling
	}
	return am.globalConfig.VNPUs.DeviceCoreScaling
}
