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
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"ascend-common/api"
	"ascend-common/devmanager/common"
	colcommon "huawei.com/npu-exporter/v6/collector/common"
	"huawei.com/npu-exporter/v6/collector/container"
	"huawei.com/npu-exporter/v6/utils/logger"
)

var (
	cardLabelForVNpuName                      = make([]string, len(colcommon.CardLabel))
	podAiCoreUtilizationRate *prometheus.Desc = nil
	podTotalMemory           *prometheus.Desc = nil
	podUsedMemory            *prometheus.Desc = nil
)

var (
	supportedVnpuDevices = map[string]bool{
		api.Ascend310P: true,
	}
)

const (
	vNpuUUID  = "v_dev_id"
	aiCoreCnt = "aicore_count"
	isVirtual = "is_virtual"
)

func init() {
	cardLabelForVNpuName = append(colcommon.CardLabel, isVirtual)
	cardLabelForVNpuName[2] = vNpuUUID
	cardLabelForVNpuName[3] = aiCoreCnt

	podAiCoreUtilizationRate = colcommon.BuildDescWithLabel("vnpu_pod_aicore_utilization",
		"the vnpu aicore utilization rate, unit is '%'", cardLabelForVNpuName)
	podTotalMemory = colcommon.BuildDescWithLabel("vnpu_pod_total_memory",
		"the vnpu total memory on pod, unit is 'KB'", cardLabelForVNpuName)
	podUsedMemory = colcommon.BuildDescWithLabel("vnpu_pod_used_memory",
		"the vnpu used memory on pod, unit is 'KB'", cardLabelForVNpuName)

}

// VnpuCollector collect vnpu info
type VnpuCollector struct {
	colcommon.MetricsCollectorAdapter
}

// IsSupported check whether the collector is supported
func (c *VnpuCollector) IsSupported(n *colcommon.NpuCollector) bool {
	isSupport := supportedVnpuDevices[n.Dmgr.GetDevType()]
	logForUnSupportDevice(isSupport, n.Dmgr.GetDevType(), colcommon.GetCacheKey(c), "")
	return isSupport
}

// Describe description of the metric
func (c *VnpuCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- podAiCoreUtilizationRate
	ch <- podTotalMemory
	ch <- podUsedMemory
}

// CollectToCache collect the metric to cache
func (c *VnpuCollector) CollectToCache(n *colcommon.NpuCollector, chipList []colcommon.HuaWeiAIChip) {
	for _, chip := range chipList {
		cache := &chipCache{
			chip: chip,
		}
		cache.timestamp = time.Now()
		c.LocalCache.Store(chip.PhyId, *cache)
	}
	colcommon.UpdateCache[chipCache](n, colcommon.GetCacheKey(c), &c.LocalCache)
}

// UpdatePrometheus update prometheus metrics
func (c *VnpuCollector) UpdatePrometheus(ch chan<- prometheus.Metric, n *colcommon.NpuCollector,
	containerMap map[int32]container.DevicesInfo, chips []colcommon.HuaWeiAIChip) {

	updateSingleChip := func(chipWithVnpu colcommon.HuaWeiAIChip, cache chipCache, cardLabel []string) {
		if chipWithVnpu.VDevActivityInfo == nil {
			return
		}
		vDevActivityInfo := chipWithVnpu.VDevActivityInfo
		if !common.IsValidVDevID(vDevActivityInfo.VDevID) {
			return
		}
		containerName := getContainerNameArray(containerMap[int32(vDevActivityInfo.VDevID)])
		if len(containerName) != colcommon.ContainerNameLen {
			return
		}
		cardLabel = getPodDisplayInfo(&chipWithVnpu, containerName)
		doUpdateMetric(ch, cache.timestamp, vDevActivityInfo.VDevAiCoreRate, cardLabel, podAiCoreUtilizationRate)
		doUpdateMetric(ch, cache.timestamp, vDevActivityInfo.VDevTotalMem, cardLabel, podTotalMemory)
		doUpdateMetric(ch, cache.timestamp, vDevActivityInfo.VDevUsedMem, cardLabel, podUsedMemory)
	}

	updateFrame[chipCache](colcommon.GetCacheKey(c), n, containerMap, chips, updateSingleChip)

}

// UpdateTelegraf update telegraf metrics
func (c *VnpuCollector) UpdateTelegraf(fieldsMap map[string]map[string]interface{}, n *colcommon.NpuCollector,
	containerMap map[int32]container.DevicesInfo, chips []colcommon.HuaWeiAIChip) map[string]map[string]interface{} {

	caches := colcommon.GetInfoFromCache[chipCache](n, colcommon.GetCacheKey(c))
	for _, chip := range chips {
		cache, ok := caches[chip.PhyId]
		if !ok {
			continue
		}

		vDevActivityInfo := chip.VDevActivityInfo
		if vDevActivityInfo == nil || !common.IsValidVDevID(vDevActivityInfo.VDevID) {
			continue
		}

		devTagKey := strconv.Itoa(int(cache.chip.LogicID)) + "_" + strconv.Itoa(int(vDevActivityInfo.VDevID))

		if fieldsMap[devTagKey] == nil {
			fieldsMap[devTagKey] = make(map[string]interface{})
		}

		doUpdateTelegraf(fieldsMap[devTagKey], podAiCoreUtilizationRate, vDevActivityInfo.VDevAiCoreRate, "")
		doUpdateTelegraf(fieldsMap[devTagKey], podTotalMemory, vDevActivityInfo.VDevTotalMem, "")
		doUpdateTelegraf(fieldsMap[devTagKey], podUsedMemory, vDevActivityInfo.VDevUsedMem, "")
	}
	return fieldsMap
}

func getPodDisplayInfo(chip *colcommon.HuaWeiAIChip, containerName []string) []string {
	if len(containerName) != colcommon.ContainerNameLen {
		logger.Errorf("container name length %v is not %v", len(containerName), colcommon.ContainerNameLen)
		return nil
	}

	chipInfo := common.DeepCopyChipInfo(chip.ChipInfo)
	vDevActivityInfo := common.DeepCopyVDevActivityInfo(chip.VDevActivityInfo)

	return []string{
		strconv.Itoa(int(chip.DeviceID)),
		common.GetNpuName(chipInfo),
		strconv.Itoa(int(vDevActivityInfo.VDevID)),
		strconv.FormatFloat(vDevActivityInfo.VDevAiCore, 'f', colcommon.DecimalPlaces, colcommon.BitSize),
		containerName[colcommon.NameSpaceIdx],
		containerName[colcommon.PodNameIdx],
		containerName[colcommon.ConNameIdx],
		strconv.FormatBool(vDevActivityInfo.IsVirtualDev),
	}
}
