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

// Package hwlog test file
package hwlog

import (
	"context"
	"errors"
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

func TestInitRunLogger(t *testing.T) {
	convey.Convey("test hwlog adaptor", t, func() {
		convey.Convey("test init run log", func() {
			ctx, cancel := context.WithCancel(context.TODO())
			err := InitRunLogger(nil, ctx)
			convey.So(err, convey.ShouldBeError, errors.New("run logger config is nil"))
			lgConfig := &LogConfig{OnlyToStdout: true}
			err = InitRunLogger(lgConfig, ctx)
			convey.So(err, convey.ShouldBeNil)
			// repeat initialize
			err = InitRunLogger(lgConfig, ctx)
			convey.So(err, convey.ShouldBeNil)
			cancel()
		})
	})
}

func TestNewCustomLogger(t *testing.T) {
	convey.Convey("test hwlog adaptor", t, func() {
		convey.Convey("test init custom log", func() {
			ctx, cancel := context.WithCancel(context.TODO())
			_, err := NewCustomLogger(nil, ctx)
			convey.So(err, convey.ShouldBeError, errors.New("custom logger config is nil"))
			lgConfig := &LogConfig{OnlyToStdout: true}
			_, err = NewCustomLogger(lgConfig, ctx)
			convey.So(err, convey.ShouldBeNil)
			// repeat initialize
			_, err = NewCustomLogger(lgConfig, ctx)
			convey.So(err, convey.ShouldBeNil)
			cancel()
		})
	})
}

func TestInitOperateLogger(t *testing.T) {
	convey.Convey("test hwlog adaptor", t, func() {
		convey.Convey("test init operate log", func() {
			ctx, cancel := context.WithCancel(context.TODO())
			err := InitOperateLogger(nil, ctx)
			convey.So(err, convey.ShouldBeError, errors.New("operate logger config is nil"))
			lgConfig := &LogConfig{OnlyToStdout: true}
			err = InitOperateLogger(lgConfig, ctx)
			convey.So(err, convey.ShouldBeNil)
			// repeat initialize
			err = InitOperateLogger(lgConfig, ctx)
			convey.So(err, convey.ShouldBeNil)
			cancel()
		})
	})
}

func TestInitSecurityLogger(t *testing.T) {
	convey.Convey("test hwlog adaptor", t, func() {
		convey.Convey("test init security log", func() {
			ctx, cancel := context.WithCancel(context.TODO())
			err := InitSecurityLogger(nil, ctx)
			convey.So(err, convey.ShouldBeError, errors.New("security logger config is nil"))
			lgConfig := &LogConfig{OnlyToStdout: true}
			err = InitSecurityLogger(lgConfig, ctx)
			convey.So(err, convey.ShouldBeNil)
			// repeat initialize
			err = InitSecurityLogger(lgConfig, ctx)
			convey.So(err, convey.ShouldBeNil)
			cancel()
		})
	})
}

func TestInitUserLogger(t *testing.T) {
	convey.Convey("test hwlog adaptor", t, func() {
		convey.Convey("test init user log", func() {
			ctx, cancel := context.WithCancel(context.TODO())
			err := InitUserLogger(nil, ctx)
			convey.So(err, convey.ShouldBeError, errors.New("user logger config is nil"))
			lgConfig := &LogConfig{OnlyToStdout: true}
			err = InitUserLogger(lgConfig, ctx)
			convey.So(err, convey.ShouldBeNil)
			// repeat initialize
			err = InitUserLogger(lgConfig, ctx)
			convey.So(err, convey.ShouldBeNil)
			cancel()
		})
	})
}

func TestInitDebugLogger(t *testing.T) {
	convey.Convey("test hwlog adaptor", t, func() {
		convey.Convey("test init debug log", func() {
			ctx, cancel := context.WithCancel(context.TODO())
			err := InitDebugLogger(nil, ctx)
			convey.So(err, convey.ShouldBeError, errors.New("debug logger config is nil"))
			lgConfig := &LogConfig{OnlyToStdout: true}
			err = InitDebugLogger(lgConfig, ctx)
			convey.So(err, convey.ShouldBeNil)
			// repeat initialize
			err = InitDebugLogger(lgConfig, ctx)
			convey.So(err, convey.ShouldBeNil)
			cancel()
		})
	})
}
