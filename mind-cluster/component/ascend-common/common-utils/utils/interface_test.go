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

// Package utils offer the some utils for certificate handling
package utils

import (
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

func TestIsNil(t *testing.T) {
	var a interface{}               // type = nil, data = nil
	var b interface{} = (*int)(nil) // type is *int , data = nil
	var c interface{} = "dd"
	convey.Convey("test IsNil func, type and data is both nil", t, func() {
		convey.So(a == nil, convey.ShouldEqual, true)
		convey.So(b == nil, convey.ShouldEqual, false)
		convey.So(c == nil, convey.ShouldEqual, false)
		convey.So(IsNil(a), convey.ShouldEqual, true)
		convey.So(IsNil(b), convey.ShouldEqual, true)
		convey.So(IsNil(c), convey.ShouldEqual, false)
	})
}
