/* Copyright(C) 2021-2023. Huawei Technologies Co.,Ltd. All rights reserved.
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

// Package npu this for parse and pack
package npu

import (
	_ "embed"
	"strings"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"

	"ascend-common/api"
	"huawei.com/npu-exporter/v6/collector/common"
	"huawei.com/npu-exporter/v6/collector/container"
	"huawei.com/npu-exporter/v6/utils/logger"
)

//go:embed sample.conf
var sampleConfig string

const (
	num2 = 2
)

// WatchNPU npu watch struct
type WatchNPU struct {
	collector *common.NpuCollector
}

// SampleConfig used to return sampleConfig
func (*WatchNPU) SampleConfig() string {
	return sampleConfig
}

// Gather used to gather information from dcmi info and hccn tool info
func (npu *WatchNPU) Gather(acc telegraf.Accumulator) error {

	fieldsMap := make(map[string]map[string]interface{})
	const devName = "ascend"

	devTagValue := ""
	if cardType := npu.collector.Dmgr.GetDevType(); cardType == api.Ascend910A3 || cardType == api.Ascend910B ||
		cardType == api.Ascend910A {
		devTagValue = strings.ToLower(api.Ascend910)
	} else {
		devTagValue = strings.ToLower(cardType)
	}
	logger.DynamicConfigure(logger.Config{Acc: acc})

	containerMap := common.GetContainerNPUInfo(npu.collector)
	chips := common.GetChipListWithVNPU(npu.collector)

	fieldsMap = npu.gatherChain(fieldsMap, common.ChainForSingleGoroutine, containerMap, chips)
	fieldsMap = npu.gatherChain(fieldsMap, common.ChainForMultiGoroutine, containerMap, chips)
	fieldsMap = npu.gatherChain(fieldsMap, common.ChainForCustomPlugin, containerMap, chips)

	generalFields := fieldsMap[common.GeneralDevTagKey]
	acc.AddFields(devName, generalFields, map[string]string{"device": devTagValue})

	// after the report is completed, deleted to avoid repeated reporting in the for loop
	delete(fieldsMap, common.GeneralDevTagKey)
	for key, fields := range fieldsMap {

		ids := strings.Split(key, "_")
		devTag := map[string]string{"device": devTagValue + "-" + ids[0]}
		if len(ids) >= num2 {
			devTag["vdev_id"] = ids[1]
		}

		acc.AddFields(devName, fields, devTag)
	}

	return nil
}

func (npu *WatchNPU) gatherChain(fieldsMap map[string]map[string]interface{}, chain []common.MetricsCollector,
	containerMap map[int32]container.DevicesInfo, chips []common.HuaWeiAIChip) map[string]map[string]interface{} {

	for _, collector := range chain {
		fieldsMap = collector.UpdateTelegraf(fieldsMap, npu.collector, containerMap, chips)
	}
	return fieldsMap
}

func init() {
	inputs.Add("npu", func() telegraf.Input {
		return &WatchNPU{
			collector: common.Collector,
		}
	})
}
