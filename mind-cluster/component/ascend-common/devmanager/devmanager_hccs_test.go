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

// Package devmanager for device driver manager
package devmanager

import (
	"errors"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/smartystreets/goconvey/convey"

	"ascend-common/devmanager/common"
	"ascend-common/devmanager/dcmi"
)

const (
	mockLogicID    int32  = 0
	mockCardID     int32  = 0
	mockDeviceID   int32  = 0
	invalidLogicID int32  = -1
	mockErrorMsg   string = "mock error"
	hccsArrayLen   int    = 8
)

type getHccsStatisticInfoInU64TestCase struct {
	name         string
	logicID      int32
	isValidID    bool
	getCardIDErr error
	dcmiCallErr  error
	expectedErr  bool
}

func TestGetHccsStatisticInfoInU64(t *testing.T) {
	testCases := buildGetHccsStatisticInfoInU64TestCases()

	for _, tc := range testCases {
		convey.Convey(tc.name, t, func() {
			patches := gomonkey.NewPatches()
			defer patches.Reset()

			clearIdCache(tc.logicID)
			manager := createMockDeviceManager()
			setupGetHccsStatisticInfoInU64Patches(patches, manager, tc)
			result, err := manager.GetHccsStatisticInfoInU64(tc.logicID)
			verifyGetHccsStatisticInfoInU64Result(result, err, tc)
		})
	}
}

func clearIdCache(logicID int32) {
	idCache.Delete(logicID)
}

func buildGetHccsStatisticInfoInU64TestCases() []getHccsStatisticInfoInU64TestCase {
	return []getHccsStatisticInfoInU64TestCase{
		{name: "should return failed info when logicID is invalid",
			logicID:     invalidLogicID,
			isValidID:   false,
			expectedErr: true},
		{name: "should return failed info when getCardIdAndDeviceId fails",
			logicID:      mockLogicID,
			isValidID:    true,
			getCardIDErr: errors.New(mockErrorMsg),
			expectedErr:  true},
		{name: "should return failed info when DcGetHccsStatisticInfoU64 fails",
			logicID:     mockLogicID,
			isValidID:   true,
			dcmiCallErr: errors.New(mockErrorMsg),
			expectedErr: true},
		{name: "should return success info when all operations succeed",
			logicID:     mockLogicID,
			isValidID:   true,
			expectedErr: false},
	}
}

func createMockDeviceManager() *DeviceManager {
	return &DeviceManager{
		DcMgr: &dcmi.DcManager{},
	}
}

func setupGetHccsStatisticInfoInU64Patches(patches *gomonkey.Patches,
	manager *DeviceManager, tc getHccsStatisticInfoInU64TestCase) {
	patches.ApplyFuncReturn(common.IsValidLogicIDOrPhyID, tc.isValidID)
	if !tc.isValidID {
		return
	}
	if tc.getCardIDErr != nil {
		patches.ApplyMethodReturn(manager.DcMgr, "DcGetCardIDDeviceID",
			mockCardID, mockDeviceID, tc.getCardIDErr)
	} else {
		patches.ApplyMethodReturn(manager.DcMgr, "DcGetCardIDDeviceID",
			mockCardID, mockDeviceID, nil)
		if tc.dcmiCallErr != nil {
			patches.ApplyMethodReturn(manager.DcMgr, "DcGetHccsStatisticInfoU64",
				common.HccsStatisticInfo{}, tc.dcmiCallErr)
		} else {
			mockHccsInfo := createMockHccsStatisticInfo()
			patches.ApplyMethodReturn(manager.DcMgr, "DcGetHccsStatisticInfoU64",
				mockHccsInfo, nil)
		}
	}
}

func createMockHccsStatisticInfo() common.HccsStatisticInfo {
	txCnt := make([]uint64, hccsArrayLen)
	rxCnt := make([]uint64, hccsArrayLen)
	crcErrCnt := make([]uint64, hccsArrayLen)
	for i := 0; i < hccsArrayLen; i++ {
		txCnt[i] = uint64(i + 1)
		rxCnt[i] = uint64(i + 1)
		crcErrCnt[i] = 0
	}
	return common.HccsStatisticInfo{
		TxCnt:     txCnt,
		RxCnt:     rxCnt,
		CrcErrCnt: crcErrCnt,
	}
}

func verifyGetHccsStatisticInfoInU64Result(result *common.HccsStatisticInfo,
	err error, tc getHccsStatisticInfoInU64TestCase) {
	if tc.expectedErr {
		convey.So(err, convey.ShouldNotBeNil)
		convey.So(result, convey.ShouldNotBeNil)
		verifyFailedHccsInfo(result)
	} else {
		convey.So(err, convey.ShouldBeNil)
		convey.So(result, convey.ShouldNotBeNil)
		verifySuccessHccsInfo(result)
	}
}

func verifyFailedHccsInfo(result *common.HccsStatisticInfo) {
	convey.So(len(result.TxCnt), convey.ShouldEqual, hccsArrayLen)
	convey.So(len(result.RxCnt), convey.ShouldEqual, hccsArrayLen)
	convey.So(len(result.CrcErrCnt), convey.ShouldEqual, hccsArrayLen)
	for i := 0; i < hccsArrayLen; i++ {
		convey.So(result.TxCnt[i], convey.ShouldEqual, common.FailedValue)
		convey.So(result.RxCnt[i], convey.ShouldEqual, common.FailedValue)
		convey.So(result.CrcErrCnt[i], convey.ShouldEqual, common.FailedValue)
	}
}

func verifySuccessHccsInfo(result *common.HccsStatisticInfo) {
	convey.So(len(result.TxCnt), convey.ShouldEqual, hccsArrayLen)
	convey.So(len(result.RxCnt), convey.ShouldEqual, hccsArrayLen)
	convey.So(len(result.CrcErrCnt), convey.ShouldEqual, hccsArrayLen)
	convey.So(result.TxCnt[0], convey.ShouldEqual, uint64(1))
	convey.So(result.RxCnt[0], convey.ShouldEqual, uint64(1))
}
