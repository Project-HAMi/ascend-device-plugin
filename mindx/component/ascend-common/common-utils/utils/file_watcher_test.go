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

// Package utils test for file watcher utils
package utils

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/fsnotify/fsnotify"
	"github.com/smartystreets/goconvey/convey"
)

var testErr = errors.New("test error")

const (
	testFilePath = "./test.txt"
	errFilePath  = "./not_exist_file.txt"
)

func TestGetFileWatcherChan(t *testing.T) {
	prepareTestFile(t)
	defer removeFile()

	p1 := gomonkey.ApplyFuncReturn(PathStringChecker, "", nil)
	defer p1.Reset()
	convey.Convey("test func GetFileWatcherChan success", t, func() {
		_, err := GetFileWatcherChan(testFilePath)
		convey.So(err, convey.ShouldBeNil)
	})
	convey.Convey("test func GetFileWatcherChan failed, new watcher err", t, func() {
		p2 := gomonkey.ApplyFuncReturn(fsnotify.NewWatcher, nil, testErr)
		defer p2.Reset()
		_, err := GetFileWatcherChan(testFilePath)
		expErr := fmt.Errorf("new file watcher failed, error: %v", testErr)
		convey.So(err, convey.ShouldResemble, expErr)
	})
	convey.Convey("test func GetFileWatcherChan failed, file does not exist", t, func() {
		_, err := GetFileWatcherChan(errFilePath)
		expErr := fmt.Sprintf("watch file <%s> failed", errFilePath)
		convey.So(err.Error(), convey.ShouldContainSubstring, expErr)
	})
	convey.Convey("test func GetFileWatcherChan failed, watcher is nil", t, func() {
		var watcher = &FileWatcher{}
		eventCh := watcher.Events()
		convey.So(eventCh, convey.ShouldBeNil)
		errCh := watcher.Errors()
		convey.So(errCh, convey.ShouldBeNil)
		err := watcher.Close()
		convey.So(err, convey.ShouldBeNil)
	})
}

func prepareTestFile(t *testing.T) {
	const mode644 = 0644
	err := os.WriteFile(testFilePath, []byte("file context"), mode644)
	if err != nil {
		t.Error(err)
	}
}

func removeFile() {
	if err := os.Remove(testFilePath); err != nil && errors.Is(err, os.ErrNotExist) {
		fmt.Printf("remove file %s failed, %v\n", testFilePath, err)
	}
}
