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
	"github.com/prometheus/client_golang/prometheus"

	"huawei.com/npu-exporter/v6/collector/common"
	"huawei.com/npu-exporter/v6/collector/container"
	"huawei.com/npu-exporter/v6/versions"
)

var (
	versionInfoDesc = common.BuildDescWithLabel("npu_exporter_version_info", "exporter version with value '1'",
		[]string{"exporterVersion"})
)

// VersionCollector collect sio info
type VersionCollector struct {
	common.MetricsCollectorAdapter
}

// Describe description of the metric
func (c *VersionCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- versionInfoDesc
}

// UpdatePrometheus update prometheus metric
func (c *VersionCollector) UpdatePrometheus(ch chan<- prometheus.Metric, n *common.NpuCollector,
	containerMap map[int32]container.DevicesInfo, chips []common.HuaWeiAIChip) {
	ch <- prometheus.MustNewConstMetric(versionInfoDesc, prometheus.GaugeValue, 1, []string{versions.BuildVersion}...)
}

// UpdateTelegraf update telegraf metric
func (c *VersionCollector) UpdateTelegraf(fieldsMap map[string]map[string]interface{}, n *common.NpuCollector,
	containerMap map[int32]container.DevicesInfo, chips []common.HuaWeiAIChip) map[string]map[string]interface{} {

	if fieldsMap[common.GeneralDevTagKey] == nil {
		fieldsMap[common.GeneralDevTagKey] = make(map[string]interface{})
	}
	doUpdateTelegraf(fieldsMap[common.GeneralDevTagKey], versionInfoDesc, versions.BuildVersion, "")
	return fieldsMap
}
