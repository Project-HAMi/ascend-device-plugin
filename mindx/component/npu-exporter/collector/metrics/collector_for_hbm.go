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
	"ascend-common/devmanager"
	"ascend-common/devmanager/common"
	colcommon "huawei.com/npu-exporter/v6/collector/common"
	"huawei.com/npu-exporter/v6/collector/container"
)

var (
	descHbmUsedMemory  = colcommon.BuildDesc("npu_chip_info_hbm_used_memory", "the npu hbm used memory")
	descHbmTotalMemory = colcommon.BuildDesc("npu_chip_info_hbm_total_memory", "the npu hbm total memory")
	descHbmUtilization = colcommon.BuildDesc("npu_chip_info_hbm_utilization", "the npu hbm utilization")
	descHbmTemperature = colcommon.BuildDesc("npu_chip_info_hbm_temperature", "the npu hbm temperature")
	descHbmBWUtil      = colcommon.BuildDesc("npu_chip_info_hbm_bandwidth_utilization", "the npu hbm bandwidth util rate")

	descEccEnableFlag = colcommon.BuildDesc("npu_chip_info_hbm_ecc_enable_flag",
		"whether HBM ecc detection is enabled")
	descEccSingleBitErrorCnt = colcommon.BuildDesc("npu_chip_info_hbm_ecc_single_bit_error_cnt",
		"HBM Single Bit Error Count")
	descEccDoubleBitErrorCnt = colcommon.BuildDesc("npu_chip_info_hbm_ecc_double_bit_error_cnt",
		"HBM Double Bit Error Count")

	descEccTotalSingleBitErrorCnt = colcommon.BuildDesc("npu_chip_info_hbm_ecc_total_single_bit_error_cnt",
		"HBM Single Bit Aggregate Total Err Cnt")
	descEccTotalDoubleBitErrorCnt = colcommon.BuildDesc("npu_chip_info_hbm_ecc_total_double_bit_error_cnt",
		"HBM Double Bit Aggregate Total Err Cnt")
	descEccSingleBitIoslatedPagesCnt = colcommon.BuildDesc("npu_chip_info_hbm_ecc_single_bit_isolated_pages_cnt",
		"HBM Single Bit Isolated Pages Count")
	descEccDoubleBitIoslatedPagesCnt = colcommon.BuildDesc("npu_chip_info_hbm_ecc_double_bit_isolated_pages_cnt",
		"HBM Double Bit Isolated Pages Count")
)

var (
	supportedHbmDevices = map[string]bool{
		api.Ascend910A:  true,
		api.Ascend910B:  true,
		api.Ascend910A3: true,
	}
)

type hbmCache struct {
	chip      colcommon.HuaWeiAIChip
	timestamp time.Time
	// extInfo the hbm info
	extInfo *common.HbmAggregateInfo
	// hbmUtilization the hbm utilization
	hbmUtilization uint32
}

// HbmCollector collects hbm info
type HbmCollector struct {
	colcommon.MetricsCollectorAdapter
}

// IsSupported check whether the collector is supported
func (c *HbmCollector) IsSupported(n *colcommon.NpuCollector) bool {
	isSupport := supportedHbmDevices[n.Dmgr.GetDevType()]
	logForUnSupportDevice(isSupport, n.Dmgr.GetDevType(), colcommon.GetCacheKey(c), "")
	return isSupport
}

// Describe describes all the metrics that will be exposed.
func (c *HbmCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- descHbmUsedMemory
	ch <- descHbmTotalMemory
	ch <- descHbmUtilization
	ch <- descHbmTemperature
	ch <- descHbmBWUtil

	ch <- descEccEnableFlag
	ch <- descEccSingleBitErrorCnt
	ch <- descEccDoubleBitErrorCnt
	ch <- descEccTotalSingleBitErrorCnt
	ch <- descEccTotalDoubleBitErrorCnt
	ch <- descEccSingleBitIoslatedPagesCnt
	ch <- descEccDoubleBitIoslatedPagesCnt
}

