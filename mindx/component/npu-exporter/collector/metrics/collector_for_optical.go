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

const (
	txPower0 = "Tx_Power0"
	txPower1 = "Tx_Power1"
	txPower2 = "Tx_Power2"
	txPower3 = "Tx_Power3"

	rxPower0 = "Rx_Power0"
	rxPower1 = "Rx_Power1"
	rxPower2 = "Rx_Power2"
	rxPower3 = "Rx_Power3"

	notPresent  = "not present"
	present     = "present"
	temperature = "temperature"
	voltage     = "Vcc"
)

var (

	// optical
	descOpticalState    = colcommon.BuildDesc("npu_chip_optical_state", "the npu interface receive optical-state")
	descOpticalVcc      = colcommon.BuildDesc("npu_chip_optical_vcc", "the npu interface receive optical-vcc")
	descOpticalTemp     = colcommon.BuildDesc("npu_chip_optical_temp", "the npu interface receive optical-temperature")
	descOpticalTxPower0 = colcommon.BuildDesc("npu_chip_optical_tx_power_0", "npu interface receive optical-tx-power-0")
	descOpticalTxPower1 = colcommon.BuildDesc("npu_chip_optical_tx_power_1", "npu interface receive optical-tx-power-1")
	descOpticalTxPower2 = colcommon.BuildDesc("npu_chip_optical_tx_power_2", "npu interface receive optical-tx-power-2")
	descOpticalTxPower3 = colcommon.BuildDesc("npu_chip_optical_tx_power_3", "npu interface receive optical-tx-power-3")

	descOpticalRxPower0 = colcommon.BuildDesc("npu_chip_optical_rx_power_0", "npu interface receive optical-rx-power-0")
	descOpticalRxPower1 = colcommon.BuildDesc("npu_chip_optical_rx_power_1", "npu interface receive optical-rx-power-1")
	descOpticalRxPower2 = colcommon.BuildDesc("npu_chip_optical_rx_power_2", "npu interface receive optical-rx-power-2")
	descOpticalRxPower3 = colcommon.BuildDesc("npu_chip_optical_rx_power_3", "npu interface receive optical-rx-power-3")
)

type opticalCache struct {
	chip      colcommon.HuaWeiAIChip
	timestamp time.Time
	// extInfo indicates the optical module information
	extInfo *common.OpticalInfo
}

// OpticalCollector collect the optical metrics
type OpticalCollector struct {
	colcommon.MetricsCollectorAdapter
}

// IsSupported judge whether the collector is supported
func (c *OpticalCollector) IsSupported(n *colcommon.NpuCollector) bool {
	isSupport := n.Dmgr.IsTrainingCard()
	logForUnSupportDevice(isSupport, n.Dmgr.GetDevType(), colcommon.GetCacheKey(c),
		"only training card supports network related info")
	return isSupport
}

// Describe description of the metric
func (c *OpticalCollector) Describe(ch chan<- *prometheus.Desc) {
	// optical
	ch <- descOpticalState
	ch <- descOpticalTxPower0
	ch <- descOpticalTxPower1
	ch <- descOpticalTxPower2
	ch <- descOpticalTxPower3
	ch <- descOpticalRxPower0
	ch <- descOpticalRxPower1
	ch <- descOpticalRxPower2
	ch <- descOpticalRxPower3
	ch <- descOpticalVcc
	ch <- descOpticalTemp
}

// CollectToCache collect the metric to cache
func (c *OpticalCollector) CollectToCache(n *colcommon.NpuCollector, chipList []colcommon.HuaWeiAIChip) {
	for _, chip := range chipList {
		opticalInfo, err := hccn.GetNPUOpticalInfo(chip.PhyId)
		if err != nil {
			logErrMetricsWithLimit(colcommon.DomainForOptical, chip.PhyId, err)
			continue
		}
		hwlog.ResetErrCnt(colcommon.DomainForOptical, chip.PhyId)
		info := getMainOptInfo(opticalInfo)
		c.LocalCache.Store(chip.PhyId, opticalCache{chip: chip, timestamp: time.Now(), extInfo: info})
	}
	colcommon.UpdateCache[opticalCache](n, colcommon.GetCacheKey(c), &c.LocalCache)
}

