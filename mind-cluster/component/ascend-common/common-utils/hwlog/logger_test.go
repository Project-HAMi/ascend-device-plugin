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
	"errors"
	"io/fs"
	"os"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/fsnotify/fsnotify"
	"github.com/smartystreets/goconvey/convey"

	"ascend-common/common-utils/utils"
)

func TestCheckDir(t *testing.T) {
	convey.Convey("test logger", t, func() {
		convey.Convey("test check dir func", func() {
			mockStat := gomonkey.ApplyFunc(os.Stat, func(_ string) (fs.FileInfo, error) {
				return nil, os.ErrNotExist
			})
			mockMkDir := gomonkey.ApplyFunc(os.MkdirAll, func(_ string, _ fs.FileMode) error {
				return nil
			})
			defer mockStat.Reset()
			defer mockMkDir.Reset()
			err := checkDir("log")
			convey.So(err, convey.ShouldBeNil)
		})
	})
}

func TestCreateFile(t *testing.T) {
	convey.Convey("test logger", t, func() {
		convey.Convey("test create file", func() {
			mockExist := gomonkey.ApplyFunc(utils.IsExist, func(_ string) bool {
				return false
			})
			mockCreate := gomonkey.ApplyFunc(os.Create, func(_ string) (*os.File, error) {
				return nil, nil
			})
			defer mockExist.Reset()
			defer mockCreate.Reset()
			err := createFile("log")
			convey.So(err, convey.ShouldBeNil)
		})
	})
}

func TestCheckAndCreateLogFile(t *testing.T) {
	convey.Convey("test logger", t, func() {
		convey.Convey("test checkAndCreateLogFile func", func() {
			mockCreate := gomonkey.ApplyFunc(createFile, func(_ string) error {
				return nil
			})
			defer mockCreate.Reset()
			err := checkAndCreateLogFile("log")
			convey.So(err, convey.ShouldBeNil)
		})
	})
}

func TestValidateLogConfigFileMaxSize(t *testing.T) {
	convey.Convey("test logger", t, func() {
		convey.Convey("test validate max size func", func() {
			conf := &LogConfig{}
			err := validateLogConfigFileMaxSize(conf)
			convey.So(err, convey.ShouldBeNil)
			convey.So(conf.FileMaxSize, convey.ShouldEqual, DefaultFileMaxSize)
			conf.FileMaxSize = -1
			err = validateLogConfigFileMaxSize(conf)
			convey.So(err, convey.ShouldBeError)
			conf.FileMaxSize = DefaultFileMaxSize + 1
			err = validateLogConfigFileMaxSize(conf)
			convey.So(err, convey.ShouldBeError)
		})
	})
}

func TestValidateLogConfigBackups(t *testing.T) {
	convey.Convey("test logger", t, func() {
		convey.Convey("test validate backups func", func() {
			conf := &LogConfig{MaxBackups: DefaultMaxBackups}
			err := validateLogConfigBackups(conf)
			convey.So(err, convey.ShouldBeNil)
			conf.MaxBackups = 0
			err = validateLogConfigBackups(conf)
			convey.So(err, convey.ShouldBeError)
			conf.FileMaxSize = DefaultMaxBackups + 1
			err = validateLogConfigBackups(conf)
			convey.So(err, convey.ShouldBeError)
		})
	})
}

func TestValidateLogConfigMaxAge(t *testing.T) {
	convey.Convey("test logger", t, func() {
		convey.Convey("test validate max age func", func() {
			conf := &LogConfig{MaxAge: DefaultMinSaveAge}
			err := validateLogConfigMaxAge(conf)
			convey.So(err, convey.ShouldBeNil)
			conf.MaxAge = 0
			err = validateLogConfigMaxAge(conf)
			convey.So(err, convey.ShouldBeError)
		})
	})
}

func TestValidateLogLevel(t *testing.T) {
	convey.Convey("test logger", t, func() {
		convey.Convey("test validate log level func", func() {
			conf := &LogConfig{}
			err := validateLogLevel(conf)
			convey.So(err, convey.ShouldBeNil)
			conf.LogLevel = minLogLevel - 1
			err = validateLogLevel(conf)
			convey.So(err, convey.ShouldBeError)
			conf.LogLevel = maxLogLevel + 1
			err = validateLogLevel(conf)
			convey.So(err, convey.ShouldBeError)
		})
	})
}

func TestValidateMaxLineLength(t *testing.T) {
	convey.Convey("test logger", t, func() {
		convey.Convey("test validate max line length func", func() {
			conf := &LogConfig{}
			err := validateMaxLineLength(conf)
			convey.So(err, convey.ShouldBeNil)
			convey.So(conf.MaxLineLength, convey.ShouldEqual, defaultMaxEachLineLen)
			conf.MaxLineLength = -1
			err = validateMaxLineLength(conf)
			convey.So(err, convey.ShouldNotBeNil)
			conf.MaxLineLength = maxEachLineLen + 1
			err = validateMaxLineLength(conf)
			convey.So(err, convey.ShouldNotBeNil)
		})
	})
}

func TestValidateLogConfigFiled(t *testing.T) {
	convey.Convey("test logger", t, func() {
		convey.Convey("test validate config filed func", func() {
			mockCheckPath := gomonkey.ApplyFunc(utils.CheckPath, func(_ string) (string, error) {
				return "", nil
			})
			mockCheckAndCreate := gomonkey.ApplyFunc(checkAndCreateLogFile, func(_ string) error {
				return nil
			})
			defer mockCheckPath.Reset()
			defer mockCheckAndCreate.Reset()
			conf := &LogConfig{
				MaxBackups:  DefaultMaxBackups,
				MaxAge:      DefaultMinSaveAge,
				CacheSize:   DefaultCacheSize,
				ExpiredTime: DefaultExpiredTime,
			}
			err := validateLogConfigFiled(conf)
			convey.So(err, convey.ShouldBeNil)
		})
		convey.Convey("test validate config filed func, log file is relative path", func() {
			mockCheckPath := gomonkey.ApplyFunc(utils.CheckPath, func(_ string) (string, error) {
				return "", nil
			})
			mockCheckAndCreate := gomonkey.ApplyFunc(checkAndCreateLogFile, func(_ string) error {
				return nil
			})
			defer mockCheckPath.Reset()
			defer mockCheckAndCreate.Reset()
			conf := &LogConfig{
				MaxBackups:  DefaultMaxBackups,
				MaxAge:      DefaultMinSaveAge,
				CacheSize:   DefaultCacheSize,
				ExpiredTime: DefaultExpiredTime,
				LogFileName: "../",
			}
			err := validateLogConfigFiled(conf)
			expErr := errors.New("log path include invalid char")
			convey.So(err, convey.ShouldResemble, expErr)
		})
	})
}

func TestChangeFileMode(t *testing.T) {
	convey.Convey("test logger", t, func() {
		convey.Convey("test changeFileMode func", func() {
			changeFileMode(nil, fsnotify.Event{}, "log")
			mockExist := gomonkey.ApplyFunc(utils.IsExist, func(_ string) bool {
				return true
			})
			mockChmod := gomonkey.ApplyFunc(os.Chmod, func(_ string, _ fs.FileMode) error {
				return nil
			})
			defer mockExist.Reset()
			defer mockChmod.Reset()
			lg := new(logger)
			evt := fsnotify.Event{Name: "run-2022-01-01T00-00-00.123.log"}
			changeFileMode(lg, evt, "log")
		})
	})
}
