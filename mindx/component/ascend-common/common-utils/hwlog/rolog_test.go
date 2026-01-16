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

// Package hwlog provides the capability of processing Huawei log rules.
package hwlog

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/smartystreets/goconvey/convey"
)

const (
	testDirPermission  = 0700
	testFilePermission = 0600
	testMByte          = 1
	testCapacity       = 10
	testCapacity2      = 100
	testCapacity3      = 5
	testSaveTime       = 10
	testSaveTime2      = 7
	testSaveVolume     = 3
	testSaveVolume2    = 1
	fileCountOne       = 1
	fileCountTwo       = 2
	fileCountFour      = 4
	waitTime           = 50
	oneDayHour         = 24
	sevenDays          = 7
	fourteenDays       = 14
	twentyOneDays      = 21
	testYear           = 2014
	testMonth          = 5
	testDay            = 4
	testHour           = 14
	testMin            = 44
	testSec            = 33
	testNsec           = 555000000
)

// TestCreate for test the function of create log file
func TestCreate(t *testing.T) {
	convey.Convey("TestCreate", t, func() {
		dir := makeTempDir("TestCrate")
		defer os.RemoveAll(dir)
		l := &Logs{
			FileName: getLogFile(dir),
		}
		defer l.Close()

		input := []byte("foobarfoobar!")
		fileWrite(input, l)
		existWithContent(input, getLogFile(dir))
		fileCount(fileCountOne, dir)
	})
}

// TestOpenFile for test the function of open log file
func TestOpenFile(t *testing.T) {
	convey.Convey("TestOpenFile", t, func() {
		dir := makeTempDir("TestOpenFile")
		defer os.RemoveAll(dir)
		fileName := getLogFile(dir)
		data := []byte("foo!")
		err := ioutil.WriteFile(fileName, data, testFilePermission)
		convey.So(err, convey.ShouldBeNil)
		existWithContent(data, fileName)

		l := &Logs{
			FileName: fileName,
		}
		defer l.Close()

		b := []byte("boo!")
		fileWrite(b, l)
		existWithContent(append(data, b...), fileName)
		fileCount(fileCountOne, dir)
	})
}

// TestWriteTooLong for test the processing of the overlong write error
func TestWriteTooLong(t *testing.T) {
	convey.Convey("TestWriteTooLong", t, func() {
		mByte = testMByte
		dir := makeTempDir("TestWriteTooLong")
		defer os.RemoveAll(dir)

		l := &Logs{
			FileName: getLogFile(dir),
			Capacity: testCapacity3,
		}
		defer l.Close()

		b := []byte("barrrrrrrrrrrrrrrrr!")
		n, err := l.Write(b)
		convey.So(err, convey.ShouldNotBeNil)
		convey.So(0, convey.ShouldEqual, n)
		convey.So(err.Error(), convey.ShouldEqual, fmt.Sprintf(
			"the write lenth %d is greater than the maximum file size %d", len(b), l.Capacity))
		_, err = os.Stat(getLogFile(dir))
		convey.So(err, shouldNotBeExist)
	})
}

// TestMakeLogDir for test the function of make log file directory
func TestMakeLogDir(t *testing.T) {
	convey.Convey("TestMakeLogDir", t, func() {
		dir := time.Now().Format("TestMakeLogDir" + timeFormat)
		dir = filepath.Join(os.TempDir(), dir)
		defer os.RemoveAll(dir)

		fileName := getLogFile(dir)
		l := &Logs{
			FileName: fileName,
		}
		defer l.Close()

		b := []byte("boo!")
		fileWrite(b, l)
		existWithContent(b, getLogFile(dir))
		fileCount(fileCountOne, dir)
	})
}

// TestDefaultFileName for test default log file name
func TestDefaultFileName(t *testing.T) {
	convey.Convey("TestDefaultFileName", t, func() {
		dir := os.TempDir()
		fileName := filepath.Join(dir, filepath.Base(os.Args[0])+"-mindx-dl.log")
		defer os.Remove(fileName)

		l := &Logs{}
		defer l.Close()

		b := []byte("boo!")
		fileWrite(b, l)
		existWithContent(b, fileName)
	})
}

