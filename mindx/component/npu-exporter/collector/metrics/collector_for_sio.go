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
)

var (
	descSioCrcTxErrCnt = colcommon.BuildDesc("npu_chip_info_sio_crc_tx_err_cnt",
		"sio transmitted error count between die")
	descSioCrcRxErrCnt = colcommon.BuildDesc("npu_chip_info_sio_crc_rx_err_cnt",
		"sio received error count between die")
)
var (
	supportedSioDevices = map[string]bool{
		api.Ascend910A3: true,
	}
)

type sioCache struct {
	chip      colcommon.HuaWeiAIChip
	timestamp time.Time
	// extInfo sio status between dies, only support super pod
	extInfo *common.SioCrcErrStatisticInfo
}

// SioCollector collect sio info
type SioCollector struct {
	colcommon.MetricsCollectorAdapter
}

// IsSupported check whether the collector is supported
func (c *SioCollector) IsSupported(n *colcommon.NpuCollector) bool {
	isSupport := supportedSioDevices[n.Dmgr.GetDevType()]
	logForUnSupportDevice(isSupport, n.Dmgr.GetDevType(), colcommon.GetCacheKey(c),
		"sio information cannot be queried.")
	return isSupport
}

// Describe description of the metric
func (c *SioCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- descSioCrcTxErrCnt
	ch <- descSioCrcRxErrCnt
}

// CollectToCache collect the metric to cache
func (c *SioCollector) CollectToCache(n *colcommon.NpuCollector, chipList []colcommon.HuaWeiAIChip) {
	for _, chip := range chipList {
		logicID := chip.LogicID
		sioInfo, err := n.Dmgr.GetSioInfo(logicID)
		if err != nil {
			logErrMetricsWithLimit(colcommon.DomainForSio, logicID, err)
			continue
		}
		hwlog.ResetErrCnt(colcommon.DomainForSio, logicID)

		c.LocalCache.Store(chip.PhyId, sioCache{chip: chip, timestamp: time.Now(), extInfo: sioInfo})
	}
	colcommon.UpdateCache[sioCache](n, colcommon.GetCacheKey(c), &c.LocalCache)
}

// UpdatePrometheus update prometheus metrics
func (c *SioCollector) UpdatePrometheus(ch chan<- prometheus.Metric, n *colcommon.NpuCollector,
	containerMap map[int32]container.DevicesInfo, chips []colcommon.HuaWeiAIChip) {

	updateSingleChip := func(chipWithVnpu colcommon.HuaWeiAIChip, cache sioCache, cardLabel []string) {
		extInfo := cache.extInfo
		if extInfo == nil {
			return
		}
		doUpdateMetric(ch, cache.timestamp, extInfo.TxErrCnt, cardLabel, descSioCrcTxErrCnt)
		doUpdateMetric(ch, cache.timestamp, extInfo.RxErrCnt, cardLabel, descSioCrcRxErrCnt)
	}
	updateFrame[sioCache](colcommon.GetCacheKey(c), n, containerMap, chips, updateSingleChip)
}

// UpdateTelegraf update telegraf metrics
func (c *SioCollector) UpdateTelegraf(fieldsMap map[string]map[string]interface{}, n *colcommon.NpuCollector,
	containerMap map[int32]container.DevicesInfo, chips []colcommon.HuaWeiAIChip) map[string]map[string]interface{} {

	caches := colcommon.GetInfoFromCache[sioCache](n, colcommon.GetCacheKey(c))
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

		doUpdateTelegraf(fieldMap, descSioCrcTxErrCnt, extInfo.TxErrCnt, "")
		doUpdateTelegraf(fieldMap, descSioCrcRxErrCnt, extInfo.RxErrCnt, "")
	}
	return fieldsMap
}
