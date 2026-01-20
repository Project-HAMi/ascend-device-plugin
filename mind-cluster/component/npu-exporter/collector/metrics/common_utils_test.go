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

// Package metrics offer common utils for collector
package metrics

import (
	"math"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/smartystreets/goconvey/convey"

	"ascend-common/devmanager/common"
	colcommon "huawei.com/npu-exporter/v6/collector/common"
)

const (
	invalidNum = -1
	num100     = 100
)

// TestValidateNum test numerical verification
func TestValidateNum(t *testing.T) {
	convey.Convey("TestValidateNum", t, func() {
		convey.Convey("return true when the num is valid", func() {
			convey.So(validateNum(0), convey.ShouldBeTrue)
			convey.So(validateNum(num100), convey.ShouldBeTrue)
		})

		convey.Convey("return false when the num is invalid", func() {
			convey.So(validateNum(invalidNum), convey.ShouldBeFalse)
			convey.So(validateNum(math.MaxUint32), convey.ShouldBeFalse)
		})
	})
}

// TestDoUpdateTelegraf test update telegraf
func TestDoUpdateTelegraf(t *testing.T) {
	convey.Convey("TestDoUpdateTelegraf", t, func() {
		fieldMap := make(map[string]interface{})
		desc := prometheus.NewDesc("test_metric", "", nil, nil)

		convey.Convey("update when num is valid", func() {
			doUpdateTelegrafWithValidateNum(fieldMap, desc, num100, "_suffix")
			convey.So(fieldMap["test_metric_suffix"], convey.ShouldEqual, num100)
		})

		convey.Convey("don't update when num is invalid", func() {
			doUpdateTelegrafWithValidateNum(fieldMap, desc, -1, "_suffix")
			convey.So(fieldMap, convey.ShouldBeEmpty)
		})
	})
}

// TestDoUpdateMetric test update prometheus
func TestDoUpdateMetric(t *testing.T) {
	const (
		num10    = 10
		num100   = 100
		negaNum  = -5
		floatNum = 3.14
	)
	convey.Convey("TestDoUpdateMetric", t, func() {
		ch := make(chan prometheus.Metric, 1)
		desc := prometheus.NewDesc("test_metric", "", []string{"label"}, nil)

		convey.Convey("convert the various numeric types correctly", func() {
			testCases := []struct {
				input    interface{}
				expected float64
			}{
				{int(num10), num10},
				{int32(negaNum), negaNum},
				{uint64(num100), num100},
				{float32(floatNum), floatNum},
			}

			for _, tc := range testCases {
				doUpdateMetric(ch, time.Now(), tc.input, []string{"label"}, desc)
				m := <-ch
				convey.So(m, convey.ShouldNotBeEmpty)
			}
		})
	})
}

// TestContainerInfo test container information processing
func TestContainerInfo(t *testing.T) {
	convey.Convey("TestContainerInfo", t, func() {
		convey.Convey("correctly split the array of container names", func() {
			testCases := []struct {
				input    []string
				expected []string
			}{
				{[]string{"ns", "pod", "container"}, []string{"container", "ns", "pod"}},
				{[]string{"short"}, []string{"", "", ""}},
			}

			for _, tc := range testCases {
				c, ns, pod := getContainerInfoWithDefault(tc.input)
				convey.So([]string{c, ns, pod}, convey.ShouldResemble, tc.expected)
			}
		})
	})
}

// TestCardLabel test card label generation
func TestCardLabel(t *testing.T) {
	convey.Convey("TestCardLabel", t, func() {
		chip := &colcommon.HuaWeiAIChip{
			DeviceID:    0,
			ChipInfo:    &common.ChipInfo{Name: "1", Type: "1", Version: "1"},
			VDieID:      "die1",
			PCIeBusInfo: "0000:00:01.0",
		}

		expected := []string{
			"0",
			"1-1-1",
			"die1",
			"0000:00:01.0",
			"test-ns",
			"test-pod",
			"test-container",
		}

		convey.Convey("correctly generate an array of tags", func() {
			labels := collectCardLabelValue(chip, "test-ns", "test-pod", "test-container")
			convey.So(labels, convey.ShouldResemble, expected)
		})
	})
}

// TestNilValidation test null pointer validation
func TestNilValidation(t *testing.T) {
	convey.Convey("TestNilValidation", t, func() {
		var nilPtr *int
		val := 10

		convey.Convey("all non null pointers should return true", func() {
			convey.So(validateNotNilForEveryElement(&val), convey.ShouldBeTrue)
		})

		convey.Convey("a null pointer should return false", func() {
			convey.So(validateNotNilForEveryElement(nilPtr), convey.ShouldBeFalse)
		})

		convey.Convey("non pointer types should return false", func() {
			convey.So(validateNotNilForEveryElement(val), convey.ShouldBeFalse)
		})
	})
}
