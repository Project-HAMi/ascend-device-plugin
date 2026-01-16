/* Copyright(C) 2021. Huawei Technologies Co.,Ltd. All rights reserved.
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

package common

import (
	"fmt"
	"strings"
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

// TestDeepCopyHccsBandwidthInfo TestDeepCopySlice
func TestDeepCopyHccsBandwidthInfo(t *testing.T) {

	convey.Convey("should copy a new []int", t, func() {
		slice := []int{1, 2}
		newSlice := deepCopySlice(slice)
		convey.So(&newSlice, convey.ShouldNotEqual, &slice)
	})

	convey.Convey("should copy a new []int32", t, func() {
		slice := []uint32{1, 2}

		newSlice := deepCopySlice(slice)
		convey.So(&newSlice, convey.ShouldNotEqual, &slice)
	})

	convey.Convey("should copy a new []float64", t, func() {
		slice := []float64{1, 2}
		newSlice := deepCopySlice(slice)
		convey.So(&newSlice, convey.ShouldNotEqual, &slice)
	})
}

func TestIsValidPortID(t *testing.T) {
	convey.Convey("Given a port ID", t, func() {
		convey.Convey("01-When the port ID is invalid, should return false", func() {
			portID1 := 1
			convey.So(IsValidPortID(portID1), convey.ShouldBeFalse)
		})

		convey.Convey("02-When the port ID is the default, should return true", func() {
			portID3 := DefaultPingMeshPortID
			convey.So(IsValidPortID(portID3), convey.ShouldBeTrue)
		})
	})
}

func TestIsValidTaskID(t *testing.T) {
	convey.Convey("Given a task ID", t, func() {
		convey.Convey("01-When the task ID is valid, should return true", func() {
			taskID1 := InternalPingMeshTaskID
			convey.So(IsValidTaskID(taskID1), convey.ShouldBeTrue)

			taskID2 := ExternalPingMeshTaskID
			convey.So(IsValidTaskID(taskID2), convey.ShouldBeTrue)
		})

		convey.Convey("02-When the task ID is invalid, should return false", func() {
			const taskID3 = 3
			convey.So(IsValidTaskID(taskID3), convey.ShouldBeFalse)
		})
	})
}

func defaultHccspingMeshOperate() HccspingMeshOperate {
	return HccspingMeshOperate{
		DstAddr:      "1111",
		PktSize:      MinPktSize,
		PktSendNum:   MinPktSendNum,
		PktInterval:  MinPktInterval,
		TaskInterval: MinTaskInterval,
		TaskId:       int(InternalPingMeshTaskID),
	}
}

func check(op HccspingMeshOperate, expectedErr error) {
	err := IsValidHccspingMeshOperate(op)
	convey.So(err, convey.ShouldResemble, expectedErr)
}

func expectedError(pattern string, current, min, max int) error {
	return fmt.Errorf(pattern, current, min, max)
}

func TestIsValidHccspingMeshOperate01(t *testing.T) {
	convey.Convey("Given a pingmesh operate", t, func() {
		op := defaultHccspingMeshOperate()
		convey.Convey("01-When operation valid, should return nil", func() {
			check(op, nil)
		})
		var expectedErr error
		convey.Convey("01-When the dst addr is invalid, should return error", func() {
			op.DstAddr = strings.Repeat("a", MaxHccspingMeshAddr+1)
			expectedErr = fmt.Errorf("dst addr length %d is invalid, should not be greater than %d", len(op.DstAddr),
				MaxHccspingMeshAddr)
			check(op, expectedErr)
		})
		op.DstAddr = "1111"
		convey.Convey("02-When the pkt size is invalid, should return error", func() {
			pattern := "pkt size %d is invalid, should be between %d and %d"
			op.PktSize = MinPktSize - 1
			check(op, expectedError(pattern, op.PktSize, MinPktSize, MaxPktSize))
			op.PktSize = MaxPktSize + 1
			check(op, expectedError(pattern, op.PktSize, MinPktSize, MaxPktSize))
		})
		op.PktSize = MinPktSize
		convey.Convey("03-When the pkt send num is invalid, should return error", func() {
			pattern := "pkt send num %d is invalid, should be between %d and %d"
			op.PktSendNum = MinPktSendNum - 1
			check(op, expectedError(pattern, op.PktSendNum, MinPktSendNum, MaxPktSendNum))
			op.PktSendNum = MaxPktSendNum + 1
			check(op, expectedError(pattern, op.PktSendNum, MinPktSendNum, MaxPktSendNum))
		})
		op.TaskInterval = MinTaskInterval
		convey.Convey("06-When the task id is invalid, should return error", func() {
			op.TaskId = int(ExternalPingMeshTaskID) + 1
			expectedErr = fmt.Errorf("task id %d is invalid", op.TaskId)
			check(op, expectedErr)
		})
	})
}

func TestIsValidHccspingMeshOperate02(t *testing.T) {
	convey.Convey("Given a pingmesh operate", t, func() {
		op := defaultHccspingMeshOperate()
		convey.Convey("04-When the pkt interval is invalid, should return error", func() {
			pattern := "pkt interval %d is invalid, should be between %d and %d"
			op.PktInterval = MinPktInterval - 1
			check(op, expectedError(pattern, op.PktInterval, MinPktInterval, MaxPktInterval))
			op.PktInterval = MaxPktInterval + 1
			check(op, expectedError(pattern, op.PktInterval, MinPktInterval, MaxPktInterval))
		})
		op.PktInterval = MinPktInterval
		convey.Convey("05-When the task interval is invalid, should return error", func() {
			pattern := "task interval %d is invalid, should be between %d and %d"
			op.TaskInterval = MinTaskInterval - 1
			check(op, expectedError(pattern, op.TaskInterval, MinTaskInterval, MaxTaskInterval))
			op.TaskInterval = MaxTaskInterval + 1
			check(op, expectedError(pattern, op.TaskInterval, MinTaskInterval, MaxTaskInterval))
		})
		op.TaskInterval = MinTaskInterval
		var expectedErr error
		convey.Convey("06-When the task id is invalid, should return error", func() {
			op.TaskId = int(ExternalPingMeshTaskID) + 1
			expectedErr = fmt.Errorf("task id %d is invalid", op.TaskId)
			check(op, expectedErr)
		})
	})
}
