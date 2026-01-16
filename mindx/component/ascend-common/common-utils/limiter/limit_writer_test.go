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

// Package limiter implement a writer limiter
package limiter

import (
	"io"
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

func TestLimitWriterWrite(t *testing.T) {
	convey.Convey("test limiter Writer write function", t, func() {
		data := []byte("test")
		limitBuffer := NewLimitedWriter(len(data))

		n, err := limitBuffer.Write(data)
		convey.So(err, convey.ShouldBeNil)
		convey.So(n, convey.ShouldEqual, len(data))
		n, err = limitBuffer.Write(data)
		convey.So(err, convey.ShouldEqual, io.EOF)
		convey.So(n, convey.ShouldEqual, 0)
	})
}
