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

// Package utils env test
package utils

import (
	"fmt"
	"os/user"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/smartystreets/goconvey/convey"
)

func TestGetCurrentUid(t *testing.T) {
	convey.Convey("test func GetCurrentUid success", t, func() {
		var p1 = gomonkey.ApplyFuncReturn(user.Current, &user.User{Uid: "0"}, nil)
		defer p1.Reset()
		uid, err := GetCurrentUid()
		convey.So(err, convey.ShouldBeNil)
		convey.So(uid, convey.ShouldEqual, 0)
	})
	convey.Convey("test func GetCurrentUid failed, get current user info failed", t, func() {
		var p1 = gomonkey.ApplyFuncReturn(user.Current, nil, testErr)
		defer p1.Reset()
		uid, err := GetCurrentUid()
		expErr := fmt.Errorf("get current user info failed: %v", testErr)
		convey.So(err, convey.ShouldResemble, expErr)
		convey.So(uid, convey.ShouldEqual, 0)
	})
	convey.Convey("test func GetCurrentUid failed, uid is invalid", t, func() {
		var p1 = gomonkey.ApplyFuncReturn(user.Current, &user.User{Uid: "invalid uid"}, nil)
		defer p1.Reset()
		uid, err := GetCurrentUid()
		convey.So(err, convey.ShouldNotBeNil)
		convey.So(err.Error(), convey.ShouldContainSubstring, "convert uid to int failed")
		convey.So(uid, convey.ShouldEqual, 0)
	})
}