// CollectToCache collects hbm info
func (c *HbmCollector) CollectToCache(n *colcommon.NpuCollector, chipList []colcommon.HuaWeiAIChip) {
	for _, chip := range chipList {
		getAllHBMEccInfo(c, chip.LogicID, n.Dmgr, &chip)
	}
	colcommon.UpdateCache[hbmCache](n, colcommon.GetCacheKey(c), &c.LocalCache)
}

// UpdatePrometheus updates the prometheus metrics.
func (c *HbmCollector) UpdatePrometheus(ch chan<- prometheus.Metric, n *colcommon.NpuCollector,
	containerMap map[int32]container.DevicesInfo, chips []colcommon.HuaWeiAIChip) {

	updateSingleChip := func(chipWithVnpu colcommon.HuaWeiAIChip, cache hbmCache, cardLabel []string) {
		extInfo := cache.extInfo
		if extInfo == nil {
			return
		}
		timestamp := cache.timestamp
		doUpdateMetricWithValidateNum(ch, timestamp, float64(cache.hbmUtilization), cardLabel, descHbmUtilization)

		c.updateHbmInfo(ch, cache, cardLabel, containerMap, chipWithVnpu)

		eccInfo := extInfo.ECCInfo
		updateHbmEccInfo(ch, eccInfo, timestamp, cardLabel)
	}

	updateFrame[hbmCache](colcommon.GetCacheKey(c), n, containerMap, chips, updateSingleChip)
}

// UpdateTelegraf updates the telegraf metrics.
func (c *HbmCollector) UpdateTelegraf(fieldsMap map[string]map[string]interface{}, n *colcommon.NpuCollector,
	containerMap map[int32]container.DevicesInfo, chips []colcommon.HuaWeiAIChip) map[string]map[string]interface{} {
	caches := colcommon.GetInfoFromCache[hbmCache](n, colcommon.GetCacheKey(c))
	for _, chip := range chips {
		cache, ok := caches[chip.PhyId]
		if !ok {
			continue
		}
		fieldMap := getFieldMap(fieldsMap, cache.chip.LogicID)

		extInfo := cache.extInfo
		if extInfo == nil {
			continue
		}

		doUpdateTelegrafWithValidateNum(fieldMap, descHbmUtilization, float64(cache.hbmUtilization), "")

		hbmInfo := extInfo.HbmInfo
		if hbmInfo != nil {
			doUpdateTelegraf(fieldMap, descHbmUsedMemory, hbmInfo.Usage, "")
			doUpdateTelegraf(fieldMap, descHbmTotalMemory, hbmInfo.MemorySize, "")
			doUpdateTelegraf(fieldMap, descHbmTemperature, hbmInfo.Temp, "")
			doUpdateTelegraf(fieldMap, descHbmBWUtil, hbmInfo.BandWidthUtilRate, "")
		}

		eccInfo := extInfo.ECCInfo
		if eccInfo != nil {
			doUpdateTelegraf(fieldMap, descEccEnableFlag, eccInfo.EnableFlag, "")
			doUpdateTelegraf(fieldMap, descEccSingleBitErrorCnt, eccInfo.SingleBitErrorCnt, "")
			doUpdateTelegraf(fieldMap, descEccDoubleBitErrorCnt, eccInfo.DoubleBitErrorCnt, "")
			doUpdateTelegraf(fieldMap, descEccTotalSingleBitErrorCnt, eccInfo.TotalSingleBitErrorCnt, "")
			doUpdateTelegraf(fieldMap, descEccTotalDoubleBitErrorCnt, eccInfo.TotalDoubleBitErrorCnt, "")
			doUpdateTelegraf(fieldMap, descEccSingleBitIoslatedPagesCnt, eccInfo.SingleBitIsolatedPagesCnt, "")
			doUpdateTelegraf(fieldMap, descEccDoubleBitIoslatedPagesCnt, eccInfo.DoubleBitIsolatedPagesCnt, "")

		}
	}
	return fieldsMap

}

