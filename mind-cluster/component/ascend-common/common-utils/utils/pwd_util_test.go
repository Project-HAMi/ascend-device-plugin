/* Copyright(C) 2023. Huawei Technologies Co.,Ltd. All rights reserved.
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

// Package utils this file for password handler
package utils

import (
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

var (
	truePasswd   = []byte("aA0!\"#$%&'()*+,-. /:;<=>?@[\\]^_`{|}~")
	falsePasswd1 = []byte("userName")
	falsePasswd2 = []byte("12345678")
	falsePasswd3 = []byte("1234567")
	falsePasswd4 = []byte("emaNresu.")
	falsePasswd5 = []byte("不支持特殊字符测试test")
)

// TestCommonCheckForPassWord test common check for passWord
func TestCommonCheckForPassWord(t *testing.T) {
	convey.Convey("correct password", t, func() {
		err := ValidatePassWord("userName", truePasswd)
		convey.So(err, convey.ShouldBeNil)
	})
	convey.Convey("username == password", t, func() {
		err := ValidatePassWord("userName", falsePasswd1)
		convey.So(err.Error(), convey.ShouldEqual, "password cannot equals username")
	})
	convey.Convey("complex not meet the requirement", t, func() {
		err := ValidatePassWord("userName", falsePasswd2)
		convey.So(err.Error(), convey.ShouldEqual, "password complex not meet the requirement")
	})
	convey.Convey("password too short", t, func() {
		err := ValidatePassWord("userName", falsePasswd3)
		convey.So(err.Error(), convey.ShouldEqual, "password not meet requirement")
	})
	convey.Convey("username equal reverse password", t, func() {
		err := ValidatePassWord(".userName", falsePasswd4)
		convey.So(err.Error(), convey.ShouldEqual, "password cannot equal reversed username")
	})
	convey.Convey("test special ", t, func() {
		err := ValidatePassWord("userName", falsePasswd5)
		convey.So(err.Error(), convey.ShouldEqual, "password not meet requirement")
	})
}