// TestAutoRoll for test the automatic log rolling
func TestAutoRoll(t *testing.T) {
	convey.Convey("TestAutoRoll", t, func() {
		mByte = testMByte
		dir := makeTempDir("TestAutoRoll")
		defer os.RemoveAll(dir)
		currentTime := time.Now()

		fileName := getLogFile(dir)
		l := &Logs{
			FileName: fileName,
			Capacity: testCapacity,
		}
		defer l.Close()

		b := []byte("aoo!")
		fileWrite(b, l)
		existWithContent(b, fileName)
		fileCount(fileCountOne, dir)

		patch1 := gomonkey.ApplyFunc(time.Now, func() time.Time {
			time1 := currentTime
			return time1.Add(time.Hour * oneDayHour * sevenDays)
		})
		defer patch1.Reset()

		b2 := []byte("foooooo!")
		fileWrite(b2, l)
		existWithContent(b2, fileName)
		existWithContent(b, getBackupFile(dir, time.Now()))
		fileCount(fileCountTwo, dir)
	})
}

// TestFirstWriteRoll for test the log rolling on first write
func TestFirstWriteRoll(t *testing.T) {
	convey.Convey("TestFirstWriteRoll", t, func() {
		mByte = testMByte
		dir := makeTempDir("TestFirstWriteRoll")
		defer os.RemoveAll(dir)
		currentTime := time.Now()

		fileName := getLogFile(dir)
		l := &Logs{
			FileName: fileName,
			Capacity: testCapacity,
		}
		defer l.Close()

		start := []byte("boooooo!")
		err := ioutil.WriteFile(fileName, start, testFilePermission)
		convey.So(err, convey.ShouldBeNil)
		patch1 := gomonkey.ApplyFunc(time.Now, func() time.Time {
			time1 := currentTime
			return time1.Add(time.Hour * oneDayHour * sevenDays)
		})
		defer patch1.Reset()

		b := []byte("fooo!")
		fileWrite(b, l)
		existWithContent(b, fileName)
		existWithContent(start, getBackupFile(dir, time.Now()))
		fileCount(fileCountTwo, dir)
	})
}

// TestSaveVolumeCase1 for test the deleting log files that exceed the volume
func TestSaveVolumeCase1(t *testing.T) {
	convey.Convey("TestSaveVolumeCase1", t, func() {
		mByte = testMByte
		dir := makeTempDir("TestSaveVolumeCase1")
		defer os.RemoveAll(dir)
		currentTime := time.Now()

		fileName := getLogFile(dir)
		l := &Logs{
			FileName:   fileName,
			Capacity:   testCapacity,
			SaveVolume: testSaveVolume2,
		}
		defer l.Close()

		b := []byte("boo!")
		fileWrite(b, l)
		existWithContent(b, fileName)
		fileCount(fileCountOne, dir)

		patch1 := gomonkey.ApplyFunc(time.Now, func() time.Time {
			time1 := currentTime
			return time1.Add(time.Hour * oneDayHour * sevenDays)
		})
		b2 := []byte("foooooo!")
		fileWrite(b2, l)
		secondFileName := getBackupFile(dir, time.Now())
		existWithContent(b, secondFileName)
		existWithContent(b2, fileName)
		fileCount(fileCountTwo, dir)

		patch1.Reset()
		patch2 := gomonkey.ApplyFunc(time.Now, func() time.Time {
			time2 := currentTime
			return time2.Add(time.Hour * oneDayHour * fourteenDays)
		})
		defer patch2.Reset()
		b3 := []byte("baaaaaar!")
		fileWrite(b3, l)
		thirdFileName := getBackupFile(dir, time.Now())
		existWithContent(b2, thirdFileName)
		existWithContent(b3, fileName)
		<-time.After(time.Millisecond * waitTime)
		fileCount(fileCountTwo, dir)
		existWithContent(b2, thirdFileName)
		convey.So(secondFileName, shouldNotExist)
	})
}

