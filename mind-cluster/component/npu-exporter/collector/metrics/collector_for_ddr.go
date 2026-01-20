/* Copyright(C) 2025-2025. Huawei Technologies Co.,Ltd. All rights reserved.
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

// Package metrics for general collector
package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"ascend-common/api"
	"ascend-common/common-utils/hwlog"
	"ascend-common/devmanager/common"
	colcommon "huawei.com/npu-exporter/v6/collector/common"
	"huawei.com/npu-exporter/v6/collector/container"
	"huawei.com/npu-exporter/v6/utils/logger"
)

var (
	descTotalMemory = colcommon.BuildDesc("npu_chip_info_total_memory", "the npu total memory")
	descUsedMemory  = colcommon.BuildDesc("npu_chip_info_used_memory", "the npu used memory")

	notSupportedDdrDevices = map[string]bool{
		api.Ascend910B:  true,
		api.Ascend910A3: true,
	}
)

type ddrCache struct {
	chip      colcommon.HuaWeiAIChip
	timestamp time.Time
	// extInfo the memoryInfo of the chip
	extInfo *common.MemoryInfo
}

// DdrCollector collect ddr info
type DdrCollector struct {
	colcommon.MetricsCollectorAdapter
}

// IsSupported check whether the metric is supported
func (c *DdrCollector) IsSupported(n *colcommon.NpuCollector) bool {
	isSupport := !notSupportedDdrDevices[n.Dmgr.GetDevType()]
	logForUnSupportDevice(isSupport, n.Dmgr.GetDevType(), colcommon.GetCacheKey(c),
		"there is no DDR module. DDR information cannot be queried.")
	return isSupport
}

// Describe description of the metric
func (c *DdrCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- descTotalMemory
	ch <- descUsedMemory
}

// CollectToCache collect the metric to cache
func (c *DdrCollector) CollectToCache(n *colcommon.NpuCollector, chipList []colcommon.HuaWeiAIChip) {

	for _, chip := range chipList {
		logicID := chip.LogicID
		mem, err := n.Dmgr.GetDeviceMemoryInfo(logicID)
		if err != nil {
			logErrMetricsWithLimit(colcommon.DomainForDDR, logicID, err)
			continue
		}
		hwlog.ResetErrCnt(colcommon.DomainForDDR, logicID)

		c.LocalCache.Store(chip.PhyId, ddrCache{chip: chip, timestamp: time.Now(), extInfo: mem})
	}
	colcommon.UpdateCache[ddrCache](n, colcommon.GetCacheKey(c), &c.LocalCache)

}

// UpdatePrometheus update prometheus metrics
func (c *DdrCollector) UpdatePrometheus(ch chan<- prometheus.Metric, n *colcommon.NpuCollector,
	containerMap map[int32]container.DevicesInfo, chips []colcommon.HuaWeiAIChip) {

	updateSingleChip := func(chipWithVnpu colcommon.HuaWeiAIChip, cache ddrCache, cardLabel []string) {
		extInfo := cache.extInfo
		if extInfo == nil {
			return
		}
		memorySize := extInfo.MemorySize
		memoryAvailable := extInfo.MemoryAvailable

		doUpdateMetric(ch, cache.timestamp, memorySize, cardLabel, descTotalMemory)
		doUpdateMetric(ch, cache.timestamp, memorySize-memoryAvailable, cardLabel, descUsedMemory)

		// vnpu not support this metrics
		vDevActivityInfo := chipWithVnpu.VDevActivityInfo
		if vDevActivityInfo != nil && common.IsValidVDevID(vDevActivityInfo.VDevID) {
			return
		}

		containerNameArray := getContainerNameArray(geenContainerInfo(&chipWithVnpu, containerMap))
		if !c.Is910Series && len(containerNameArray) == colcommon.ContainerNameLen {
			doUpdateMetric(ch, cache.timestamp, memorySize, cardLabel, npuCtrTotalMemory)
			doUpdateMetric(ch, cache.timestamp, memorySize-memoryAvailable, cardLabel, npuCtrUsedMemory)
		}
	}

	updateFrame[ddrCache](colcommon.GetCacheKey(c), n, containerMap, chips, updateSingleChip)
}

// UpdateTelegraf update telegraf metrics
func (c *DdrCollector) UpdateTelegraf(fieldsMap map[string]map[string]interface{}, n *colcommon.NpuCollector,
	containerMap map[int32]container.DevicesInfo, chips []colcommon.HuaWeiAIChip) map[string]map[string]interface{} {

	caches := colcommon.GetInfoFromCache[ddrCache](n, colcommon.GetCacheKey(c))
	for _, chip := range chips {
		cache, ok := caches[chip.PhyId]
		if !ok {
			logger.Debugf("cacheKey(%v) not found", chip.PhyId)
			continue
		}
		fieldMap := getFieldMap(fieldsMap, cache.chip.LogicID)

		memoryInfo := cache.extInfo
		if memoryInfo == nil {
			logger.Debugf("info in cache is nil,cacheKey(%v)", chip.PhyId)
			continue
		}
		memorySize := memoryInfo.MemorySize
		memoryAvailable := memoryInfo.MemoryAvailable

		doUpdateTelegraf(fieldMap, descTotalMemory, memorySize, "")
		doUpdateTelegraf(fieldMap, descUsedMemory, memorySize-memoryAvailable, "")

	}
	return fieldsMap
}
