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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/smartystreets/goconvey/convey"
)

func TestReadLimitBytes(t *testing.T) {
	convey.Convey("test ReadLimitBytes func", t, func() {
		convey.Convey("should return nil given empty string", func() {
			emptyString := ""
			const limitLength = 10
			res, err := ReadLimitBytes(emptyString, limitLength)
			convey.So(res, convey.ShouldBeNil)
			convey.So(err, convey.ShouldBeError)
		})

		convey.Convey("should not return nil given valid path", func() {
			const limitLength = 10
			res, err := ReadLimitBytes("../../go.mod", limitLength)
			convey.So(res, convey.ShouldNotBeNil)
			convey.So(err, convey.ShouldBeNil)
		})

		convey.Convey("should return nil given invalid limit length", func() {
			const limitLength = -1
			res, err := ReadLimitBytes("../../go.mod", limitLength)
			convey.So(res, convey.ShouldBeNil)
			convey.So(err.Error(), convey.ShouldEqual, "the limit length is not valid")
		})

		convey.Convey("should return nil when check path failed", func() {
			checkStub := gomonkey.ApplyFunc(CheckPath, func(path string) (string, error) {
				return "", errors.New("check failed")
			})
			defer checkStub.Reset()
			const limitLength = 10
			res, err := ReadLimitBytes("../../go.mod", limitLength)
			convey.So(res, convey.ShouldBeNil)
			convey.So(err.Error(), convey.ShouldEqual, "check failed")
		})

		convey.Convey("should return nil when read file failed", func() {
			var file *os.File
			checkStub := gomonkey.ApplyMethod(reflect.TypeOf(file), "Read",
				func(_ *os.File, _ []byte) (int, error) {
					return 0, errors.New("read file failed")
				})
			defer checkStub.Reset()
			const limitLength = 10
			res, err := ReadLimitBytes("../../go.mod", limitLength)
			convey.So(res, convey.ShouldBeNil)
			convey.So(err.Error(), convey.ShouldEqual, "read file failed: read file failed")
		})
	})
}

func TestLoadFile(t *testing.T) {
	convey.Convey("test LoadFile func", t, func() {
		convey.Convey("should return error given empty path", func() {
			res, err := LoadFile("")
			convey.So(res, convey.ShouldBeNil)
			convey.So(err, convey.ShouldBeNil)
		})

		convey.Convey("should return nil given path not existing", func() {
			res, err := LoadFile("xxxx")
			convey.So(res, convey.ShouldBeNil)
			convey.So(err, convey.ShouldBeNil)
		})

		convey.Convey("should not return nil given valid path", func() {
			res, err := LoadFile("../../go.mod")
			convey.So(res, convey.ShouldNotBeNil)
			convey.So(err, convey.ShouldBeNil)
		})

		convey.Convey("should return nil given invalid path", func() {
			absStub := gomonkey.ApplyFunc(filepath.Abs, func(path string) (string, error) {
				return "", errors.New("the path is invalid")
			})
			defer absStub.Reset()
			res, err := LoadFile("../../go.mod")
			convey.So(res, convey.ShouldBeNil)
			convey.So(err.Error(), convey.ShouldEqual, "the filePath is invalid: the path is invalid")
		})

		convey.Convey("should return nil when read file failed", func() {
			readStub := gomonkey.ApplyFunc(ReadLimitBytes, func(path string, limitLength int) ([]byte, error) {
				return nil, errors.New("read file failed")
			})
			defer readStub.Reset()
			res, err := LoadFile("../../go.mod")
			convey.So(res, convey.ShouldBeNil)
			convey.So(err.Error(), convey.ShouldEqual, "read file failed")
		})
	})
}

func TestCopyDir(t *testing.T) {
	convey.Convey("test CopyDir func", t, func() {
		convey.Convey("should return error given empty src path", func() {
			err := CopyDir("", "")
			convey.So(err, convey.ShouldNotBeNil)
		})
		convey.Convey("should return error given file src path", func() {
			err := CopyDir("../../go.mod", "")
			convey.So(err, convey.ShouldNotBeNil)
		})
		convey.Convey("should return nil given dir src path", func() {
			err := CopyDir("../utils", "../utils_test")
			convey.So(err, convey.ShouldBeNil)
		})
		convey.Convey("should return error given file dst path", func() {
			err := CopyDir("../utils", "../utils_test/file_test.go")
			convey.So(err, convey.ShouldNotBeNil)
		})
	})
}

func TestCopyFile(t *testing.T) {
	convey.Convey("test CopyFile func", t, func() {
		convey.Convey("should return error given empty src file path", func() {
			err := CopyFile("", "../utils_test/file_test.go")
			convey.So(err, convey.ShouldNotBeNil)
		})
		convey.Convey("should return error given empty dst path", func() {
			err := CopyFile("../utils_test/file_test.go", "")
			convey.So(err, convey.ShouldNotBeNil)
		})
		convey.Convey("should return error given dir scr path", func() {
			err := CopyFile("../utils", "../utils_test/file_test.go")
			convey.So(err, convey.ShouldNotBeNil)
		})
		convey.Convey("should return error given dir dst path", func() {
			err := CopyFile("../utils/file_test.go", "../utils_test")
			convey.So(err, convey.ShouldNotBeNil)
		})
		convey.Convey("should return nil given file scr and dst path", func() {
			err := CopyFile("../utils/file_test.go", "../utils_test/file_test.go")
			convey.So(err, convey.ShouldBeNil)
		})
	})
	if err := os.RemoveAll("../utils_test"); err != nil {
		fmt.Print("remove util_test file failed")
	}
}
