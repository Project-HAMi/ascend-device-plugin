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
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/smartystreets/goconvey/convey"

	"ascend-common/devmanager/common"
	colcommon "huawei.com/npu-exporter/v6/collector/common"
)

type TestCase struct {
	name            string
	initFunc        func()
	expectMetricLen int
}

const (
	expectMetricLen4 = 4
	expectMetricLen6 = 6
	vdevId           = 132
	maxMetrics       = 10
	mockNs           = "mockNs"
	mockPodName      = "mockPodName"
)

func TestUpdateHbmInfo(t *testing.T) {
	collector := HbmCollector{}
	ch := make(chan int, maxMetrics)
	defer close(ch)
	cache := buildHbmCache()
	chipWithVnpu := &colcommon.HuaWeiAIChip{}
	cases := buildTestCases(&collector, chipWithVnpu, &cache)
	patch := gomonkey.NewPatches()
	patch.ApplyFunc(doUpdateMetric, func(_ chan<- prometheus.Metric, _ time.Time, _ interface{}, _ []string,
		desc *prometheus.Desc) {
		ch <- 0
	})
	patch.ApplyFuncReturn(geenContainerInfo, nil)
	patch.ApplyFuncReturn(getContainerNameArray, []string{mockNs, mockPodName, mockContainerName})
	defer patch.Reset()

	for _, c := range cases {
		convey.Convey(c.name, t, func() {
			ch = make(chan int, maxMetrics)
			c.initFunc()
			collector.updateHbmInfo(nil, cache, nil, nil, *chipWithVnpu)
			convey.So(len(ch), convey.ShouldEqual, c.expectMetricLen)
		})
	}
}

func buildTestCases(collector *HbmCollector, chipWithVnpu *colcommon.HuaWeiAIChip, cache *hbmCache) []TestCase {
	cases := []TestCase{
		{name: "when npu is not 910 series ", initFunc: func() {}, expectMetricLen: expectMetricLen4},
		{name: "when vnpu is nil and with container info", initFunc: func() {
			collector.Is910Series = true
		}, expectMetricLen: expectMetricLen6},
		{name: "when chip is vnpu", initFunc: func() {
			chipWithVnpu.VDevActivityInfo = &common.VDevActivityInfo{
				VDevID: vdevId,
			}
		}, expectMetricLen: expectMetricLen4},
		{name: "when extInfo.HbmInfo is nil", initFunc: func() { cache.extInfo.HbmInfo = nil }, expectMetricLen: 0},
		{name: "when extInfo is nil", initFunc: func() { cache.extInfo = nil }, expectMetricLen: 0},
	}
	return cases
}

func buildHbmCache() hbmCache {
	cache := hbmCache{
		chip: colcommon.HuaWeiAIChip{
			PhyId: 0,
		},
		hbmUtilization: 0,
		timestamp:      time.Now(),
		extInfo: &common.HbmAggregateInfo{
			HbmInfo: &common.HbmInfo{
				BandWidthUtilRate: 0,
				Frequency:         0,
				MemorySize:        0,
				Temp:              0,
				Usage:             0,
			},
			ECCInfo: &common.ECCInfo{
				EnableFlag:                0,
				SingleBitErrorCnt:         0,
				DoubleBitErrorCnt:         0,
				TotalSingleBitErrorCnt:    0,
				TotalDoubleBitErrorCnt:    0,
				SingleBitIsolatedPagesCnt: 0,
				DoubleBitIsolatedPagesCnt: 0,
			},
		},
	}
	return cache
}
