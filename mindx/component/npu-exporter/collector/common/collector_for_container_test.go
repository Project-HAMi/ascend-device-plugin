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

// Package common for general collector
package common

import (
	"sync"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/smartystreets/goconvey/convey"

	"ascend-common/common-utils/cache"
	"huawei.com/npu-exporter/v6/collector/container"
)

const (
	testCacheTime      = 60 * time.Second
	testUpdateTime     = 10 * time.Millisecond
	testDeviceID0      = 0
	testDeviceID1      = 1
	testDeviceID2      = 2
	testContainerID1   = "container1"
	testContainerID2   = "container2"
	testContainerName1 = "test-container-1"
	testContainerName2 = "test-container-2"
)

var (
	testDevicesInfos = container.DevicesInfos{
		testContainerID1: {
			ID:      testContainerID1,
			Name:    testContainerName1,
			Devices: []int{testDeviceID0, testDeviceID1},
		},
		testContainerID2: {
			ID:      testContainerID2,
			Name:    testContainerName2,
			Devices: []int{testDeviceID2},
		},
	}
)

func createTestNpuCollector() *NpuCollector {
	parser := &container.DevicesParser{}
	return &NpuCollector{
		cache:         cache.New(cacheSize),
		devicesParser: parser,
		updateTime:    testUpdateTime,
		cacheTime:     testCacheTime,
	}
}

func resetNpuContainerInfoInit() {
	npuContainerInfoInit = sync.Once{}
}

type getContainerNPUInfoTestCase struct {
	name           string
	setupCache     func(*NpuCollector)
	mockParser     func(*gomonkey.Patches, *container.DevicesParser)
	expectedResult map[int32]container.DevicesInfo
}

func createGetContainerNPUInfoTestCases() []getContainerNPUInfoTestCase {
	return []getContainerNPUInfoTestCase{
		{
			name: "should return container npu info when cache exists",
			setupCache: func(n *NpuCollector) {
				n.cache.Set(containersDevicesCacheKey, testDevicesInfos, testCacheTime)
			},
			mockParser: func(patches *gomonkey.Patches, parser *container.DevicesParser) {},
			expectedResult: map[int32]container.DevicesInfo{
				int32(testDeviceID0): testDevicesInfos[testContainerID1],
				int32(testDeviceID1): testDevicesInfos[testContainerID1],
				int32(testDeviceID2): testDevicesInfos[testContainerID2],
			},
		},
		{
			name:       "should rebuild cache when cache not exists",
			setupCache: func(n *NpuCollector) {},
			mockParser: func(patches *gomonkey.Patches, parser *container.DevicesParser) {
				patches.ApplyMethod(parser, "FetchAndParse",
					func(p *container.DevicesParser, resultOut chan<- container.DevicesInfos) {
						if resultOut != nil {
							resultOut <- testDevicesInfos
						}
					})
			},
			expectedResult: map[int32]container.DevicesInfo{
				int32(testDeviceID0): testDevicesInfos[testContainerID1],
				int32(testDeviceID1): testDevicesInfos[testContainerID1],
				int32(testDeviceID2): testDevicesInfos[testContainerID2],
			},
		},
		{
			name: "should return nil when cache type conversion failed",
			setupCache: func(n *NpuCollector) {
				n.cache.Set(containersDevicesCacheKey, "invalid type", testCacheTime)
			},
			mockParser:     func(patches *gomonkey.Patches, parser *container.DevicesParser) {},
			expectedResult: nil,
		},
	}
}

func TestGetContainerNPUInfo(t *testing.T) {
	testCases := createGetContainerNPUInfoTestCases()

	for _, tc := range testCases {
		convey.Convey(tc.name, t, func() {
			resetNpuContainerInfoInit()
			n := createTestNpuCollector()
			tc.setupCache(n)

			patches := gomonkey.NewPatches()
			defer patches.Reset()
			tc.mockParser(patches, n.devicesParser)

			result := GetContainerNPUInfo(n)
			convey.So(result, convey.ShouldResemble, tc.expectedResult)
		})
	}
}
