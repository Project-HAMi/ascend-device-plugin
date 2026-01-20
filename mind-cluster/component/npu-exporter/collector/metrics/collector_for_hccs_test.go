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

	"github.com/agiledragon/gomonkey/v2"
	"github.com/smartystreets/goconvey/convey"

	"ascend-common/api"
	"ascend-common/devmanager/common"
	colcommon "huawei.com/npu-exporter/v6/collector/common"
)

const (
	mockLogicID         int32  = 0
	mockMainBoardId     uint32 = 100
	errorMsgWith8001    string = "error code 8001 occurred"
	errorMsgWithout8001 string = "error code 8002 occurred"
	singleChipList      int    = 1
	unsupportedBoardId  uint32 = 999
)

type preCollectTestCase struct {
	name               string
	chipList           []colcommon.HuaWeiAIChip
	devType            string
	mainBoardId        uint32
	isA900A3SuperPod   bool
	isA9000A3SuperPod  bool
	is800IA3Chip       bool
	getStatInfoErr     error
	expectedBeginIndex int
	expectedFuncSet    bool
}

func TestPreCollect(t *testing.T) {
	n := mockNewNpuCollector()
	testCases := buildPreCollectTestCases()

	for _, tc := range testCases {
		convey.Convey(tc.name, t, func() {
			patches := gomonkey.NewPatches()
			defer patches.Reset()

			setupPatches(patches, n, tc)
			collector := &HccsCollector{}
			collector.PreCollect(n, tc.chipList)
			verifyPreCollectResult(collector, tc)
		})
	}
}

func buildPreCollectTestCases() []preCollectTestCase {
	cases := []preCollectTestCase{
		{name: "should return early when chipList is empty",
			chipList:           []colcommon.HuaWeiAIChip{},
			expectedBeginIndex: 0,
			expectedFuncSet:    false},
		{name: "should not set beginIndex when mainBoardId is not supported",
			chipList:           createMockChipList(singleChipList, unsupportedBoardId),
			devType:            api.Ascend910A3,
			mainBoardId:        unsupportedBoardId,
			getStatInfoErr:     nil,
			expectedBeginIndex: 0,
			expectedFuncSet:    true},
	}
	cases = append(cases, buildBeginIndexCases()...)
	return cases
}

func buildBeginIndexCases() []preCollectTestCase {
	return []preCollectTestCase{
		{name: "should set beginIndex to num1 when devType is Ascend910B",
			chipList:           createMockChipList(singleChipList, mockMainBoardId),
			devType:            api.Ascend910B,
			mainBoardId:        mockMainBoardId,
			getStatInfoErr:     nil,
			expectedBeginIndex: num1,
			expectedFuncSet:    true},
		{name: "should set beginIndex to num1 when IsA900A3SuperPod returns true",
			chipList:           createMockChipList(singleChipList, mockMainBoardId),
			devType:            api.Ascend910A3,
			mainBoardId:        mockMainBoardId,
			isA900A3SuperPod:   true,
			getStatInfoErr:     nil,
			expectedBeginIndex: num1,
			expectedFuncSet:    true},
		{name: "should set beginIndex to num1 when Is800IA3Chip returns true",
			chipList:           createMockChipList(singleChipList, mockMainBoardId),
			devType:            api.Ascend910A3,
			mainBoardId:        mockMainBoardId,
			is800IA3Chip:       true,
			getStatInfoErr:     nil,
			expectedBeginIndex: num1,
			expectedFuncSet:    true},
		{name: "should set beginIndex to num2 when IsA9000A3SuperPod returns true",
			chipList:           createMockChipList(singleChipList, mockMainBoardId),
			devType:            api.Ascend910A3,
			mainBoardId:        mockMainBoardId,
			isA9000A3SuperPod:  true,
			getStatInfoErr:     nil,
			expectedBeginIndex: num2,
			expectedFuncSet:    true},
	}
}

func createMockChipList(count int, mainBoardId uint32) []colcommon.HuaWeiAIChip {
	if count == 0 {
		return []colcommon.HuaWeiAIChip{}
	}
	return []colcommon.HuaWeiAIChip{
		{
			LogicID:     mockLogicID,
			MainBoardId: mainBoardId,
		},
	}
}

func setupPatches(patches *gomonkey.Patches, n *colcommon.NpuCollector, tc preCollectTestCase) {
	patches.ApplyMethodReturn(n.Dmgr, "GetDevType", tc.devType)
	patches.ApplyFuncReturn(common.IsA900A3SuperPod, tc.isA900A3SuperPod)
	patches.ApplyFuncReturn(common.IsA9000A3SuperPod, tc.isA9000A3SuperPod)
	patches.ApplyFuncReturn(common.Is800IA3Chip, tc.is800IA3Chip)
	patches.ApplyMethodReturn(n.Dmgr, "GetHccsStatisticInfoInU64",
		&common.HccsStatisticInfo{}, tc.getStatInfoErr)
}

func verifyPreCollectResult(collector *HccsCollector, tc preCollectTestCase) {
	convey.So(collector.hccsBeginIndex, convey.ShouldEqual, tc.expectedBeginIndex)
	if tc.expectedFuncSet {
		convey.So(collector.realGetStatisticInfoFunc, convey.ShouldNotBeNil)
	} else {
		convey.So(collector.realGetStatisticInfoFunc, convey.ShouldBeNil)
	}
}
