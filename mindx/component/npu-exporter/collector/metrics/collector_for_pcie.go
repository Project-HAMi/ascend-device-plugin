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

const (
	pcieBwType = "pcie_bw_type"
	avgPcieBw  = "avgPcieBw"
	minPcieBw  = "minPcieBw"
	maxPcieBw  = "maxPcieBw"

	avgPostfix = "_avgPcieBw"
	minPostfix = "_minPcieBw"
	maxPostfix = "_maxPcieBw"
)

var (
	pcieBwLabel = append(colcommon.CardLabel, pcieBwType)

	descRxPBW = colcommon.BuildDescWithLabel("npu_chip_info_pcie_rx_p_bw",
		"the npu write bw to remote‘s speed, unit is 'MB/ms'", pcieBwLabel)

	descRxNpBW = colcommon.BuildDescWithLabel("npu_chip_info_pcie_rx_np_bw",
		"the npu read bw's speed from remote, unit is 'MB/ms'", pcieBwLabel)

	descRxCplBW = colcommon.BuildDescWithLabel("npu_chip_info_pcie_rx_cpl_bw",
		"the npu reply remote read operate cpl's speed, unit is 'MB/ms'", pcieBwLabel)

	descTxPBW = colcommon.BuildDescWithLabel("npu_chip_info_pcie_tx_p_bw",
		"the npu receive remote write operate's speed, unit is 'MB/ms'", pcieBwLabel)

	descTxNpBW = colcommon.BuildDescWithLabel("npu_chip_info_pcie_tx_np_bw",
		"the npu receive remote read operate's speed, unit is 'MB/ms'", pcieBwLabel)

	descTxCplBW = colcommon.BuildDescWithLabel("npu_chip_info_pcie_tx_cpl_bw",
		"the npu read cpl's responese bw speed from remote, unit is 'MB/ms'", pcieBwLabel)
)
var (
	supportedPcieDevices = map[string]bool{
		api.Ascend910B: true,
	}
)

type pcieCache struct {
	chip      colcommon.HuaWeiAIChip
	timestamp time.Time
	// extInfo pcie transport and receive bandwidth, have six metrics
	extInfo *common.PCIEBwStat
}

// PcieCollector collect pcie info
type PcieCollector struct {
	colcommon.MetricsCollectorAdapter
}

// IsSupported check whether the collector is supported
func (c *PcieCollector) IsSupported(n *colcommon.NpuCollector) bool {
	// only 910A2 supports pcie info
	isSupport := supportedPcieDevices[n.Dmgr.GetDevType()]
	logForUnSupportDevice(isSupport, n.Dmgr.GetDevType(), colcommon.GetCacheKey(c), "")
	return isSupport
}

// Describe description of the metric
func (c *PcieCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- descRxPBW
	ch <- descTxPBW
	ch <- descRxNpBW
	ch <- descTxNpBW
	ch <- descRxCplBW
	ch <- descTxCplBW
}

// CollectToCache collect the metric to cache
func (c *PcieCollector) CollectToCache(n *colcommon.NpuCollector, chipList []colcommon.HuaWeiAIChip) {
	for _, chip := range chipList {
		pcieBwInfo, err := n.Dmgr.GetPCIEBandwidth(chip.LogicID, common.ProfilingTime)
		if err != nil {
			logErrMetricsWithLimit(colcommon.DomainForPcieBandwidth, chip.LogicID, err)
			continue
		}
		hwlog.ResetErrCnt(colcommon.DomainForPcieBandwidth, chip.LogicID)
		c.LocalCache.Store(chip.PhyId, pcieCache{chip: chip, timestamp: time.Now(), extInfo: &pcieBwInfo})
	}
	colcommon.UpdateCache[pcieCache](n, colcommon.GetCacheKey(c), &c.LocalCache)
}

// UpdatePrometheus update prometheus metrics
func (c *PcieCollector) UpdatePrometheus(ch chan<- prometheus.Metric, n *colcommon.NpuCollector,
	containerMap map[int32]container.DevicesInfo, chips []colcommon.HuaWeiAIChip) {

	updateSingleChip := func(chipWithVnpu colcommon.HuaWeiAIChip, cache pcieCache, cardLabel []string) {
		pcieBwInfo := cache.extInfo
		if pcieBwInfo == nil {
			return
		}

		if cache.chip.VDevActivityInfo != nil && common.IsValidVDevID(cache.chip.VDevActivityInfo.VDevID) {
			logger.Debug("vnpu does not supports pcie info query")
			return
		}

		timestamp := cache.timestamp

		updateAvgPcieBwInfo(ch, timestamp, pcieBwInfo, cardLabel)
		updateMinPcieBwInfo(ch, timestamp, pcieBwInfo, cardLabel)
		updateMaxPcieBwInfo(ch, timestamp, pcieBwInfo, cardLabel)
	}

	updateFrame[pcieCache](colcommon.GetCacheKey(c), n, containerMap, chips, updateSingleChip)

}