// TestSaveVolumeCase2 for test the deleting log files that exceed the volume when a non-log file exists
func TestSaveVolumeCase2(t *testing.T) {
	convey.Convey("TestSaveVolumeCase2", t, func() {
		mByte = testMByte
		dir := makeTempDir("TestSaveVolumeCase2")
		defer os.RemoveAll(dir)
		currentTime := time.Now()

		fileName := getLogFile(dir)
		l := &Logs{FileName: fileName, Capacity: testCapacity, SaveVolume: testSaveVolume2}
		defer l.Close()

		b := []byte("boo!")
		fileWrite(b, l)
		patch1 := gomonkey.ApplyFunc(time.Now, func() time.Time {
			time1 := currentTime
			return time1.Add(time.Hour * oneDayHour * sevenDays)
		})
		b2 := []byte("baaaaaar!")
		fileWrite(b2, l)
		secondFileName := getBackupFile(dir, time.Now())

		patch1.Reset()
		patch2 := gomonkey.ApplyFunc(time.Now, func() time.Time {
			time2 := currentTime
			return time2.Add(time.Hour * oneDayHour * fourteenDays)
		})
		notLogFile := getLogFile(dir) + ".foo"
		err := ioutil.WriteFile(notLogFile, []byte("data"), testFilePermission)
		convey.So(err, convey.ShouldBeNil)
		notLogFileDir := getBackupFile(dir, time.Now())
		err = os.Mkdir(notLogFileDir, testDirPermission)
		convey.So(err, convey.ShouldBeNil)

		patch2.Reset()
		patch3 := gomonkey.ApplyFunc(time.Now, func() time.Time {
			time3 := currentTime
			return time3.Add(time.Hour * oneDayHour * twentyOneDays)
		})
		defer patch3.Reset()
		thirdFileName := getBackupFile(dir, time.Now())
		b3 := []byte("baaaaaaz!")
		fileWrite(b3, l)
		existWithContent(b2, thirdFileName)
		<-time.After(time.Millisecond * waitTime)
		fileCount(fileCountFour, dir)
		existWithContent(b3, fileName)
		convey.So(secondFileName, shouldNotExist)
		convey.So(notLogFile, shouldExist)
		convey.So(notLogFileDir, shouldExist)
	})
}

// TestCleanupExistingBackupFiles fot test the clearing the current backup log files
func TestCleanupExistingBackupFiles(t *testing.T) {
	convey.Convey("TestCleanupExistingBackupFiles", t, func() {
		mByte = testMByte
		dir := makeTempDir("TestCleanupExistingBackupFiles")
		defer os.RemoveAll(dir)
		currentTime := time.Now()

		data := []byte("data")
		backup := getBackupFile(dir, time.Now())
		err := ioutil.WriteFile(backup, data, testFilePermission)
		convey.So(err, convey.ShouldBeNil)

		patch1 := gomonkey.ApplyFunc(time.Now, func() time.Time {
			time1 := currentTime
			return time1.Add(time.Hour * oneDayHour * sevenDays)
		})
		backup = getBackupFile(dir, time.Now())
		err = ioutil.WriteFile(backup, data, testFilePermission)
		convey.So(err, convey.ShouldBeNil)
		fileName := getLogFile(dir)
		err = ioutil.WriteFile(fileName, data, testFilePermission)
		convey.So(err, convey.ShouldBeNil)

		l := &Logs{
			FileName:   fileName,
			Capacity:   testCapacity,
			SaveVolume: testSaveVolume2,
		}
		defer l.Close()

		patch1.Reset()
		patch2 := gomonkey.ApplyFunc(time.Now, func() time.Time {
			time2 := currentTime
			return time2.Add(time.Hour * oneDayHour * fourteenDays)
		})
		defer patch2.Reset()
		b2 := []byte("foooooo!")
		fileWrite(b2, l)

		<-time.After(time.Millisecond * waitTime)

		fileCount(fileCountTwo, dir)
	})
}

// TestSaveTime for test the deleting log files that exceed the time
func TestSaveTime(t *testing.T) {
	convey.Convey("TestSaveTime", t, func() {
		mByte = testMByte
		dir := makeTempDir("TestSaveTime")
		defer os.RemoveAll(dir)
		currentTime := time.Now()

		fileName := getLogFile(dir)
		l := &Logs{
			FileName: fileName,
			Capacity: testCapacity,
			SaveTime: testSaveTime2,
		}
		defer l.Close()

		patch1 := gomonkey.ApplyFunc(time.Now, func() time.Time {
			time1 := currentTime
			return time1.Add(time.Hour * oneDayHour * sevenDays)
		})
		b := []byte("zoo!")
		fileWrite(b, l)
		existWithContent(b, fileName)
		fileCount(fileCountOne, dir)

		patch1.Reset()
		patch2 := gomonkey.ApplyFunc(time.Now, func() time.Time {
			time2 := currentTime
			return time2.Add(time.Hour * oneDayHour * fourteenDays)
		})
		b2 := []byte("foooooo!")
		fileWrite(b2, l)
		existWithContent(b, getBackupFile(dir, time.Now()))

		<-time.After(waitTime * time.Millisecond)

		fileCount(fileCountTwo, dir)
		existWithContent(b2, fileName)
		existWithContent(b, getBackupFile(dir, time.Now()))

		patch2.Reset()
		patch3 := gomonkey.ApplyFunc(time.Now, func() time.Time {
			time3 := currentTime
			return time3.Add(time.Hour * oneDayHour * twentyOneDays)
		})
		defer patch3.Reset()
		b3 := []byte("baaaaar!")
		fileWrite(b3, l)
		existWithContent(b2, getBackupFile(dir, time.Now()))

		<-time.After(waitTime * time.Millisecond)

		fileCount(fileCountTwo, dir)
		existWithContent(b3, fileName)
		existWithContent(b2, getBackupFile(dir, time.Now()))
	})
}

