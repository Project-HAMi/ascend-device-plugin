/* Copyright(C) 2025. Huawei Technologies Co.,Ltd. All rights reserved.
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
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/influxdata/telegraf"
	"github.com/smartystreets/goconvey/convey"

	"ascend-common/api"
	"ascend-common/common-utils/hwlog"
	"ascend-common/devmanager"
	"huawei.com/npu-exporter/v6/collector/common"
	"huawei.com/npu-exporter/v6/collector/container"
	"huawei.com/npu-exporter/v6/collector/metrics"
	"huawei.com/npu-exporter/v6/utils/logger"
)

const (
	num5 = 5
)

func init() {
	logger.HwLogConfig = &hwlog.LogConfig{
		OnlyToStdout: true,
	}
	logger.InitLogger("Prometheus")
	initChain()
}

func initChain() {
	common.ChainForSingleGoroutine = []common.MetricsCollector{
		&metrics.VersionCollector{},
	}
}

func mockNewNpuCollector() *common.NpuCollector {
	tc := newNpuCollectorTestCase{
		cacheTime:    time.Duration(num5),
		updateTime:   time.Duration(num5),
		deviceParser: &container.DevicesParser{},
		dmgr:         &devmanager.DeviceManager{},
	}
	c := common.NewNpuCollector(tc.cacheTime, tc.updateTime, tc.deviceParser, tc.dmgr)
	return c
}

// TestGather verifies different device type scenarios
func TestGather(t *testing.T) {
	tests := []struct {
		name        string
		deviceType  string
		expectedTag string
	}{
		{name: api.Ascend910A3,
			deviceType:  api.Ascend910A3,
			expectedTag: api.Ascend910,
		},
		{name: api.Ascend310P,
			deviceType:  api.Ascend310P,
			expectedTag: api.Ascend310P,
		},
	}
	npu := &WatchNPU{
		collector: mockNewNpuCollector(),
	}
	acc := &MockAccumulator{}

	for _, tt := range tests {
		convey.Convey(tt.name, t, func() {
			patches := gomonkey.NewPatches()
			defer patches.Reset()

			patches.ApplyMethodReturn(npu.collector.Dmgr, "GetDevType", tt.deviceType)
			patches.ApplyFuncReturn(common.GetContainerNPUInfo, nil)
			patches.ApplyFuncReturn(common.GetChipListWithVNPU, nil)
			patches.ApplyMethodReturn(common.ChainForSingleGoroutine[0], "UpdateTelegraf",
				map[string]map[string]interface{}{
					common.GeneralDevTagKey: {"npu_exporter_version_info": "7.0.0"},
					"0":                     {"npu_chip_info_power": "1"},
					"1_100":                 {"npu_chip_info_voltage": "1"},
				})

			err := npu.Gather(acc)
			convey.So(err, convey.ShouldBeNil)
			convey.So(acc.fields["ascend,device="+strings.ToLower(tt.expectedTag)], convey.ShouldNotBeEmpty)
		})
	}
}

// TestGatherChain tests the gatherChain method of WatchNPU
func TestGatherChain(t *testing.T) {
	npu := &WatchNPU{}
	fieldsMap := make(map[string]map[string]interface{})
	chain := []common.MetricsCollector{&metrics.VersionCollector{}}

	convey.Convey("TestGatherChain", t, func() {
		result := npu.gatherChain(fieldsMap, chain, nil, nil)
		logger.Infof("result:%v", result)
		convey.So(len(result), convey.ShouldEqual, 1)
	})
}

type newNpuCollectorTestCase struct {
	cacheTime    time.Duration
	updateTime   time.Duration
	deviceParser *container.DevicesParser
	dmgr         *devmanager.DeviceManager
}

// MockAccumulator is a mock implementation of telegraf.Accumulator
type MockAccumulator struct {
	fields map[string]map[string]interface{}
}

func (m *MockAccumulator) AddFields(measurement string, fields map[string]interface{}, tags map[string]string,
	t ...time.Time) {
	if m.fields == nil {
		m.fields = make(map[string]map[string]interface{})
	}
	pairs := make([]string, 0, len(tags))
	for k, v := range tags {
		pairs = append(pairs, fmt.Sprintf("%s=%v", k, v))
	}
	metricKey := measurement + "," + strings.Join(pairs, ",")
	m.fields[metricKey] = fields
}

func (m *MockAccumulator) AddGauge(measurement string, fields map[string]interface{}, tags map[string]string,
	t ...time.Time) {
}

func (m *MockAccumulator) AddCounter(measurement string, fields map[string]interface{}, tags map[string]string,
	t ...time.Time) {
}

func (m *MockAccumulator) AddSummary(measurement string, fields map[string]interface{}, tags map[string]string,
	t ...time.Time) {
}

func (m *MockAccumulator) AddHistogram(measurement string, fields map[string]interface{}, tags map[string]string,
	t ...time.Time) {
}

func (m *MockAccumulator) AddMetric(metric telegraf.Metric) {
}

func (m *MockAccumulator) SetPrecision(precision time.Duration) {
}

func (m *MockAccumulator) AddError(err error) {
}

func (m *MockAccumulator) WithTracking(maxTracked int) telegraf.TrackingAccumulator {
	return nil
}
