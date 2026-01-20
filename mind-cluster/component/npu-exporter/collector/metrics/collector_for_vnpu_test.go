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
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/smartystreets/goconvey/convey"

	"ascend-common/api"
	"ascend-common/devmanager/common"
	colcommon "huawei.com/npu-exporter/v6/collector/common"
	"huawei.com/npu-exporter/v6/collector/container"
)

const (
	vnpuMetricNum = 3
	validVnpuID   = 100
	invalidVnpuID = 1
)

// TestVnpuCollectorIsSupported test VnpuCollector IsSupported
func TestVnpuCollectorIsSupported(t *testing.T) {
	n := mockNewNpuCollector()
	cases := []testCase{
		buildTestCase("VnpuCollector: testIsSupported on Ascend310P", &VnpuCollector{}, api.Ascend310P, true),
		buildTestCase("VnpuCollector: testIsSupported on other type", &VnpuCollector{}, "OTHER", false),
	}

	for _, c := range cases {
		patches := gomonkey.NewPatches()
		convey.Convey(c.name, t, func() {
			defer patches.Reset()
			patches.ApplyMethodReturn(n.Dmgr, "GetDevType", c.deviceType)
			isSupported := c.collectorType.IsSupported(n)
			convey.So(isSupported, convey.ShouldEqual, c.expectValue)
		})
	}
}

func TestVnpuCollectorDescribe(t *testing.T) {
	collector := &VnpuCollector{}
	convey.Convey("TestVnpuCollectorDescribe", t, func() {
		ch := make(chan *prometheus.Desc, vnpuMetricNum)
		collector.Describe(ch)
		convey.So(len(ch), convey.ShouldEqual, vnpuMetricNum)
		close(ch)
	})
}

func TestVnpuCollectorCollectToCache(t *testing.T) {
	collector := &VnpuCollector{}
	n := mockNewNpuCollector()
	testChips := []colcommon.HuaWeiAIChip{{PhyId: 0}}

	convey.Convey("TestVnpuCollectorCollectToCache", t, func() {
		collector.CollectToCache(n, testChips)
		cacheInfo := colcommon.GetInfoFromCache[chipCache](n, colcommon.GetCacheKey(collector))
		convey.So(cacheInfo, convey.ShouldNotBeNil)
	})
}

func TestVnpuCollectorUpdatePrometheus(t *testing.T) {
	collector := &VnpuCollector{}
	n := mockNewNpuCollector()
	containerMap := mockContainerInfo()

	testChips := []colcommon.HuaWeiAIChip{{PhyId: 0}}
	collector.CollectToCache(n, testChips)
	chip := createValidVnpuChip()
	testCases := []struct {
		name          string
		preHandleFunc func()
		expectValue   int
	}{
		{name: "TestVnpuCollectorUpdatePrometheus_effective virtual device scenarios",
			preHandleFunc: func() {},
			expectValue:   vnpuMetricNum,
		},
		{name: "TestVnpuCollectorUpdatePrometheus_there is no container info",
			preHandleFunc: func() {
				containerMap = map[int32]container.DevicesInfo{}
			},
			expectValue: 0,
		},
		{name: "TestVnpuCollectorUpdatePrometheus_the vdevid is invalid",
			preHandleFunc: func() {
				chip.VDevActivityInfo.VDevID = invalidVnpuID
			},
			expectValue: 0,
		},
		{name: "TestVnpuCollectorUpdatePrometheus_there is no vdev info",
			preHandleFunc: func() {
				chip.VDevActivityInfo = nil
			},
			expectValue: 0,
		},
	}
	ch := make(chan prometheus.Metric, vnpuMetricNum)
	defer close(ch)
	for _, tt := range testCases {
		convey.Convey(tt.name, t, func() {
			tt.preHandleFunc()
			collector.UpdatePrometheus(ch, n, containerMap, []colcommon.HuaWeiAIChip{chip})
			convey.So(len(ch), convey.ShouldEqual, tt.expectValue)
			//clean ch
			for {
				if len(ch) == 0 {
					break
				}
				<-ch
			}
		})
	}
}

func mockContainerInfo() map[int32]container.DevicesInfo {
	containerMap := map[int32]container.DevicesInfo{
		validVnpuID: {
			Devices: []int{0},
			ID:      strconv.Itoa(validVnpuID),
			Name:    "nsName_podName_ctrName",
		},
	}
	return containerMap
}

func TestVnpuCollectorUpdateTelegraf(t *testing.T) {
	collector := &VnpuCollector{}
	n := mockNewNpuCollector()
	containerMap := mockContainerInfo()
	testChips := []colcommon.HuaWeiAIChip{{PhyId: 0}}
	collector.CollectToCache(n, testChips)
	chip := createValidVnpuChip()
	convey.Convey("TestVnpuCollectorUpdateTelegraf", t, func() {
		convey.Convey("effective virtual device scenarios", func() {
			chipsWithVnpu := []colcommon.HuaWeiAIChip{chip}
			newFieldMaps := collector.UpdateTelegraf(make(map[string]map[string]interface{}), n, containerMap, chipsWithVnpu)
			convey.So(len(newFieldMaps), convey.ShouldEqual, 1)
			convey.So(len(newFieldMaps["0_100"]), convey.ShouldEqual, vnpuMetricNum)
		})
		convey.Convey("there is no container info", func() {
			chip.VDevActivityInfo = nil
			chipsWithVnpu := []colcommon.HuaWeiAIChip{chip}
			containerMap = map[int32]container.DevicesInfo{}
			newFieldMaps := collector.UpdateTelegraf(make(map[string]map[string]interface{}), n, containerMap, chipsWithVnpu)
			convey.So(len(newFieldMaps), convey.ShouldEqual, 0)
		})

	})
}

func TestGetPodDisplayInfo(t *testing.T) {
	const num8 = 8
	convey.Convey("TestGetPodDisplayInfo", t, func() {
		chip := createValidVnpuChip()
		convey.Convey("valid container information", func() {
			containerNames := []string{"namespace", "pod-name", "container-name"}
			labels := getPodDisplayInfo(&chip, containerNames)
			convey.Convey("should return 8 metrics", func() {
				convey.So(len(labels), convey.ShouldEqual, num8)
				convey.So(labels[len(labels)-1], convey.ShouldEqual, "true")
			})
		})

		convey.Convey("invalid container information", func() {
			containerNames := []string{"short"}
			labels := getPodDisplayInfo(&chip, containerNames)
			convey.Convey("should return nil", func() {
				convey.So(labels, convey.ShouldBeNil)
			})
		})
	})
}

func createValidVnpuChip() colcommon.HuaWeiAIChip {
	chip := createChip()
	chip.VDevActivityInfo = &common.VDevActivityInfo{
		VDevID:       validVnpuID,
		VDevAiCore:   1,
		VDevTotalMem: 1,
		VDevUsedMem:  1,
		IsVirtualDev: true,
	}
	return chip
}