func getAllHBMEccInfo(c *HbmCollector, logicID int32, dmgr devmanager.DeviceInterface, chip *colcommon.HuaWeiAIChip) {

	hbmInfo := &common.HbmAggregateInfo{}
	var utilizationRate uint32
	var err error
	hbmInfo.HbmInfo, err = dmgr.GetDeviceHbmInfo(logicID)
	handleErr(err, colcommon.DomainForHBM, logicID)

	utilizationRate, err = dmgr.GetDeviceUtilizationRate(logicID, common.HbmUtilization)
	handleErr(err, colcommon.DomainForHbmUtilization, logicID)

	hbmInfo.ECCInfo, err = dmgr.GetDeviceEccInfo(logicID, common.DcmiDeviceTypeHBM)
	handleErr(err, colcommon.DomainForHBMECC, logicID)
	c.LocalCache.Store(chip.PhyId, hbmCache{
		chip:           *chip,
		timestamp:      time.Now(),
		extInfo:        hbmInfo,
		hbmUtilization: utilizationRate},
	)
}

func updateHbmEccInfo(ch chan<- prometheus.Metric, eccInfo *common.ECCInfo, timestamp time.Time, cardLabel []string) {
	if eccInfo == nil {
		return
	}
	doUpdateMetric(ch, timestamp, eccInfo.EnableFlag, cardLabel, descEccEnableFlag)
	doUpdateMetric(ch, timestamp, eccInfo.SingleBitErrorCnt, cardLabel, descEccSingleBitErrorCnt)
	doUpdateMetric(ch, timestamp, eccInfo.DoubleBitErrorCnt, cardLabel, descEccDoubleBitErrorCnt)
	doUpdateMetric(ch, timestamp, eccInfo.TotalSingleBitErrorCnt, cardLabel, descEccTotalSingleBitErrorCnt)
	doUpdateMetric(ch, timestamp, eccInfo.TotalDoubleBitErrorCnt, cardLabel, descEccTotalDoubleBitErrorCnt)
	doUpdateMetric(ch, timestamp, eccInfo.SingleBitIsolatedPagesCnt, cardLabel, descEccSingleBitIoslatedPagesCnt)
	doUpdateMetric(ch, timestamp, eccInfo.DoubleBitIsolatedPagesCnt, cardLabel, descEccDoubleBitIoslatedPagesCnt)
}

func (c *HbmCollector) updateHbmInfo(ch chan<- prometheus.Metric, cache hbmCache, cardLabel []string,
	containerMap map[int32]container.DevicesInfo, chipWithVnpu colcommon.HuaWeiAIChip) {
	hbmInfo := cache.extInfo
	if hbmInfo == nil || hbmInfo.HbmInfo == nil {
		return
	}
	timestamp := cache.timestamp
	doUpdateMetric(ch, timestamp, hbmInfo.Usage, cardLabel, descHbmUsedMemory)
	doUpdateMetric(ch, timestamp, hbmInfo.MemorySize, cardLabel, descHbmTotalMemory)
	doUpdateMetric(ch, timestamp, hbmInfo.Temp, cardLabel, descHbmTemperature)
	doUpdateMetric(ch, timestamp, hbmInfo.BandWidthUtilRate, cardLabel, descHbmBWUtil)

	// vnpu not support this metrics
	vDevActivityInfo := chipWithVnpu.VDevActivityInfo
	if vDevActivityInfo != nil && common.IsValidVDevID(vDevActivityInfo.VDevID) {
		return
	}

	containerNameArray := getContainerNameArray(geenContainerInfo(&chipWithVnpu, containerMap))
	if c.Is910Series && len(containerNameArray) == colcommon.ContainerNameLen {
		doUpdateMetric(ch, timestamp, hbmInfo.MemorySize, cardLabel, npuCtrTotalMemory)
		doUpdateMetric(ch, timestamp, hbmInfo.Usage, cardLabel, npuCtrUsedMemory)
	}
}
