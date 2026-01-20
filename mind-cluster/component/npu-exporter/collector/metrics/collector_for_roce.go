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
	macRxMacPauseNum       = "mac_rx_mac_pause_num"
	macTxMacPauseNum       = "mac_tx_mac_pause_num"
	macRxPfcPktNum         = "mac_rx_pfc_pkt_num"
	macTxPfcPktNum         = "mac_tx_pfc_pkt_num"
	macRxBadPktNum         = "mac_rx_bad_pkt_num"
	macTxBadPktNum         = "mac_tx_bad_pkt_num"
	roCERxAllPktNum        = "roce_rx_all_pkt_num"
	roCETxAllPktNum        = "roce_tx_all_pkt_num"
	roCERxErrPktNum        = "roce_rx_err_pkt_num"
	roCETxErrPktNum        = "roce_tx_err_pkt_num"
	roCERxCnpPktNum        = "roce_rx_cnp_pkt_num"
	roCETxCnpPktNum        = "roce_tx_cnp_pkt_num"
	macRxBadOctNum         = "mac_rx_bad_oct_num"
	macTxBadOctNum         = "mac_tx_bad_oct_num"
	roCEUnexpectedAckNum   = "roce_unexpected_ack_num"
	roCEOutOfOrderNum      = "roce_out_of_order_num"
	roCEVerificationErrNum = "roce_verification_err_num"
	roCEQpStatusErrNum     = "roce_qp_status_err_num"
	roCENewPktRtyNum       = "roce_new_pkt_rty_num"
	roCEEcnDBNum           = "roce_ecn_db_num"
	macRXFcsErrPktNum      = "mac_rx_fcs_err_pkt_num"
)

var (
	// mac
	descMacRxPauseNum  = colcommon.BuildDesc("npu_chip_mac_rx_pause_num", "npu interface receive mac-rx-pause-num")
	descMacTxPauseNum  = colcommon.BuildDesc("npu_chip_mac_tx_pause_num", "npu interface receive mac-tx-pause-num")
	descMacRxPfcPktNum = colcommon.BuildDesc("npu_chip_mac_rx_pfc_pkt_num", "npu interface receive mac-rx-pfc-pkt-num")
	descMacTxPfcPktNum = colcommon.BuildDesc("npu_chip_mac_tx_pfc_pkt_num", "npu interface receive mac-tx-pfc-pkt-num")
	descMacRxBadPktNum = colcommon.BuildDesc("npu_chip_mac_rx_bad_pkt_num", "npu interface receive mac-rx-bad-pkt-num")
	descMacTxBadPktNum = colcommon.BuildDesc("npu_chip_mac_tx_bad_pkt_num", "npu interface receive mac-tx-bad-pkt-num")
	descMacTxBadOctNum = colcommon.BuildDesc("npu_chip_mac_tx_bad_oct_num", "npu interface receive mac-tx-bad-oct-num")
	descMacRxBadOctNum = colcommon.BuildDesc("npu_chip_mac_rx_bad_oct_num", "npu interface receive mac-rx-bad-oct-num")

	descRxFCSNum = colcommon.BuildDesc("npu_chip_info_rx_fcs_num", "the npu network fcs receive number")
	descRxECNNum = colcommon.BuildDesc("npu_chip_info_rx_ecn_num", "the npu network ecn receive number")

	// roce
	descRoceRxAllPktNum = colcommon.BuildDesc("npu_chip_roce_rx_all_pkt_num", "npu interface receive roce-rx-all-pkt-num")
	descRoceTxAllPktNum = colcommon.BuildDesc("npu_chip_roce_tx_all_pkt_num", "npu interface receive roce-tx-all-pkt-num")
	descRoceRxErrPktNum = colcommon.BuildDesc("npu_chip_roce_rx_err_pkt_num", "npu interface receive roce-rx-err-pkt-num")
	descRoceTxErrPktNum = colcommon.BuildDesc("npu_chip_roce_tx_err_pkt_num", "npu interface receive roce-tx-err-pkt-num")
	descRoceRxCnpPktNum = colcommon.BuildDesc("npu_chip_roce_rx_cnp_pkt_num", "npu interface receive roce-rx-cnp-pkt-num")
	descRoceTxCnpPktNum = colcommon.BuildDesc("npu_chip_roce_tx_cnp_pkt_num", "npu interface receive roce-tx-cnp-pkt-num")

	descRoceNewPktRtyNum = colcommon.BuildDesc("npu_chip_roce_new_pkt_rty_num",
		"npu interface receive roce-new-pkt-rty-num")
	descRoceOutOfOrderNum = colcommon.BuildDesc("npu_chip_roce_out_of_order_num",
		"the npu interface receive roce-out-of-order-num")
	descRoceQpStatusErrNum = colcommon.BuildDesc("npu_chip_roce_qp_status_err_num",
		"the npu interface receive roce-qp-status-err-num")
	descRoceUnexpectedAcktNum = colcommon.BuildDesc("npu_chip_roce_unexpected_ack_num",
		"the npu interface receive roce-unexpected-ack-num")
	descRoceVerificationErrNum = colcommon.BuildDesc("npu_chip_roce_verification_err_num",
		"the npu interface receive roce-verification-err-num")
)

