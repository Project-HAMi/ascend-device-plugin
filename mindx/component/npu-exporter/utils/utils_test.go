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

// Package utils for common utils
package utils

import (
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/smartystreets/goconvey/convey"
)

const (
	emptyString     = ""
	testMetricName  = "test_metric"
	testMetricName2 = "another_metric"
	invalidDescStr  = "invalid description"
	noCommaDescStr  = "fqName: test_metric"
	normalDescStr   = `fqName: "test_metric", help: "test help"`
	normalDescStr2  = `fqName: another_metric, help: "another help"`
	noQuoteDescStr  = `fqName: test_metric, help: "test help"`
	testHelp        = "test help"
)

func TestGetDescName(t *testing.T) {
	convey.Convey("should return empty string when desc is nil", t, testGetDescNameNil)
	convey.Convey("should return empty string when desc.String does not contain fqName prefix", t,
		testGetDescNameNoFqName)
	convey.Convey("should return empty string when desc.String does not contain comma", t,
		testGetDescNameNoComma)
	convey.Convey("should return metric name when desc.String contains valid format", t,
		testGetDescNameValidFormat)
}

func testGetDescNameNil() {
	result := GetDescName(nil)
	convey.So(result, convey.ShouldEqual, emptyString)
}

func testGetDescNameNoFqName() {
	desc := prometheus.NewDesc(testMetricName, testHelp, nil, nil)
	patch := gomonkey.ApplyMethodReturn(desc, "String", invalidDescStr)
	defer patch.Reset()

	result := GetDescName(desc)
	convey.So(result, convey.ShouldEqual, emptyString)
}

func testGetDescNameNoComma() {
	desc := prometheus.NewDesc(testMetricName, testHelp, nil, nil)
	patch := gomonkey.ApplyMethodReturn(desc, "String", noCommaDescStr)
	defer patch.Reset()

	result := GetDescName(desc)
	convey.So(result, convey.ShouldEqual, emptyString)
}

func testGetDescNameValidFormat() {
	testCases := []struct {
		name     string
		descStr  string
		expected string
	}{
		{
			name:     "should return metric name when desc.String contains normal format with quotes",
			descStr:  normalDescStr,
			expected: testMetricName,
		},
		{
			name:     "should return metric name when desc.String contains normal format without quotes",
			descStr:  noQuoteDescStr,
			expected: testMetricName,
		},
		{
			name:     "should return correct metric name when desc.String contains another metric",
			descStr:  normalDescStr2,
			expected: testMetricName2,
		},
	}

	for _, tc := range testCases {
		desc := prometheus.NewDesc(testMetricName, testHelp, nil, nil)
		patch := gomonkey.ApplyMethodReturn(desc, "String", tc.descStr)

		result := GetDescName(desc)
		convey.So(result, convey.ShouldEqual, tc.expected)

		patch.Reset()
	}
}
