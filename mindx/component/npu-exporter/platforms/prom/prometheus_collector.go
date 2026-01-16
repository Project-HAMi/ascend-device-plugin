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

// Package prometheus for prometheus collector
package prom

import (
	"github.com/prometheus/client_golang/prometheus"

	"huawei.com/npu-exporter/v6/collector/common"
	"huawei.com/npu-exporter/v6/collector/container"
	"huawei.com/npu-exporter/v6/utils"
	"huawei.com/npu-exporter/v6/utils/logger"
)

// CollectorForPrometheus Entry point for collecting and converting
type CollectorForPrometheus struct {
	collector *common.NpuCollector
}

// NewPrometheusCollector create an instance of prometheus Collector
func NewPrometheusCollector(collector *common.NpuCollector) *CollectorForPrometheus {
	promCollector := &CollectorForPrometheus{
		collector: collector,
	}
	return promCollector
}

// Describe desc metrics of prometheus
func (*CollectorForPrometheus) Describe(ch chan<- *prometheus.Desc) {
	if ch == nil {
		logger.Error("ch is nil ")
		return
	}
	const cacheSize = 100
	tempCh := make(chan *prometheus.Desc, cacheSize)
	done := make(chan bool)

	go func() {
		seenMetrics := make(map[string]struct{})
		for desc := range tempCh {
			if desc == nil {
				continue
			}
			descKey := utils.GetDescName(desc)
			if _, exists := seenMetrics[descKey]; exists {
				logger.Warnf("duplicate metric description detected, keeping first declaration, ignoring duplicate: %s", desc)
				continue
			}
			seenMetrics[descKey] = struct{}{}
			ch <- desc
		}
		// tempCh closed
		done <- true
	}()

	describeChain(tempCh, common.ChainForSingleGoroutine)
	describeChain(tempCh, common.ChainForMultiGoroutine)
	describeChain(tempCh, common.ChainForCustomPlugin)

	close(tempCh)

	<-done
}

func describeChain(ch chan<- *prometheus.Desc, chain []common.MetricsCollector) {
	for _, collector := range chain {
		if collector != nil {
			collector.Describe(ch)
		}
	}
}

// Collect update metrics of prometheus
func (n *CollectorForPrometheus) Collect(ch chan<- prometheus.Metric) {
	containerMap := common.GetContainerNPUInfo(n.collector)
	chips := common.GetChipListWithVNPU(n.collector)
	collectChain(ch, n, containerMap, chips, common.ChainForSingleGoroutine)
	collectChain(ch, n, containerMap, chips, common.ChainForMultiGoroutine)
	collectChain(ch, n, containerMap, chips, common.ChainForCustomPlugin)
}

func collectChain(ch chan<- prometheus.Metric, n *CollectorForPrometheus, containerMap map[int32]container.DevicesInfo,
	chips []common.HuaWeiAIChip, chain []common.MetricsCollector) {
	if ch == nil {
		logger.Error("ch is nil")
		return
	}
	for _, collector := range chain {
		collector.UpdatePrometheus(ch, n.collector, containerMap, chips)
	}
}