// TestOldLogFilesList for test the obtaining the list of old log files
func TestOldLogFilesList(t *testing.T) {
	convey.Convey("TestOldLogFilesList", t, func() {
		mByte = testMByte
		dir := makeTempDir("TestOldLogFiles")
		defer os.RemoveAll(dir)
		currentTime := time.Now()

		fileName := getLogFile(dir)
		data := []byte("data")
		err := ioutil.WriteFile(fileName, data, testDirPermission)
		convey.So(err, convey.ShouldBeNil)
		t1, err := time.Parse(timeFormat, currentTime.UTC().Format(timeFormat))
		convey.So(err, convey.ShouldBeNil)
		backup := getBackupFile(dir, currentTime)
		err = ioutil.WriteFile(backup, data, testDirPermission)
		convey.So(err, convey.ShouldBeNil)

		patch := gomonkey.ApplyFunc(time.Now, func() time.Time {
			time1 := currentTime
			return time1.Add(time.Hour * oneDayHour * sevenDays)
		})
		defer patch.Reset()
		t2, err := time.Parse(timeFormat, time.Now().UTC().Format(timeFormat))
		convey.So(err, convey.ShouldBeNil)
		backup2 := getBackupFile(dir, time.Now())
		err = ioutil.WriteFile(backup2, data, testDirPermission)
		convey.So(err, convey.ShouldBeNil)

		l := &Logs{FileName: fileName}
		files, err := l.oldFilesList()
		convey.So(err, convey.ShouldBeNil)
		convey.So(fileCountTwo, convey.ShouldEqual, len(files))
		convey.So(t2, convey.ShouldEqual, files[0].timeStamp)
		convey.So(t1, convey.ShouldEqual, files[1].timeStamp)
	})
}

// TestExtractTime for test obtaining log file timestamp
func TestExtractTime(t *testing.T) {
	convey.Convey("TestExtractTime", t, func() {
		l := &Logs{FileName: "/var/log/myfoo/foo.log"}
		prefix, extention := l.getPreAndExt()

		tests := []struct {
			fileName string
			want     time.Time
			wantErr  bool
		}{
			{"foo-2014-05-04T14-44-33.555.log", time.Date(
				testYear, testMonth, testDay, testHour, testMin, testSec, testNsec, time.UTC), false},
			{"foo-2014-05-04T14-44-33.555", time.Time{}, true},
			{"2014-05-04T14-44-33.555.log", time.Time{}, true},
			{"foo.log", time.Time{}, true},
		}

		for _, test := range tests {
			got, err := l.extractTime(test.fileName, prefix, extention)
			convey.So(got, convey.ShouldEqual, test.want)
			convey.So(err != nil, convey.ShouldEqual, test.wantErr)
		}
	})
}

// TestLocalTime for test the situation that current time is the local time
func TestLocalTime(t *testing.T) {
	convey.Convey("TestLocalTime", t, func() {
		mByte = testMByte
		dir := makeTempDir("TestLocalTime")
		defer os.RemoveAll(dir)
		currentTime := time.Now()

		l := &Logs{
			FileName:   getLogFile(dir),
			Capacity:   testCapacity,
			LocalOrUTC: true,
		}
		defer l.Close()

		patch := gomonkey.ApplyFunc(time.Now, func() time.Time {
			return currentTime
		})
		defer patch.Reset()
		b := []byte("boo!")
		fileWrite(b, l)

		b2 := []byte("fooooooo!")
		fileWrite(b2, l)
		existWithContent(b2, getLogFile(dir))
		existWithContent(b, getBackupFileLocal(dir, currentTime))
	})
}