type roceCache struct {
	chip      colcommon.HuaWeiAIChip
	timestamp time.Time
	// extInfo the statistics about packets
	extInfo *common.StatInfo
}

// RoceCollector collect roce info
type RoceCollector struct {
	colcommon.MetricsCollectorAdapter
}

// IsSupported check whether the collector is supported
func (c *RoceCollector) IsSupported(n *colcommon.NpuCollector) bool {
	isSupport := n.Dmgr.IsTrainingCard()
	logForUnSupportDevice(isSupport, n.Dmgr.GetDevType(), colcommon.GetCacheKey(c),
		"only training card supports network related info")
	return isSupport
}

// Describe description of the metric
func (c *RoceCollector) Describe(ch chan<- *prometheus.Desc) {

	// mac
	ch <- descMacRxPauseNum
	ch <- descMacTxPauseNum
	ch <- descMacRxPfcPktNum
	ch <- descMacTxPfcPktNum
	ch <- descMacRxBadPktNum
	ch <- descMacTxBadPktNum
	ch <- descMacTxBadOctNum
	ch <- descMacRxBadOctNum
	ch <- descRxFCSNum

	// roce
	ch <- descRoceRxAllPktNum
	ch <- descRoceTxAllPktNum
	ch <- descRoceRxErrPktNum
	ch <- descRoceTxErrPktNum
	ch <- descRoceRxCnpPktNum
	ch <- descRoceTxCnpPktNum
	ch <- descRoceNewPktRtyNum
	ch <- descRoceUnexpectedAcktNum
	ch <- descRoceOutOfOrderNum
	ch <- descRoceVerificationErrNum
	ch <- descRoceQpStatusErrNum
	ch <- descRxECNNum

}

// CollectToCache collect the metric to cache
func (c *RoceCollector) CollectToCache(n *colcommon.NpuCollector, chipList []colcommon.HuaWeiAIChip) {
	for _, chip := range chipList {
		statInfo, err := hccn.GetNPUStatInfo(chip.DeviceID)
		if err != nil {
			logErrMetricsWithLimit(colcommon.DomainForRoce, chip.LogicID, err)
			return
		}
		hwlog.ResetErrCnt(colcommon.DomainForRoce, chip.LogicID)
		c.LocalCache.Store(chip.PhyId, roceCache{chip: chip, timestamp: time.Now(), extInfo: getMainStatInfo(statInfo)})
	}
	colcommon.UpdateCache[roceCache](n, colcommon.GetCacheKey(c), &c.LocalCache)

}

// UpdatePrometheus update prometheus metrics
func (c *RoceCollector) UpdatePrometheus(ch chan<- prometheus.Metric, n *colcommon.NpuCollector,
	containerMap map[int32]container.DevicesInfo, chips []colcommon.HuaWeiAIChip) {

	updateSingleChip := func(chipWithVnpu colcommon.HuaWeiAIChip, cache roceCache, cardLabel []string) {
		statInfo := cache.extInfo
		if statInfo == nil {
			return
		}
		updateStatInfoOfMac(ch, cache.timestamp, statInfo, cardLabel)
		updateStatInfoOfRoCE(ch, cache.timestamp, statInfo, cardLabel)
	}
	updateFrame[roceCache](colcommon.GetCacheKey(c), n, containerMap, chips, updateSingleChip)

}

