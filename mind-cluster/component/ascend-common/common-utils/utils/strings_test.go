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

// Package utils provides the util func
package utils

import (
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

const byteLength = 32

func TestReplacePrefix(t *testing.T) {
	convey.Convey("relative path", t, func() {
		path := ReplacePrefix("./testdata/cert/ca.crt", "****")
		convey.So(path, convey.ShouldEqual, "****testdata/cert/ca.crt")
	})
	convey.Convey("abconvey.Solute path", t, func() {
		path := ReplacePrefix("/testdata/cert/ca.crt", "****")
		convey.So(path, convey.ShouldEqual, "****estdata/cert/ca.crt")
	})
	convey.Convey("path length less than 2", t, func() {
		path := ReplacePrefix("/", "****")
		convey.So(path, convey.ShouldEqual, "****")
	})
	convey.Convey("empty string", t, func() {
		path := ReplacePrefix("", "****")
		convey.So(path, convey.ShouldEqual, "****")
	})

}

func TestMaskPrefix(t *testing.T) {
	convey.Convey("relative path", t, func() {
		path := MaskPrefix("./testdata/cert/ca.crt")
		convey.So(path, convey.ShouldEqual, "****testdata/cert/ca.crt")
	})
	convey.Convey("abconvey.Solute path", t, func() {
		path := MaskPrefix("/testdata/cert/ca.crt")
		convey.So(path, convey.ShouldEqual, "****estdata/cert/ca.crt")
	})
	convey.Convey("path length less than 2", t, func() {
		path := MaskPrefix("/")
		convey.So(path, convey.ShouldEqual, "****")
	})
	convey.Convey("empty string", t, func() {
		path := MaskPrefix("")
		convey.So(path, convey.ShouldEqual, "****")
	})

}

func TestGetSha256Code(t *testing.T) {
	convey.Convey("test sha256", t, func() {
		hashs := GetSha256Code([]byte("this is a test sentence"))
		convey.So(len(hashs), convey.ShouldEqual, byteLength)
	})
}

func TestIsDigitString(t *testing.T) {
	convey.Convey("test IsDigitString", t, func() {
		convey.Convey("case IsDigitString is true", func() {
			str := "123"
			convey.ShouldBeTrue(IsDigitString(str))
		})
		convey.Convey("case IsDigitString is false", func() {
			str := "123a"
			convey.ShouldBeFalse(IsDigitString(str))
		})
	})
}