// TestRoll for test rolling
func TestRoll(t *testing.T) {
	convey.Convey("TestRoll", t, func() {
		dir := makeTempDir("TestRotate")
		defer os.RemoveAll(dir)
		currentTime := time.Now()

		fileName := getLogFile(dir)
		l := &Logs{
			FileName:   fileName,
			SaveVolume: testSaveVolume2,
			Capacity:   testCapacity2, // megabytes
		}
		defer l.Close()

		patch1 := gomonkey.ApplyFunc(time.Now, func() time.Time {
			time1 := currentTime
			return time1.Add(time.Hour * oneDayHour * sevenDays)
		})
		b := []byte("boo!")
		fileWrite(b, l)
		existWithContent(b, fileName)
		fileCount(fileCountOne, dir)

		patch1.Reset()
		patch2 := gomonkey.ApplyFunc(time.Now, func() time.Time {
			time2 := currentTime
			return time2.Add(time.Hour * oneDayHour * fourteenDays)
		})
		err := l.Roll()
		convey.So(err, convey.ShouldBeNil)

		<-time.After(waitTime * time.Millisecond)

		filename2 := getBackupFile(dir, time.Now())
		existWithContent(b, filename2)
		existWithContent([]byte{}, fileName)
		fileCount(fileCountTwo, dir)

		patch2.Reset()
		patch3 := gomonkey.ApplyFunc(time.Now, func() time.Time {
			time3 := currentTime
			return time3.Add(time.Hour * oneDayHour * twentyOneDays)
		})
		defer patch3.Reset()
		err = l.Roll()
		convey.So(err, convey.ShouldBeNil)

		<-time.After(waitTime * time.Millisecond)

		filename3 := getBackupFile(dir, time.Now())
		existWithContent([]byte{}, filename3)
		existWithContent([]byte{}, fileName)
		fileCount(fileCountTwo, dir)

		b2 := []byte("foooooo!")
		fileWrite(b2, l)
		existWithContent(b2, fileName)
	})
}

// TestJson for test JSON conversion
func TestJson(t *testing.T) {
	convey.Convey("TestJson", t, func() {
		data := []byte(`
		{
			"filename": "foo",
			"capacity": 10,
			"savetime": 10,
			"savevolume": 3,
			"localorutc": true
		}`[1:])

		l := Logs{}
		err := json.Unmarshal(data, &l)
		convey.So(err, convey.ShouldBeNil)
		convey.So("foo", convey.ShouldEqual, l.FileName)
		convey.So(testCapacity, convey.ShouldEqual, l.Capacity)
		convey.So(testSaveTime, convey.ShouldEqual, l.SaveTime)
		convey.So(testSaveVolume, convey.ShouldEqual, l.SaveVolume)
		convey.So(true, convey.ShouldEqual, l.LocalOrUTC)
	})
}

// makeTempDir creates a file in the OS temp directory to keep parallel test
func makeTempDir(name string) string {
	dir := time.Now().Format(name + timeFormat)
	dir = filepath.Join(os.TempDir(), dir)
	err := os.Mkdir(dir, testDirPermission)
	convey.So(err, convey.ShouldBeNil)
	return dir
}

// existWithContent checks that the given file exists and has the correct content
func existWithContent(content []byte, dir string) {
	info, err := os.Stat(dir)
	convey.So(err, convey.ShouldBeNil)
	convey.So(int64(len(content)), convey.ShouldEqual, info.Size())

	b, err := ioutil.ReadFile(dir)
	convey.So(err, convey.ShouldBeNil)
	convey.So(content, convey.ShouldResemble, b)
}

// getLogFile returns the log file name in the given directory for the current fake time
func getLogFile(dir string) string {
	return filepath.Join(dir, "foobar.log")
}

func getBackupFile(dir string, t time.Time) string {
	return filepath.Join(dir, "foobar-"+t.UTC().Format(timeFormat)+".log")
}

func getBackupFileLocal(dir string, t time.Time) string {
	return filepath.Join(dir, "foobar-"+t.Format(timeFormat)+".log")
}

// fileCount checks that the number of files in the directory is exp.
func fileCount(exp int, dir string) {
	files, err := ioutil.ReadDir(dir)
	convey.So(err, convey.ShouldBeNil)
	convey.So(len(files), convey.ShouldEqual, exp)
}

func fileWrite(b []byte, l *Logs) {
	n, err := l.Write(b)
	convey.So(err, convey.ShouldBeNil)
	convey.So(len(b), convey.ShouldEqual, n)
}

func shouldNotBeExist(actual interface{}, expected ...interface{}) string {
	err, ok := actual.(error)
	if !ok {
		return "incorrect parameter type"
	}
	if os.IsNotExist(err) {
		return ""
	}
	return "File exists, but should not have been created"
}
func shouldNotExist(actual interface{}, expected ...interface{}) string {
	path, ok := actual.(string)
	if !ok {
		return "incorrect parameter type"
	}
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return ""
	}
	return fmt.Sprintf("expected to get os.IsNotExist, but instead got %v", err)
}

func shouldExist(actual interface{}, expected ...interface{}) string {
	path, ok := actual.(string)
	if !ok {
		return "incorrect parameter type"
	}
	_, err := os.Stat(path)
	if err != nil {
		return fmt.Sprintf("expected file to exist, but got error from os.Stat: %v", err)
	}
	return ""
}
