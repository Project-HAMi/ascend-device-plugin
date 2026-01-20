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

	"ascend-common/common-utils/hwlog"
	"ascend-common/devmanager/common"
	"ascend-common/devmanager/hccn"
	colcommon "huawei.com/npu-exporter/v6/collector/common"
	"huawei.com/npu-exporter/v6/collector/container"
)

var (
	// bandwidth
	descBandwidthTx = colcommon.BuildDesc("npu_chip_info_bandwidth_tx",
		"the npu interface transport speed, unit is 'MB/s'")
	descBandwidthRx = colcommon.BuildDesc("npu_chip_info_bandwidth_rx",
		"the npu interface receive speed, unit is 'MB/s'")

	// linkspeed
	npuChipLinkSpeed = colcommon.BuildDesc("npu_chip_link_speed",
		"the npu interface receive link speed, unit is 'Mb/s'")

	// linkupNum
	npuChipLinkUpNum = colcommon.BuildDesc("npu_chip_link_up_num", "the npu interface receive link-up num")

	// linkstatus
	descLinkStatus = colcommon.BuildDesc("npu_chip_info_link_status", "the npu link status")
)

type netInfoCache struct {
	chip      colcommon.HuaWeiAIChip
	timestamp time.Time
	extInfo   *common.NpuNetInfo
}

// NetworkCollector collects the network info
type NetworkCollector struct {
	colcommon.MetricsCollectorAdapter
}

// IsSupported check if the collector is supported
func (c *NetworkCollector) IsSupported(n *colcommon.NpuCollector) bool {
	isSupport := n.Dmgr.IsTrainingCard()
	logForUnSupportDevice(isSupport, n.Dmgr.GetDevType(), colcommon.GetCacheKey(c),
		"only training card supports network related info")
	return isSupport
}

// Describe description of the metric
func (c *NetworkCollector) Describe(ch chan<- *prometheus.Desc) {
	// bandwidth
	ch <- descBandwidthTx
	ch <- descBandwidthRx
	// linkspeed
	ch <- npuChipLinkSpeed
	// linkupNum
	ch <- npuChipLinkUpNum
	// linkstatus
	ch <- descLinkStatus
}

// CollectToCache collect the metric to cache
func (c *NetworkCollector) CollectToCache(n *colcommon.NpuCollector, chipList []colcommon.HuaWeiAIChip) {
	for _, chip := range chipList {
		netInfo := collectNetworkInfo(chip.PhyId)
		c.LocalCache.Store(chip.PhyId, netInfoCache{chip: chip, timestamp: time.Now(), extInfo: &netInfo})
	}
	colcommon.UpdateCache[netInfoCache](n, colcommon.GetCacheKey(c), &c.LocalCache)
}

// UpdatePrometheus update prometheus metrics
func (c *NetworkCollector) UpdatePrometheus(ch chan<- prometheus.Metric, n *colcommon.NpuCollector,
	containerMap map[int32]container.DevicesInfo, chips []colcommon.HuaWeiAIChip) {

	updateSingleChip := func(chipWithVnpu colcommon.HuaWeiAIChip, cache netInfoCache, cardLabel []string) {
		netInfo := cache.extInfo
		if netInfo == nil {
			return
		}
		time := cache.timestamp
		if validateNotNilForEveryElement(netInfo.BandwidthInfo) {
			doUpdateMetricWithValidateNum(ch, time, netInfo.BandwidthInfo.TxValue, cardLabel, descBandwidthTx)
			doUpdateMetricWithValidateNum(ch, time, netInfo.BandwidthInfo.RxValue, cardLabel, descBandwidthRx)
		}
		if validateNotNilForEveryElement(netInfo.LinkSpeedInfo) {
			doUpdateMetricWithValidateNum(ch, time, netInfo.LinkSpeedInfo.Speed, cardLabel, npuChipLinkSpeed)
		}
		if validateNotNilForEveryElement(netInfo.LinkStatInfo) {
			doUpdateMetricWithValidateNum(ch, time, netInfo.LinkStatInfo.LinkUPNum, cardLabel, npuChipLinkUpNum)
		}
		if validateNotNilForEveryElement(netInfo.LinkStatusInfo) {
			doUpdateMetricWithValidateNum(ch, time, float64(hccn.GetLinkStatusCode(netInfo.LinkStatusInfo.LinkState)),
				cardLabel, descLinkStatus)
		}
	}
	updateFrame[netInfoCache](colcommon.GetCacheKey(c), n, containerMap, chips, updateSingleChip)
}