// UpdateTelegraf update telegraf metrics
func (c *RoceCollector) UpdateTelegraf(fieldsMap map[string]map[string]interface{}, n *colcommon.NpuCollector,
	containerMap map[int32]container.DevicesInfo, chips []colcommon.HuaWeiAIChip) map[string]map[string]interface{} {

	caches := colcommon.GetInfoFromCache[roceCache](n, colcommon.GetCacheKey(c))
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
		doUpdateTelegraf(fieldMap, descMacRxPauseNum, extInfo.MacRxPauseNum, "")
		doUpdateTelegraf(fieldMap, descMacTxPauseNum, extInfo.MacTxPauseNum, "")
		doUpdateTelegraf(fieldMap, descMacRxPfcPktNum, extInfo.MacRxPfcPktNum, "")
		doUpdateTelegraf(fieldMap, descMacTxPfcPktNum, extInfo.MacTxPfcPktNum, "")
		doUpdateTelegraf(fieldMap, descMacRxBadPktNum, extInfo.MacRxBadPktNum, "")
		doUpdateTelegraf(fieldMap, descMacTxBadPktNum, extInfo.MacTxBadPktNum, "")
		doUpdateTelegraf(fieldMap, descMacTxBadOctNum, extInfo.MacTxBadOctNum, "")
		doUpdateTelegraf(fieldMap, descMacRxBadOctNum, extInfo.MacRxBadOctNum, "")
		doUpdateTelegraf(fieldMap, descRxFCSNum, extInfo.MacRXFcsErrPktNum, "")

		doUpdateTelegraf(fieldMap, descRoceRxAllPktNum, extInfo.RoceRxAllPktNum, "")
		doUpdateTelegraf(fieldMap, descRoceTxAllPktNum, extInfo.RoceTxAllPktNum, "")
		doUpdateTelegraf(fieldMap, descRoceRxErrPktNum, extInfo.RoceRxErrPktNum, "")
		doUpdateTelegraf(fieldMap, descRoceTxErrPktNum, extInfo.RoceTxErrPktNum, "")
		doUpdateTelegraf(fieldMap, descRoceRxCnpPktNum, extInfo.RoceRxCnpPktNum, "")
		doUpdateTelegraf(fieldMap, descRoceTxCnpPktNum, extInfo.RoceTxCnpPktNum, "")
		doUpdateTelegraf(fieldMap, descRoceNewPktRtyNum, extInfo.RoceNewPktRtyNum, "")
		doUpdateTelegraf(fieldMap, descRoceUnexpectedAcktNum, extInfo.RoceUnexpectedAckNum, "")
		doUpdateTelegraf(fieldMap, descRoceOutOfOrderNum, extInfo.RoceOutOfOrderNum, "")
		doUpdateTelegraf(fieldMap, descRoceVerificationErrNum, extInfo.RoceVerificationErrNum, "")
		doUpdateTelegraf(fieldMap, descRoceQpStatusErrNum, extInfo.RoceQpStatusErrNum, "")
		doUpdateTelegraf(fieldMap, descRxECNNum, extInfo.RoceEcnDBNum, "")
	}
	return fieldsMap
}
func getMainStatInfo(statInfo map[string]int) *common.StatInfo {
	mainStatInfo := common.StatInfo{}
	mainStatInfo.MacRxPauseNum = float64(statInfo[macRxMacPauseNum])
	mainStatInfo.MacTxPauseNum = float64(statInfo[macTxMacPauseNum])
	mainStatInfo.MacRxPfcPktNum = float64(statInfo[macRxPfcPktNum])
	mainStatInfo.MacTxPfcPktNum = float64(statInfo[macTxPfcPktNum])
	mainStatInfo.MacRxBadPktNum = float64(statInfo[macRxBadPktNum])
	mainStatInfo.MacTxBadPktNum = float64(statInfo[macTxBadPktNum])
	mainStatInfo.RoceRxAllPktNum = float64(statInfo[roCERxAllPktNum])
	mainStatInfo.RoceTxAllPktNum = float64(statInfo[roCETxAllPktNum])
	mainStatInfo.RoceRxErrPktNum = float64(statInfo[roCERxErrPktNum])
	mainStatInfo.RoceTxErrPktNum = float64(statInfo[roCETxErrPktNum])
	mainStatInfo.RoceRxCnpPktNum = float64(statInfo[roCERxCnpPktNum])
	mainStatInfo.RoceTxCnpPktNum = float64(statInfo[roCETxCnpPktNum])
	mainStatInfo.MacRxBadOctNum = float64(statInfo[macRxBadOctNum])
	mainStatInfo.MacTxBadOctNum = float64(statInfo[macTxBadOctNum])
	mainStatInfo.RoceUnexpectedAckNum = float64(statInfo[roCEUnexpectedAckNum])
	mainStatInfo.RoceOutOfOrderNum = float64(statInfo[roCEOutOfOrderNum])
	mainStatInfo.RoceVerificationErrNum = float64(statInfo[roCEVerificationErrNum])
	mainStatInfo.RoceQpStatusErrNum = float64(statInfo[roCEQpStatusErrNum])
	mainStatInfo.RoceNewPktRtyNum = float64(statInfo[roCENewPktRtyNum])
	mainStatInfo.RoceEcnDBNum = float64(statInfo[roCEEcnDBNum])
	mainStatInfo.MacRXFcsErrPktNum = float64(statInfo[macRXFcsErrPktNum])

	return &mainStatInfo
}

