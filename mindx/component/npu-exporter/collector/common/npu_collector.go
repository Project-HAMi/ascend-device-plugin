/* Copyright(C) 2021-2025. Huawei Technologies Co.,Ltd. All rights reserved.
   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

// Package common for general collector
package common

import (
	"context"
	"sync"
	"time"

	"ascend-common/api"
	"ascend-common/common-utils/cache"
	"ascend-common/devmanager"
	"ascend-common/devmanager/common"
	"ascend-common/devmanager/dcmi"
	"huawei.com/npu-exporter/v6/collector/container"
	"huawei.com/npu-exporter/v6/utils/logger"
)

var (
	npuContainerInfoInit sync.Once
	npuChipInfoInit      sync.Once
	// Collector base collector for prometheus and telegraf
	Collector *NpuCollector

	// ChainForSingleGoroutine a list of collectors for single goroutine
	ChainForSingleGoroutine []MetricsCollector

	// ChainForMultiGoroutine a list of collectors for multi goroutine
	ChainForMultiGoroutine []MetricsCollector

	// ChainForCustomPlugin a list of collectors for plugin
	ChainForCustomPlugin []MetricsCollector

	updateTimeForCardIds = time.Minute
)

const (
	maxCollectTimeout = 10 * time.Second
)

// NpuCollector for collect metrics
type NpuCollector struct {
	cache         *cache.ConcurrencyLRUCache
	devicesParser *container.DevicesParser
	updateTime    time.Duration
	cacheTime     time.Duration
	Dmgr          *devmanager.DeviceManager
}

// NewNpuCollector create a new collector
func NewNpuCollector(cacheTime time.Duration, updateTime time.Duration,
	deviceParser *container.DevicesParser, dmgr *devmanager.DeviceManager) *NpuCollector {
	CommonCollector := &NpuCollector{
		cache:         cache.New(cacheSize),
		cacheTime:     cacheTime,
		updateTime:    updateTime,
		devicesParser: deviceParser,
		Dmgr:          dmgr,
	}
	return CommonCollector
}

// StartCollect start collect
func StartCollect(group *sync.WaitGroup, ctx context.Context, n *NpuCollector) {
	npuChipInfoInitAtFirstTime(n)
	startCollectSingleGoroutine(group, ctx, n)
	startCollectForMultiGoroutine(group, ctx, n)
	startCollectForPluginGoroutine(group, ctx, n)
}

func startCollectForPluginGoroutine(group *sync.WaitGroup, ctx context.Context, n *NpuCollector) {
	group.Add(1)
	go func() {
		defer group.Done()
		ticker := time.NewTicker(n.updateTime)
		defer ticker.Stop()
		goroutinePreCollect(ChainForCustomPlugin, n)
		defer goroutinePostCollect(ChainForCustomPlugin, n)
		runPluginCollect(ctx, n, ticker)
	}()
}

func runPluginCollect(ctx context.Context, n *NpuCollector, ticker *time.Ticker) {
	for {
		select {
		case <-ctx.Done():
			logger.Info("received the stop signal,stop plugin collect")
			return
		default:
			collectPluginMetrics(n)
			if _, ok := <-ticker.C; !ok {
				logger.Errorf(tickerFailedPattern, "handling plugin collectors")
				return
			}
		}
	}
}

func collectPluginMetrics(n *NpuCollector) {
	chipList := getChipListCache(n)
	for _, c := range ChainForCustomPlugin {
		resultChan := make(chan struct{}, 1)
		go func(cur MetricsCollector) {
			cur.CollectToCache(n, chipList)
			resultChan <- struct{}{}
		}(c)
		select {
		case <-resultChan:
			continue
		case <-time.After(maxCollectTimeout):
			logger.Errorf("collect timeout for %v", GetCacheKey(c))
			continue
		}

	}
}

func startCollectForMultiGoroutine(group *sync.WaitGroup, ctx context.Context, n *NpuCollector) {
	chips := getChipListCache(n)

	group.Add(len(chips))
	for _, chip := range chips {
		go func(chip HuaWeiAIChip) {
			defer group.Done()
			runChipCollector(ctx, n, chip)
		}(chip)
	}
}

func runChipCollector(ctx context.Context, n *NpuCollector, chip HuaWeiAIChip) {
	ticker := time.NewTicker(n.updateTime)
	defer ticker.Stop()
	goroutinePreCollect(ChainForMultiGoroutine, n)
	defer goroutinePostCollect(ChainForMultiGoroutine, n)
	for {
		select {
		case <-ctx.Done():
			logger.Infof("received the stop signal,stop collect network info of npu(%d)", chip.LogicID)
			return
		default:
			singleChipSlice := []HuaWeiAIChip{chip}
			for _, c := range ChainForMultiGoroutine {
				c.CollectToCache(n, singleChipSlice)
			}
			if _, ok := <-ticker.C; !ok {
				logger.Errorf(tickerFailedPattern, "collect for multigroutine ")
				return
			}
		}
	}
}

func goroutinePreCollect(collectors []MetricsCollector, n *NpuCollector) {
	chipList := getChipListCache(n)
	for _, c := range collectors {
		c.PreCollect(n, chipList)
	}
}

func goroutinePostCollect(collectors []MetricsCollector, n *NpuCollector) {
	for _, c := range collectors {
		c.PostCollect(n)
	}
}

func startCollectSingleGoroutine(group *sync.WaitGroup, ctx context.Context, n *NpuCollector) {
	group.Add(1)
	go func() {
		defer group.Done()
		ticker := time.NewTicker(n.updateTime)
		defer ticker.Stop()
		goroutinePreCollect(ChainForSingleGoroutine, n)
		defer goroutinePostCollect(ChainForSingleGoroutine, n)
		for {
			select {
			case <-ctx.Done():
				logger.Info("received the stop signal,stop npu base info collect")
				return
			default:
				chipList := getChipListCache(n)
				for _, c := range ChainForSingleGoroutine {
					c.CollectToCache(n, chipList)
				}
				if _, ok := <-ticker.C; !ok {
					logger.Errorf(tickerFailedPattern, "handling all collectors")
					return
				}
			}
		}
	}()
}

// npuChipInfoInitAtFirstTime When first enter, the cache data is empty,
// need to get the data from the device, and build the cache
func npuChipInfoInitAtFirstTime(n *NpuCollector) {
	npuChipInfoInit.Do(func() {
		_, err := n.cache.Get(npuListCacheKey)
		if err != nil {
			logger.Debug("no cache in first time, start to collect chip list and rebuild cache")

			npuInfo := getNPUChipList(n.Dmgr)
			if err := n.cache.Set(npuListCacheKey, npuInfo, n.cacheTime); err != nil {
				logger.Error(err)
			} else {
				logger.Infof(UpdateCachePattern, npuListCacheKey)
			}
			logger.Debug("rebuild cache successfully")
		}
	})
}

// InitCardInfo init card info
func InitCardInfo(group *sync.WaitGroup, ctx context.Context, n *NpuCollector) {

	group.Add(1)
	go func() {
		defer group.Done()
		ticker := time.NewTicker(updateTimeForCardIds)
		defer ticker.Stop()
		for {
			logger.Info("start to collect npu chip list info")
			select {
			case <-ctx.Done():
				logger.Info("received the stop signal,stop card info collect")
				return
			default:
				npuInfo := getNPUChipList(n.Dmgr)
				if err := n.cache.Set(npuListCacheKey, npuInfo, n.cacheTime); err != nil {
					logger.Error(err)
				} else {
					logger.Infof(UpdateCachePattern, npuListCacheKey)
				}
				if _, ok := <-ticker.C; !ok {
					logger.Errorf(tickerFailedPattern, npuListCacheKey)
					return
				}
			}
		}
	}()
}

func getNPUChipList(dmgr devmanager.DeviceInterface) (npuInfo []HuaWeiAIChip) {
	chipList := make([]HuaWeiAIChip, 0)

	cardNum, cards, err := dmgr.GetCardList()
	if err != nil || cardNum == 0 {
		logger.Errorf("failed to get npu info, error is: %v", err)
		return chipList
	}

	chipListIDs := make([]int32, 0)

	for _, cardID := range cards {
		deviceNum, _ := dmgr.GetDeviceNumInCard(cardID)
		for deviceID := int32(0); deviceID < deviceNum; deviceID++ {
			var chip HuaWeiAIChip
			// get logicID
			logicID, err := dmgr.GetDeviceLogicID(cardID, deviceID)
			if err != nil {
				logger.Errorf("get logic ID of card: %v device:%v failed: %v", cardID, deviceID, err)
				continue
			}

			chip.LogicID = logicID
			chip.CardId = cardID
			chip.MainBoardId = dmgr.GetMainBoardId()

			setPhyId(&chip, dmgr, cardID, deviceID)
			setChipInfo(&chip, dmgr, cardID, deviceID)
			setBoardInfo(&chip, dmgr, cardID, deviceID)
			setVdieID(&chip, dmgr, cardID, deviceID)
			assemblevNPUInfo(dmgr, logicID, &chip)
			setPCIeBusInfo(logicID, dmgr, &chip)
			setElabelInfo(&chip, dmgr, cardID)

			chipList = append(chipList, chip)
			chipListIDs = append(chipListIDs, logicID)
		}
	}

	logger.Debugf("flush chip info list successed,chip num is : %v, chipLogicIDs: %v",
		len(chipList), chipListIDs)
	return chipList
}

func setBoardInfo(chip *HuaWeiAIChip, dmgr devmanager.DeviceInterface, cardID int32, deviceID int32) {
	boardInfo, err := dmgr.GetBoardInfo(chip.LogicID)
	if err != nil {
		logger.Errorf("get board info of card: %v device:%v failed: %v", cardID, deviceID, err)
		boardInfo = common.BoardInfo{}
	}
	chip.BoardInfo = &boardInfo
}
func setVdieID(chip *HuaWeiAIChip, dmgr devmanager.DeviceInterface, cardID int32, deviceID int32) {
	vdieID, err := dmgr.GetDieID(chip.LogicID, dcmi.VDIE)
	if err != nil {
		logger.Debug(err)
	}
	chip.VDieID = vdieID
}

func setPhyId(chip *HuaWeiAIChip, dmgr devmanager.DeviceInterface, cardID int32, deviceID int32) {
	phyID, err := dmgr.GetPhysicIDFromLogicID(chip.LogicID)
	if err != nil {
		logger.Errorf("get phy ID of card: %v device:%v failed: %v", cardID, deviceID, err)
	}
	chip.PhyId = phyID
	chip.DeviceID = phyID
}
func setChipInfo(chip *HuaWeiAIChip, dmgr devmanager.DeviceInterface, cardID int32, deviceID int32) {
	// get chip info
	chipInfo, err := dmgr.GetChipInfo(chip.LogicID)
	if err != nil {
		logger.Errorf("get chip info of card: %v device:%v failed: %v", cardID, deviceID, err)
		chipInfo = &common.ChipInfo{}
	}
	chip.ChipInfo = chipInfo
}

func setPCIeBusInfo(logicID int32, dmgr devmanager.DeviceInterface, hwChip *HuaWeiAIChip) {
	productTypes := dmgr.GetProductTypeArray()
	pcieInfo, err := dmgr.GetPCIeBusInfo(logicID)
	if err != nil {
		if len(productTypes) == 1 && productTypes[0] == common.Atlas200ISoc {
			logger.Debugf("pcie bus info is not supported on %s", common.Atlas200ISoc)
			hwChip.PCIeBusInfo = ""
			return
		}
		logger.Error(err)
		pcieInfo = ""
	}
	hwChip.PCIeBusInfo = pcieInfo
}

func setElabelInfo(chip *HuaWeiAIChip, dmgr devmanager.DeviceInterface, cardID int32) {
	elabelInfo, err := dmgr.GetCardElabelV2(cardID)
	if err != nil {
		logger.Errorf("get elabel info of card: %v failed: %v", cardID, err)
		chip.ElabelInfo = &common.ElabelInfo{SerialNumber: "NA"}
		return
	}
	chip.ElabelInfo = &common.ElabelInfo{
		SerialNumber: elabelInfo.SerialNumber,
	}
}

func assemblevNPUInfo(dmgr devmanager.DeviceInterface, logicID int32, baseChipInfo *HuaWeiAIChip) {
	if dmgr.GetDevType() != api.Ascend310P {
		return
	}
	vDevInfos, err := dmgr.GetVirtualDeviceInfo(logicID)
	if err != nil {
		logger.Warnf("failed to get virtual device info,logicID(%d),err: %v", logicID, err)
		baseChipInfo.VDevInfos = nil
	}
	if vDevInfos.TotalResource.VDevNum == 0 {
		baseChipInfo.VDevInfos = &common.VirtualDevInfo{}
	}
	baseChipInfo.VDevInfos = &vDevInfos
}

// GetChipListWithVNPU get chip list with vnpu
func GetChipListWithVNPU(n *NpuCollector) []HuaWeiAIChip {
	result := make([]HuaWeiAIChip, 0)
	chips := getChipListCache(n)

	for _, chipInfo := range chips {
		isNeedHandleVnpu := n.Dmgr.GetDevType() == api.Ascend310P && chipInfo.VDevInfos != nil &&
			len(chipInfo.VDevInfos.VDevActivityInfo) > 0

		if !isNeedHandleVnpu {
			result = append(result, chipInfo)
			continue
		}

		for _, activityVDev := range chipInfo.VDevInfos.VDevActivityInfo {
			vDevInfo := chipInfo
			activityVDevCopy := activityVDev
			vDevInfo.VDevActivityInfo = &activityVDevCopy
			result = append(result, vDevInfo)
		}
	}

	return result

}
func getChipListCache(n *NpuCollector) []HuaWeiAIChip {
	obj, err := n.cache.Get(npuListCacheKey)
	if err != nil {
		logger.Errorf("get npu chip list from cache failed,err is : %v", err)
		return make([]HuaWeiAIChip, 0)
	}
	if obj == nil {
		logger.LogfWithOptions(logger.ErrorLevel, logger.LogOptions{Domain: "getChipListCache"},
			"there is no chip list info in cache,please check collect logs")
		return make([]HuaWeiAIChip, 0)
	}

	chipList, ok := obj.([]HuaWeiAIChip)
	if !ok {
		logger.Errorf("error npu chip info cache and convert failed,real type is (%T)", obj)
		n.cache.Delete(npuListCacheKey)
		return make([]HuaWeiAIChip, 0)
	}
	// if cache is empty or nil, return empty list
	if len(chipList) == 0 {
		return make([]HuaWeiAIChip, 0)
	}
	return chipList
}