// UpdateTelegraf update telegraf metrics
func (c *PcieCollector) UpdateTelegraf(fieldsMap map[string]map[string]interface{}, n *colcommon.NpuCollector,
	containerMap map[int32]container.DevicesInfo, chips []colcommon.HuaWeiAIChip) map[string]map[string]interface{} {

	caches := colcommon.GetInfoFromCache[pcieCache](n, colcommon.GetCacheKey(c))
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
		doUpdateTelegraf(fieldMap, descTxPBW, extInfo.PcieTxPBw.PcieAvgBw, avgPostfix)
		doUpdateTelegraf(fieldMap, descTxNpBW, extInfo.PcieTxNPBw.PcieAvgBw, avgPostfix)
		doUpdateTelegraf(fieldMap, descTxCplBW, extInfo.PcieTxCPLBw.PcieAvgBw, avgPostfix)
		doUpdateTelegraf(fieldMap, descRxPBW, extInfo.PcieRxPBw.PcieAvgBw, avgPostfix)
		doUpdateTelegraf(fieldMap, descRxNpBW, extInfo.PcieRxNPBw.PcieAvgBw, avgPostfix)
		doUpdateTelegraf(fieldMap, descRxCplBW, extInfo.PcieRxCPLBw.PcieAvgBw, avgPostfix)

		doUpdateTelegraf(fieldMap, descTxPBW, extInfo.PcieTxPBw.PcieMinBw, minPostfix)
		doUpdateTelegraf(fieldMap, descTxNpBW, extInfo.PcieTxNPBw.PcieMinBw, minPostfix)
		doUpdateTelegraf(fieldMap, descTxCplBW, extInfo.PcieTxCPLBw.PcieMinBw, minPostfix)
		doUpdateTelegraf(fieldMap, descRxPBW, extInfo.PcieRxPBw.PcieMinBw, minPostfix)
		doUpdateTelegraf(fieldMap, descRxNpBW, extInfo.PcieRxNPBw.PcieMinBw, minPostfix)
		doUpdateTelegraf(fieldMap, descRxCplBW, extInfo.PcieRxCPLBw.PcieMinBw, minPostfix)

		doUpdateTelegraf(fieldMap, descTxPBW, extInfo.PcieTxPBw.PcieMaxBw, maxPostfix)
		doUpdateTelegraf(fieldMap, descTxNpBW, extInfo.PcieTxNPBw.PcieMaxBw, maxPostfix)
		doUpdateTelegraf(fieldMap, descTxCplBW, extInfo.PcieTxCPLBw.PcieMaxBw, maxPostfix)
		doUpdateTelegraf(fieldMap, descRxPBW, extInfo.PcieRxPBw.PcieMaxBw, maxPostfix)
		doUpdateTelegraf(fieldMap, descRxNpBW, extInfo.PcieRxNPBw.PcieMaxBw, maxPostfix)
		doUpdateTelegraf(fieldMap, descRxCplBW, extInfo.PcieRxCPLBw.PcieMaxBw, maxPostfix)

	}
	return fieldsMap
}

func pcieBwLabelVal(cardLabels []string, pcieBwType string) []string {
	return append(cardLabels, pcieBwType)
}

func metricWithPcieBw(labelsVal []string, metrics *prometheus.Desc, val float64, valType string) prometheus.Metric {
	return prometheus.MustNewConstMetric(metrics, prometheus.GaugeValue, val, pcieBwLabelVal(labelsVal, valType)...)
}