func updateStatInfoOfMac(ch chan<- prometheus.Metric, ts time.Time, statInfo *common.StatInfo, cardLabel []string) {
	doUpdateMetric(ch, ts, statInfo.MacRxPauseNum, cardLabel, descMacRxPauseNum)
	doUpdateMetric(ch, ts, statInfo.MacTxPauseNum, cardLabel, descMacTxPauseNum)
	doUpdateMetric(ch, ts, statInfo.MacRxPfcPktNum, cardLabel, descMacRxPfcPktNum)
	doUpdateMetric(ch, ts, statInfo.MacTxPfcPktNum, cardLabel, descMacTxPfcPktNum)
	doUpdateMetric(ch, ts, statInfo.MacRxBadPktNum, cardLabel, descMacRxBadPktNum)
	doUpdateMetric(ch, ts, statInfo.MacTxBadPktNum, cardLabel, descMacTxBadPktNum)
	doUpdateMetric(ch, ts, statInfo.MacTxBadOctNum, cardLabel, descMacTxBadOctNum)
	doUpdateMetric(ch, ts, statInfo.MacRxBadOctNum, cardLabel, descMacRxBadOctNum)
	doUpdateMetric(ch, ts, statInfo.MacRXFcsErrPktNum, cardLabel, descRxFCSNum)
}

func updateStatInfoOfRoCE(ch chan<- prometheus.Metric, ts time.Time, statInfo *common.StatInfo, cardLabel []string) {
	doUpdateMetric(ch, ts, statInfo.RoceRxAllPktNum, cardLabel, descRoceRxAllPktNum)
	doUpdateMetric(ch, ts, statInfo.RoceTxAllPktNum, cardLabel, descRoceTxAllPktNum)
	doUpdateMetric(ch, ts, statInfo.RoceRxErrPktNum, cardLabel, descRoceRxErrPktNum)
	doUpdateMetric(ch, ts, statInfo.RoceTxErrPktNum, cardLabel, descRoceTxErrPktNum)
	doUpdateMetric(ch, ts, statInfo.RoceRxCnpPktNum, cardLabel, descRoceRxCnpPktNum)
	doUpdateMetric(ch, ts, statInfo.RoceTxCnpPktNum, cardLabel, descRoceTxCnpPktNum)
	doUpdateMetric(ch, ts, statInfo.RoceNewPktRtyNum, cardLabel, descRoceNewPktRtyNum)
	doUpdateMetric(ch, ts, statInfo.RoceUnexpectedAckNum, cardLabel, descRoceUnexpectedAcktNum)
	doUpdateMetric(ch, ts, statInfo.RoceOutOfOrderNum, cardLabel, descRoceOutOfOrderNum)
	doUpdateMetric(ch, ts, statInfo.RoceVerificationErrNum, cardLabel, descRoceVerificationErrNum)
	doUpdateMetric(ch, ts, statInfo.RoceQpStatusErrNum, cardLabel, descRoceQpStatusErrNum)
	doUpdateMetric(ch, ts, statInfo.RoceEcnDBNum, cardLabel, descRxECNNum)
	doUpdateMetric(ch, ts, statInfo.RoceEcnDBNum, cardLabel, descRxECNNum)
}
