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

// Package rand implement the security rand
package rand

import (
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/smartystreets/goconvey/convey"
)

const (
	illegalSize = 1 << 25
)

func TestInnerRead(t *testing.T) {
	convey.Convey("test random read func", t, func() {
		reader := &randomReader{}
		convey.Convey("read size too large, err returned", func() {
			bs := make([]byte, illegalSize, illegalSize)
			r, err := reader.Read(bs)
			convey.So(err.Error(), convey.ShouldEqual, "byte size is too large")
			convey.So(r, convey.ShouldEqual, 0)
		})
		convey.Convey("windows,err returned", func() {
			mock := gomonkey.ApplyGlobalVar(&supportOs, "windows")
			defer mock.Reset()
			bs := make([]byte, 1, 1)
			r, err := reader.Read(bs)
			convey.So(err.Error(), convey.ShouldEqual, "not supported")
			convey.So(r, convey.ShouldEqual, 0)
		})
		convey.Convey("normal situation,no err returned", func() {
			//  the length of byte is one, to prevent block when generate random
			bs := make([]byte, 1, 1)
			r, err := reader.Read(bs)
			convey.So(err, convey.ShouldEqual, nil)
			convey.So(r, convey.ShouldEqual, 1)
		})
	})
}
