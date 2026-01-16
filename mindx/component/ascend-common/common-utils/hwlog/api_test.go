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
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/smartystreets/goconvey/convey"

	"ascend-common/common-utils/utils"
)

func TestNewLogger(t *testing.T) {
	convey.Convey("test api", t, func() {
		convey.Convey("test setLogger func", func() {
			lgConfig := &LogConfig{
				OnlyToStdout: true,
			}
			lg := new(logger)
			err := lg.setLogger(lgConfig)
			convey.So(err, convey.ShouldBeNil)
			// test for log file
			mockPathCheck := gomonkey.ApplyFunc(utils.CheckPath, func(_ string) (string, error) {
				return "", nil
			})
			mockMkdir := gomonkey.ApplyFunc(os.Chmod, func(_ string, _ fs.FileMode) error {
				return nil
			})
			defer mockPathCheck.Reset()
			defer mockMkdir.Reset()
			lgConfig = &LogConfig{
				LogFileName: path.Join(filepath.Dir(os.Args[0]), "t.log"),
				OnlyToFile:  true,
				MaxBackups:  DefaultMaxBackups,
				MaxAge:      DefaultMinSaveAge,
				CacheSize:   DefaultCacheSize,
				ExpiredTime: DefaultExpiredTime,
			}
			err = lg.setLogger(lgConfig)
			convey.So(err, convey.ShouldBeNil)
		})
	})
}

func TestLoggerPrint(t *testing.T) {
	convey.Convey("test api", t, func() {
		convey.Convey("test logger print func", func() {
			lgConfig := &LogConfig{
				OnlyToStdout: true,
				LogLevel:     -1,
			}
			lg := new(logger)
			err := lg.setLogger(lgConfig)
			convey.So(err, convey.ShouldBeNil)
			lg.Debug("test debug")
			lg.Debugf("test debugf")
			lg.Info("test info")
			lg.Infof("test infof")
			lg.Warn("test warn")
			lg.Warnf("test warnf")
			lg.Error("test error")
			lg.Errorf("test errorf")
			lg.Critical("test critical")
			lg.Criticalf("test criticalf")
			lg.setLoggerLevel(maxLogLevel + 1)
			lg.Debug("test debug")
			lg.Debugf("test debugf")
			lg.Info("test info")
			lg.Infof("test infof")
			lg.Warn("test warn")
			lg.Warnf("test warnf")
			lg.Error("test error")
			lg.Errorf("test errorf")
			lg.Critical("test critical")
			lg.Criticalf("test criticalf")
		})
	})
}
func TestLoggerPrintWithLimit(t *testing.T) {
	convey.Convey("test api", t, func() {
		convey.Convey("test logger print func with limit", func() {
			lgConfig := &LogConfig{
				OnlyToStdout: true,
				LogLevel:     -1,
			}
			lg := new(logger)
			err := lg.setLogger(lgConfig)
			convey.So(err, convey.ShouldBeNil)
			domain := "hccs"
			logicId := 1

			errFormat := "collect failed ,err:%v"
			collectErr := fmt.Errorf("detail errs,logicId(%d)", logicId)
			lg.ErrorfWithLimit(domain, logicId, errFormat, collectErr)
			lg.ErrorfWithLimit(domain, logicId, errFormat, collectErr)
			lg.ErrorfWithLimit(domain, logicId, errFormat, collectErr)
			lg.ErrorfWithLimit(domain, logicId, errFormat, collectErr)
			ResetErrCnt(domain, logicId)
			lg.ErrorfWithLimit(domain, logicId, errFormat, collectErr)
			lg.ErrorfWithLimit(domain, logicId, errFormat, collectErr)
		})
	})
}

func TestWarnfWithLimit(t *testing.T) {
	convey.Convey("test api", t, func() {
		convey.Convey("test warn logger print func with limit", func() {
			lgConfig := &LogConfig{
				OnlyToStdout: true,
				LogLevel:     -1,
			}
			lg := new(logger)
			err := lg.setLogger(lgConfig)
			convey.So(err, convey.ShouldBeNil)
			domain := "hccs"
			logicId := 1

			errFormat := "collect failed ,err:%v"
			collectErr := fmt.Errorf("detail errs,logicId(%d)", logicId)
			lg.WarnfWithLimit(domain, logicId, errFormat, collectErr)
			lg.WarnfWithLimit(domain, logicId, errFormat, collectErr)
			lg.WarnfWithLimit(domain, logicId, errFormat, collectErr)
			lg.WarnfWithLimit(domain, logicId, errFormat, collectErr)
			ResetErrCnt(domain, logicId)
			lg.WarnfWithLimit(domain, logicId, errFormat, collectErr)
			lg.WarnfWithLimit(domain, logicId, errFormat, collectErr)
		})
	})
}

func TestValidate(t *testing.T) {
	convey.Convey("test api", t, func() {
		convey.Convey("test validate", func() {
			lg := new(logger)
			res := lg.validate()
			convey.So(res, convey.ShouldBeFalse)
			lgConfig := &LogConfig{
				OnlyToStdout: true,
			}
			err := lg.setLogger(lgConfig)
			convey.So(err, convey.ShouldBeNil)
			res = lg.validate()
			convey.So(res, convey.ShouldBeTrue)
		})
	})
}