// UpdatePrometheus update prometheus metrics
func (c *OpticalCollector) UpdatePrometheus(ch chan<- prometheus.Metric, n *colcommon.NpuCollector,
	containerMap map[int32]container.DevicesInfo, chips []colcommon.HuaWeiAIChip) {

	updateSingleChip := func(chipWithVnpu colcommon.HuaWeiAIChip, cache opticalCache, cardLabel []string) {
		opticalInfo := cache.extInfo
		if opticalInfo == nil {
			return
		}
		timestamp := cache.timestamp
		doUpdateMetricWithValidateNum(ch, timestamp, opticalInfo.OpticalState, cardLabel, descOpticalState)
		doUpdateMetricWithValidateNum(ch, timestamp, opticalInfo.OpticalVcc, cardLabel, descOpticalVcc)
		doUpdateMetricWithValidateNum(ch, timestamp, opticalInfo.OpticalTemp, cardLabel, descOpticalTemp)

		doUpdateMetricWithValidateNum(ch, timestamp, opticalInfo.OpticalTxPower0, cardLabel, descOpticalTxPower0)
		doUpdateMetricWithValidateNum(ch, timestamp, opticalInfo.OpticalTxPower1, cardLabel, descOpticalTxPower1)
		doUpdateMetricWithValidateNum(ch, timestamp, opticalInfo.OpticalTxPower2, cardLabel, descOpticalTxPower2)
		doUpdateMetricWithValidateNum(ch, timestamp, opticalInfo.OpticalTxPower3, cardLabel, descOpticalTxPower3)

		doUpdateMetricWithValidateNum(ch, timestamp, opticalInfo.OpticalRxPower0, cardLabel, descOpticalRxPower0)
		doUpdateMetricWithValidateNum(ch, timestamp, opticalInfo.OpticalRxPower1, cardLabel, descOpticalRxPower1)
		doUpdateMetricWithValidateNum(ch, timestamp, opticalInfo.OpticalRxPower2, cardLabel, descOpticalRxPower2)
		doUpdateMetricWithValidateNum(ch, timestamp, opticalInfo.OpticalRxPower3, cardLabel, descOpticalRxPower3)
	}

	updateFrame[opticalCache](colcommon.GetCacheKey(c), n, containerMap, chips, updateSingleChip)

}

// UpdateTelegraf update telegraf metrics
func (c *OpticalCollector) UpdateTelegraf(fieldsMap map[string]map[string]interface{}, n *colcommon.NpuCollector,
	containerMap map[int32]container.DevicesInfo, chips []colcommon.HuaWeiAIChip) map[string]map[string]interface{} {

	caches := colcommon.GetInfoFromCache[opticalCache](n, colcommon.GetCacheKey(c))
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
		doUpdateTelegrafWithValidateNum(fieldMap, descOpticalState, extInfo.OpticalState, "")
		doUpdateTelegrafWithValidateNum(fieldMap, descOpticalVcc, extInfo.OpticalVcc, "")
		doUpdateTelegrafWithValidateNum(fieldMap, descOpticalTemp, extInfo.OpticalTemp, "")

		doUpdateTelegrafWithValidateNum(fieldMap, descOpticalTxPower0, extInfo.OpticalTxPower0, "")
		doUpdateTelegrafWithValidateNum(fieldMap, descOpticalTxPower1, extInfo.OpticalTxPower1, "")
		doUpdateTelegrafWithValidateNum(fieldMap, descOpticalTxPower2, extInfo.OpticalTxPower2, "")
		doUpdateTelegrafWithValidateNum(fieldMap, descOpticalTxPower3, extInfo.OpticalTxPower3, "")

		doUpdateTelegrafWithValidateNum(fieldMap, descOpticalRxPower0, extInfo.OpticalRxPower0, "")
		doUpdateTelegrafWithValidateNum(fieldMap, descOpticalRxPower1, extInfo.OpticalRxPower1, "")
		doUpdateTelegrafWithValidateNum(fieldMap, descOpticalRxPower2, extInfo.OpticalRxPower2, "")
		doUpdateTelegrafWithValidateNum(fieldMap, descOpticalRxPower3, extInfo.OpticalRxPower3, "")
	}
	return fieldsMap
}

func getMainOptInfo(opticalInfo map[string]string) *common.OpticalInfo {
	mainOpticalInfo := common.OpticalInfo{}
	mainOpticalInfo.OpticalTxPower0 = hccn.GetFloatDataFromStr(opticalInfo[txPower0], txPower0)
	mainOpticalInfo.OpticalTxPower1 = hccn.GetFloatDataFromStr(opticalInfo[txPower1], txPower1)
	mainOpticalInfo.OpticalTxPower2 = hccn.GetFloatDataFromStr(opticalInfo[txPower2], txPower2)
	mainOpticalInfo.OpticalTxPower3 = hccn.GetFloatDataFromStr(opticalInfo[txPower3], txPower3)
	mainOpticalInfo.OpticalRxPower0 = hccn.GetFloatDataFromStr(opticalInfo[rxPower0], rxPower0)
	mainOpticalInfo.OpticalRxPower1 = hccn.GetFloatDataFromStr(opticalInfo[rxPower1], rxPower1)
	mainOpticalInfo.OpticalRxPower2 = hccn.GetFloatDataFromStr(opticalInfo[rxPower2], rxPower2)
	mainOpticalInfo.OpticalRxPower3 = hccn.GetFloatDataFromStr(opticalInfo[rxPower3], rxPower3)
	mainOpticalInfo.OpticalVcc = hccn.GetFloatDataFromStr(opticalInfo[voltage], voltage)
	mainOpticalInfo.OpticalTemp = hccn.GetFloatDataFromStr(opticalInfo[temperature], temperature)
	var optState float64
	if opticalInfo[present] == present {
		optState = 1.0
	} else if opticalInfo[present] == notPresent {
		optState = 0.0
	} else {
		optState = common.RetError
	}
	mainOpticalInfo.OpticalState = optState

	return &mainOpticalInfo
}
