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
	"math"
	"runtime"
	"sort"
	"sync"
	"time"

	"ascend-common/devmanager"
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
	mu                sync.RWMutex
	mgr               devmanager.DeviceInterface
	config            internal.VNPUConfig
	globalConfig      internal.Config
	devs              []*Device
	nodeConfig        *internal.NodeConfig
	healthUnavailable bool
}

func NewAscendManager() (*AscendManager, error) {
	mgr, err := devmanager.AutoInit("", 30)
	if err != nil {
		return nil, fmt.Errorf("failed to auto-init device manager: %w", err)
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

func (am *AscendManager) isAscend950() bool {
	return internal.ChipFamily(am.config.ChipName) != ""
}

func (am *AscendManager) deviceDieType() dcmi.DieType {
	if am.isAscend950() {
		return dcmi.DDIE
	}
	return dcmi.VDIE
}

// deviceIdentity prefers the hardware die ID. If that query fails, the
// physical ID from the same driver still identifies the device. Ascend950DT
// die-id queries have failed in a mismatched container image (DCMI -8010)
// while the host driver still enumerates the NPU.
func (am *AscendManager) deviceIdentity(logicID, phyID int32) string {
	primary := am.deviceDieType()
	alternate := dcmi.DDIE
	if primary == dcmi.DDIE {
		alternate = dcmi.VDIE
	}
	for _, dieType := range []dcmi.DieType{primary, alternate} {
		uuid, err := am.mgr.GetDieID(logicID, dieType)
		if err == nil && uuid != "" {
			return uuid
		}
		klog.Warningf("die ID type %d unavailable for logicID %d phyID %d: %v", dieType, logicID, phyID, err)
	}
	if phyID >= 0 {
		return fmt.Sprintf("phy-%d", phyID)
	}
	return fmt.Sprintf("logic-%d", logicID)
}

// deviceHealthy keeps the NPU registered when the health query fails or hangs.
// A failed query is not a detected fault.
func (am *AscendManager) deviceHealthy(logicID int32) bool {
	if am.healthUnavailable {
		return true
	}
	type healthResult struct {
		code uint32
		err  error
	}
	ch := make(chan healthResult, 1)
	go func() {
		code, err := am.mgr.GetDeviceHealth(logicID)
		ch <- healthResult{code, err}
	}()
	select {
	case res := <-ch:
		if res.err != nil {
			am.healthUnavailable = true
			klog.Warningf("device health unavailable for logicID %d, keeping the device registered: %v", logicID, res.err)
			return true
		}
		return res.code == 0
	case <-time.After(time.Second):
		am.healthUnavailable = true
		klog.Warningf("device health timed out for logicID %d, keeping the device registered", logicID)
		return true
	}
}

// deviceIndex is the filter index. On Ascend950DT, NPU ID equals Chip Phy-ID.
func (am *AscendManager) deviceIndex(logicID int32) (int32, error) {
	if am.isAscend950() {
		return am.mgr.GetPhysicIDFromLogicID(logicID)
	}
	cardID, _, err := am.mgr.GetCardIDDeviceID(logicID)
	return cardID, err
}

func (am *AscendManager) LoadConfig(path string) error {
	config, err := internal.LoadConfig(path)
	if err != nil {
		return fmt.Errorf("failed to load config from %s: %w", path, err)
	}
	chipInfo, err := am.mgr.GetValidChipInfo()
	if err != nil {
		return fmt.Errorf("failed to get valid chip info: %w", err)
	}
	if chipInfo.Type != "Ascend" {
		return fmt.Errorf("chip type is not Ascend")
	}
	names := make([]string, len(config.VNPUs.Configs))
	for i, vnpu := range config.VNPUs.Configs {
		names[i] = vnpu.ChipName
	}
	idx, err := internal.MatchChipConfig(chipInfo.Name, names)
	if err != nil {
		return err
	}
	klog.Infof("detected Ascend chip type=%q name=%q, selected config=%q", chipInfo.Type, chipInfo.Name, names[idx])
	am.config = config.VNPUs.Configs[idx]
	am.globalConfig = *config
	sort.Slice(am.config.Templates, func(i, j int) bool {
		return am.config.Templates[i].Memory < am.config.Templates[j].Memory
	})
	klog.Infof("load config: %v", am.config)
	return nil
}

func (am *AscendManager) CommonWord() string {
	return am.config.CommonWord
}

func (am *AscendManager) ResourceName() string {
	return am.config.ResourceName
}

func (am *AscendManager) VDeviceCount() int {
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
		cardID, deviceID := int32(-1), int32(-1)
		deviceIndex := phyID
		if !am.isAscend950() {
			cardID, deviceID, err = am.mgr.GetCardIDDeviceID(ID)
			if err != nil {
				klog.Errorf("failed to get card id from device id: %v", err)
				return err
			}
			deviceIndex = cardID
		}
		uuid := am.deviceIdentity(ID, phyID)
		if am.shouldIgnoreDevice(uuid, deviceIndex) {
			klog.V(4).Infof("ignore device matched filterDevices uuid=%s index=%d logicID=%d phyID=%d deviceID=%d", uuid, deviceIndex, ID, phyID, deviceID)
			continue
		}
		newDevs = append(newDevs, &Device{
			UUID:     uuid,
			LogicID:  ID,
			PhyID:    phyID,
			CardID:   cardID,
			DeviceID: deviceID,
			Memory:   am.config.MemoryAllocatable,
			AICore:   am.config.AICore,
			Health:   am.deviceHealthy(ID),
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
		cardID, err := am.deviceIndex(id)
		if err != nil {
			klog.Warningf("failed to get device index for logic ID %d: %v", id, err)
			continue
		}
		uuid := ""
		if am.nodeConfig.FilterDevices.HasUUID() {
			uuid, err = am.mgr.GetDieID(id, am.deviceDieType())
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
	_, IDs, err := am.mgr.GetDeviceList()
	if err != nil {
		return nil
	}
	filterDevicesEnabled := am.shouldCheckIgnored()
	var unhealthy []int32
	for _, d := range IDs {
		if filterDevicesEnabled {
			cardID, err := am.deviceIndex(d)
			if err != nil {
				klog.Warningf("failed to get device index for logic ID %d: %v", d, err)
				continue
			}
			uuid := ""
			if am.nodeConfig.FilterDevices.HasUUID() {
				uuid, err = am.mgr.GetDieID(d, am.deviceDieType())
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
	if am.isAscend950() {
		// Ascend950DT does not create hardware template vNPU instances.
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
	mode, err := am.vnpuMode()
	return err == nil && mode == internal.VNPUModeHamiCore
}

// IsEnpu returns whether ENPU is enabled, preferring the node setting.
func (am *AscendManager) IsEnpu() bool {
	mode, err := am.vnpuMode()
	return err == nil && mode == internal.VNPUModeENPU
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

// ValidateExperimental950 runs after global and node configuration are loaded.
func (am *AscendManager) ValidateExperimental950() error {
	return am.validateExperimental950(runtime.GOARCH)
}

func (am *AscendManager) validateExperimental950(arch string) error {
	family := internal.ChipFamily(am.config.ChipName)
	if family == "" {
		return nil
	}
	if am.nodeConfig == nil || !am.nodeConfig.EnableExperimental950 {
		return fmt.Errorf("%s requires node enableExperimental950: true", family)
	}
	if !am.IsHamiVnpuCore() {
		// Ascend950DT driver 25.1.rc3 rejects vNPU creation:
		//   npu-smi set -t create-vnpu -h => "This device does not support setting create-vnpu"
		// libdcmi only has 910/910B/310P template tables, so template mode cannot work.
		return fmt.Errorf("%s requires hami-core mode: the driver reports "+
			"\"This device does not support setting create-vnpu\", so vNPU template hard slicing is "+
			"unavailable; use hamiVnpuMode: hami-core (fixed memory quota + fixed time slice)", family)
	}
	if arch != "arm64" {
		return fmt.Errorf("%s nodes are arm64, got %s", family, arch)
	}
	if am.config.CommonWord != family || am.config.ResourceName != "huawei.com/"+family {
		return fmt.Errorf("950 commonWord/resourceName must use canonical family %s", family)
	}
	if am.config.MemoryAllocatable <= 0 || am.config.MemoryAllocatable > math.MaxInt32 || am.config.MemoryCapacity < am.config.MemoryAllocatable {
		return fmt.Errorf("%s requires memoryAllocatable and memoryCapacity in MiB (capacity >= allocatable > 0)", family)
	}
	if len(am.config.Templates) != 0 {
		return fmt.Errorf("%s must not define hardware templates", family)
	}
	if am.nodeConfig.VDeviceCount < 2 {
		return fmt.Errorf("%s requires node vDeviceCount >= 2", family)
	}
	if am.nodeConfig.DeviceShareMode != "card-chip" && am.nodeConfig.DeviceShareMode != "npu-id" {
		return fmt.Errorf("%s requires node deviceShareMode: card-chip or npu-id", family)
	}
	return nil
}

// DeviceShareMode selects the SMI command layout recorded for this node.
func (am *AscendManager) DeviceShareMode() string {
	if am.nodeConfig != nil && am.nodeConfig.DeviceShareMode != "" {
		return am.nodeConfig.DeviceShareMode
	}
	return "card-chip"
}

// DeviceManager exposes the initialized DCMI manager to in-process reporters.
func (am *AscendManager) DeviceManager() devmanager.DeviceInterface {
	return am.mgr
}