// UpdateTelegraf update telegraf metrics
func (c *NetworkCollector) UpdateTelegraf(fieldsMap map[string]map[string]interface{}, n *colcommon.NpuCollector,
	containerMap map[int32]container.DevicesInfo, chips []colcommon.HuaWeiAIChip) map[string]map[string]interface{} {

	caches := colcommon.GetInfoFromCache[netInfoCache](n, colcommon.GetCacheKey(c))
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
		if validateNotNilForEveryElement(extInfo.BandwidthInfo) {
			doUpdateTelegrafWithValidateNum(fieldMap, descBandwidthTx, extInfo.BandwidthInfo.TxValue, "")
			doUpdateTelegrafWithValidateNum(fieldMap, descBandwidthRx, extInfo.BandwidthInfo.RxValue, "")
		}
		if validateNotNilForEveryElement(extInfo.LinkSpeedInfo) {
			doUpdateTelegrafWithValidateNum(fieldMap, npuChipLinkSpeed, extInfo.LinkSpeedInfo.Speed, "")
		}
		if validateNotNilForEveryElement(extInfo.LinkStatInfo) {
			doUpdateTelegrafWithValidateNum(fieldMap, npuChipLinkUpNum, extInfo.LinkStatInfo.LinkUPNum, "")
		}
		if validateNotNilForEveryElement(extInfo.LinkStatusInfo) {
			doUpdateTelegrafWithValidateNum(fieldMap, descLinkStatus,
				float64(hccn.GetLinkStatusCode(extInfo.LinkStatusInfo.LinkState)), "")
		}
	}
	return fieldsMap
}

func collectNetworkInfo(phyID int32) common.NpuNetInfo {
	newNetInfo := common.NpuNetInfo{}

	newNetInfo.LinkStatusInfo = &common.LinkStatusInfo{}
	if linkState, err := hccn.GetNPULinkStatus(phyID); err == nil {
		newNetInfo.LinkStatusInfo.LinkState = linkState
		hwlog.ResetErrCnt(colcommon.DomainForLinkState, phyID)
	} else {
		logErrMetricsWithLimit(colcommon.DomainForLinkState, phyID, err)
		newNetInfo.LinkStatusInfo.LinkState = colcommon.Abnormal
	}

	if tx, rx, err := hccn.GetNPUInterfaceTraffic(phyID); err == nil {
		newNetInfo.BandwidthInfo = &common.BandwidthInfo{}
		newNetInfo.BandwidthInfo.RxValue = rx
		newNetInfo.BandwidthInfo.TxValue = tx
		hwlog.ResetErrCnt(colcommon.DomainForBandwidth, phyID)
	} else {
		newNetInfo.BandwidthInfo = nil
		logErrMetricsWithLimit(colcommon.DomainForBandwidth, phyID, err)
	}
	if linkUpNum, err := hccn.GetNPULinkUpNum(phyID); err == nil {
		newNetInfo.LinkStatInfo = &common.LinkStatInfo{}
		newNetInfo.LinkStatInfo.LinkUPNum = float64(linkUpNum)
		hwlog.ResetErrCnt(colcommon.DomainForLinkStat, phyID)
	} else {
		newNetInfo.LinkStatInfo = nil
		logErrMetricsWithLimit(colcommon.DomainForLinkStat, phyID, err)
	}

	if speed, err := hccn.GetNPULinkSpeed(phyID); err == nil {
		newNetInfo.LinkSpeedInfo = &common.LinkSpeedInfo{}
		newNetInfo.LinkSpeedInfo.Speed = float64(speed)
		hwlog.ResetErrCnt(colcommon.DomainForLinkSpeed, phyID)
	} else {
		newNetInfo.LinkSpeedInfo = nil
		logErrMetricsWithLimit(colcommon.DomainForLinkSpeed, phyID, err)
	}

	return newNetInfo
}