func updateAvgPcieBwInfo(ch chan<- prometheus.Metric, timestamp time.Time, pcieBwInfo *common.PCIEBwStat,
	cardLabel []string) {
	ch <- prometheus.NewMetricWithTimestamp(timestamp,
		metricWithPcieBw(cardLabel, descTxPBW, float64(pcieBwInfo.PcieTxPBw.PcieAvgBw), avgPcieBw))
	ch <- prometheus.NewMetricWithTimestamp(timestamp,
		metricWithPcieBw(cardLabel, descTxNpBW, float64(pcieBwInfo.PcieTxNPBw.PcieAvgBw), avgPcieBw))
	ch <- prometheus.NewMetricWithTimestamp(timestamp,
		metricWithPcieBw(cardLabel, descTxCplBW, float64(pcieBwInfo.PcieTxCPLBw.PcieAvgBw), avgPcieBw))
	ch <- prometheus.NewMetricWithTimestamp(timestamp,
		metricWithPcieBw(cardLabel, descRxPBW, float64(pcieBwInfo.PcieRxPBw.PcieAvgBw), avgPcieBw))
	ch <- prometheus.NewMetricWithTimestamp(timestamp,
		metricWithPcieBw(cardLabel, descRxNpBW, float64(pcieBwInfo.PcieRxNPBw.PcieAvgBw), avgPcieBw))
	ch <- prometheus.NewMetricWithTimestamp(timestamp,
		metricWithPcieBw(cardLabel, descRxCplBW, float64(pcieBwInfo.PcieRxCPLBw.PcieAvgBw), avgPcieBw))
}

func updateMinPcieBwInfo(ch chan<- prometheus.Metric, timestamp time.Time, pcieBwInfo *common.PCIEBwStat,
	cardLabel []string) {
	ch <- prometheus.NewMetricWithTimestamp(timestamp,
		metricWithPcieBw(cardLabel, descTxPBW, float64(pcieBwInfo.PcieTxPBw.PcieMinBw), minPcieBw))
	ch <- prometheus.NewMetricWithTimestamp(timestamp,
		metricWithPcieBw(cardLabel, descTxNpBW, float64(pcieBwInfo.PcieTxNPBw.PcieMinBw), minPcieBw))
	ch <- prometheus.NewMetricWithTimestamp(timestamp,
		metricWithPcieBw(cardLabel, descTxCplBW, float64(pcieBwInfo.PcieTxCPLBw.PcieMinBw), minPcieBw))
	ch <- prometheus.NewMetricWithTimestamp(timestamp,
		metricWithPcieBw(cardLabel, descRxPBW, float64(pcieBwInfo.PcieRxPBw.PcieMinBw), minPcieBw))
	ch <- prometheus.NewMetricWithTimestamp(timestamp,
		metricWithPcieBw(cardLabel, descRxNpBW, float64(pcieBwInfo.PcieRxNPBw.PcieMinBw), minPcieBw))
	ch <- prometheus.NewMetricWithTimestamp(timestamp,
		metricWithPcieBw(cardLabel, descRxCplBW, float64(pcieBwInfo.PcieRxCPLBw.PcieMinBw), minPcieBw))
}

func updateMaxPcieBwInfo(ch chan<- prometheus.Metric, timestamp time.Time, pcieBwInfo *common.PCIEBwStat,
	cardLabel []string) {
	ch <- prometheus.NewMetricWithTimestamp(timestamp,
		metricWithPcieBw(cardLabel, descTxPBW, float64(pcieBwInfo.PcieTxPBw.PcieMaxBw), maxPcieBw))
	ch <- prometheus.NewMetricWithTimestamp(timestamp,
		metricWithPcieBw(cardLabel, descTxNpBW, float64(pcieBwInfo.PcieTxNPBw.PcieMaxBw), maxPcieBw))
	ch <- prometheus.NewMetricWithTimestamp(timestamp,
		metricWithPcieBw(cardLabel, descTxCplBW, float64(pcieBwInfo.PcieTxCPLBw.PcieMaxBw), maxPcieBw))
	ch <- prometheus.NewMetricWithTimestamp(timestamp,
		metricWithPcieBw(cardLabel, descRxPBW, float64(pcieBwInfo.PcieRxPBw.PcieMaxBw), maxPcieBw))
	ch <- prometheus.NewMetricWithTimestamp(timestamp,
		metricWithPcieBw(cardLabel, descRxNpBW, float64(pcieBwInfo.PcieRxNPBw.PcieMaxBw), maxPcieBw))
	ch <- prometheus.NewMetricWithTimestamp(timestamp,
		metricWithPcieBw(cardLabel, descRxCplBW, float64(pcieBwInfo.PcieRxCPLBw.PcieMaxBw), maxPcieBw))
}
