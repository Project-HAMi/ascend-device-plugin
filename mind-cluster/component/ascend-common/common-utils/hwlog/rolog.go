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
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	oneDaySeconds           = 24 * 60 * 60
	defaultCapacity         = 20
	timeFormat              = "2006-01-02T15-04-05.000"
	kilobytes               = 1024
	defaultDirPermission    = 0750
	defaultFilePermission   = 0600
	defaultBackupPermission = 0400
	maxCapacity             = 20
	minSaveVolume           = 1
	maxSaveVolume           = 30
	maxSaveTime             = 700
	minSaveTime             = 7
)

// Logs is an io.WriteCloser.
type Logs struct {
	file   *os.File
	mutex  sync.Mutex
	rmOnce sync.Once

	// FileName is the file where logs are written.
	FileName string `json:"filename" yaml:"filename"`

	// Capacity is the maximum number of bytes before the log file
	// is rotated, and the default value is 128 megabytes.
	Capacity int `json:"capacity" yaml:"capacity"`

	// SaveTime is the maximum number of days for retaining old log
	// files. It calculates the retention time based on the timestamp
	// of the old log file name and the current time.
	SaveTime int `json:"savetime" yaml:"savetime"`

	// SaveVolume is the maximum number of old log files that can be
	// retained. It saves all old files by default.
	SaveVolume int `json:"savevolume" yaml:"savevolume"`

	// UTC determines whether to use the local time of the computer
	// or the UTC time as the timestamp in the formatted backup file.
	LocalOrUTC bool `json:"localorutc" yaml:"localorutc"`

	length int64
	rmCh   chan bool
}

// logFile is a struct that is used to return filename and
// timestamp.
type logFile struct {
	fileInfo  os.FileInfo
	timeStamp time.Time
}

var (
	// mByte is used to convert capacity into bytes.
	mByte = kilobytes * kilobytes
)

// Write implements io.Writer. If a write would not cause the size of
// the log file to exceed Capacity, the log file is written normally.
// If a write would cause the size of the log file to exceed Capacity,
// but the write length is less than Capacity, the log file is closed,
// renamed to include a timestamp of the current time, and a new log
// is created using the original log file name. If the length of a write
// is greater than the Capacity, an error is returned.
func (l *Logs) Write(d []byte) (int, error) {
	if l == nil {
		return 0, fmt.Errorf("logs pointer does not exist")
	}

	l.mutex.Lock()
	defer l.mutex.Unlock()

	writeLenth := int64(len(d))
	if writeLenth > l.maxLenth() {
		return 0, fmt.Errorf("the write lenth %d is greater than the maximum file size %d",
			writeLenth, l.maxLenth(),
		)
	}

	if l.file == nil {
		if err := l.openOrCreateFile(writeLenth); err != nil {
			return 0, err
		}
	}
	fileInfo, err := l.file.Stat()
	if err != nil {
		return 0, err
	}
	l.length = fileInfo.Size()
	if writeLenth+l.length > l.maxLenth() {
		if err := l.roll(); err != nil {
			return 0, err
		}
	}

	n, err := l.file.Write(d)
	if err != nil {
		return 0, err
	}
	l.length += int64(n)
	return n, err
}

// Roll causes Logs to close the existing log file and create a new log
// file immediately. The purpose of this function is to provide rotation
// outside the normal rotation rule, e.g. in response to SIGHUP. After
// rotation, the deletion of the old log files is initiated.
func (l *Logs) Roll() error {
	if l == nil {
		return fmt.Errorf("logs pointer does not exist")
	}

	l.mutex.Lock()
	defer l.mutex.Unlock()
	return l.roll()
}

// Close implements io.Closer. It closses the current log file.
func (l *Logs) Close() error {
	if l == nil {
		return fmt.Errorf("logs pointer does not exist")
	}

	l.mutex.Lock()
	defer l.mutex.Unlock()

	return l.close()
}

// Flush persist the contents of the current memory.
func (l *Logs) Flush() error {
	if l == nil {
		return fmt.Errorf("logs pointer does not exist")
	}

	l.mutex.Lock()
	defer l.mutex.Unlock()
	if l.file == nil {
		return nil
	}
	return l.file.Sync()
}

// maxLenth return the number of bytes of the maximum log size
// before rotating.
func (l *Logs) maxLenth() int64 {
	if l.Capacity > 0 && l.Capacity < maxCapacity {
		return int64(l.Capacity) * int64(mByte)
	}
	return int64(defaultCapacity * mByte)
}

// fileName return the name of the log file.
func (l *Logs) fileName() string {
	if l.FileName != "" {
		return l.FileName
	}
	logName := filepath.Base(os.Args[0]) + "-mindx-dl.log"
	return filepath.Join(os.TempDir(), logName)
}

// openOrCreateFile opens the log file if it exists and the
// current write would not exceed the Capacity. It will create
// a new file if there is no such file or the write would exceed
// the Capacity.
func (l *Logs) openOrCreateFile(writeLen int64) error {
	l.remove()

	name := l.fileName()
	message, err := os.Stat(name)
	if os.IsNotExist(err) {
		return l.create()
	}

	if err != nil {
		return fmt.Errorf("failed to get log file message: %v", err)
	}

	if writeLen+message.Size() >= l.maxLenth() {
		return l.roll()
	}

	f, err := os.OpenFile(name, os.O_APPEND|os.O_WRONLY, defaultFilePermission)
	if err != nil {
		return l.create()
	}
	l.file = f
	l.length = message.Size()
	return nil
}

// create creates a new log file for writing, and backs up the
// old log file. The file is closed when this method is invoked
// by default.
func (l *Logs) create() error {
	if err := os.MkdirAll(l.getDir(), defaultDirPermission); err != nil {
		return fmt.Errorf("unable to create directory for new log file: %v", err)
	}

	fileName, fileMode := l.fileName(), os.FileMode(defaultFilePermission)
	if message, err := os.Stat(fileName); err == nil {
		fileMode = message.Mode()
		backupName := l.backup()
		if err := os.Rename(fileName, backupName); err != nil {
			return fmt.Errorf("failed to rename the log file: %v", err)
		}
		if err := os.Chmod(backupName, defaultBackupPermission); err != nil {
			return fmt.Errorf("failed to change backup log file permission: %v", err)
		}
	}
	newFile, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fileMode)
	if err != nil {
		return fmt.Errorf("unable to open new log file: %v", err)
	}
	l.length, l.file = 0, newFile
	return nil
}

// backup generates a backup file name based on the original file
// name and inserts a timestamp between the file name and extension.
// The timestamp uses the UTC time by default.
func (l *Logs) backup() string {
	prefix, extension := l.getPreAndExt()
	return filepath.Join(l.getDir(), fmt.Sprintf("%s%s%s", prefix, l.getTimestamp(), extension))
}

// getDir returns the directory for the current filename.
func (l *Logs) getDir() string {
	return filepath.Dir(l.fileName())
}

// getPreAndExt returns the prefix name and extension name
// from Logs's filename.
func (l *Logs) getPreAndExt() (string, string) {
	name := filepath.Base(l.fileName())
	extension := filepath.Ext(name)
	prefix := name[:len(name)-len(extension)] + "-"
	return prefix, extension
}

// getTimestamp returns the timestamp of current time, and
// uses UTC time by default.
func (l *Logs) getTimestamp() string {
	t := time.Now()
	if !l.LocalOrUTC {
		t = t.UTC()
	}
	return t.Format(timeFormat)
}

// roll rotates the log file, close the existing log file and
// create a new one immediately. After rotating, this method
// deletes the old log files according to the configuration.
func (l *Logs) roll() error {
	if err := l.close(); err != nil {
		return err
	}
	if err := l.create(); err != nil {
		return err
	}
	l.remove()
	return nil
}

// close closes the file if it is open.
func (l *Logs) close() error {
	if l.file == nil {
		return nil
	}
	err := l.file.Sync()
	if err != nil {
		return err
	}
	err = l.file.Close()
	l.file = nil
	return err
}

// remove delete outdated log files, starting the remove
// goroutine if necessary.
func (l *Logs) remove() {
	l.rmOnce.Do(func() {
		l.rmCh = make(chan bool, 1)
		go l.removeRun()
	})
	select {
	case l.rmCh <- true:
	default:
	}
}

// removeRun manages the deletion of the old log files after
// rotating, which runs in a goroutine.
func (l *Logs) removeRun() {
	for range l.rmCh {
		if err := l.removeRunOnce(); err != nil {
			fmt.Println("failed to remove runonce: ", err)
		}
	}
}

// removeRunOnce performs removal of outdated log files.
// Old log files are removed if the number of old files
// exceed the Capacity or the retention time of old files
// is greater than SaveTime.
func (l *Logs) removeRunOnce() error {
	if l.SaveVolume == 0 && l.SaveTime == 0 {
		return nil
	}

	if err := checkParam(l.SaveVolume, l.SaveTime); err != nil {
		return err
	}

	oldFiles, err := l.oldFilesList()
	if err != nil {
		return err
	}

	var removeFiles []logFile
	if l.SaveTime > 0 {
		delTime := time.Now().Unix() - int64(l.SaveTime)*oneDaySeconds
		var remainingFiles []logFile
		for _, f := range oldFiles {
			if f.timeStamp.Unix() <= delTime {
				removeFiles = append(removeFiles, f)
				continue
			}
			remainingFiles = append(remainingFiles, f)
		}
		oldFiles = remainingFiles
	}

	if l.SaveVolume > 0 && l.SaveVolume < len(oldFiles) {
		saved := make(map[string]struct{}, len(oldFiles))
		var remainingFiles []logFile
		for _, f := range oldFiles {
			saved[f.fileInfo.Name()] = struct{}{}
			if l.SaveVolume >= len(saved) {
				remainingFiles = append(remainingFiles, f)
				continue
			}
			removeFiles = append(removeFiles, f)
		}
		oldFiles = remainingFiles
	}

	for _, f := range removeFiles {
		rmError := os.Remove(filepath.Join(l.getDir(), f.fileInfo.Name()))
		if rmError != nil {
			err = rmError
		}
	}
	return err
}

// oldFilesList returns the list of backup log files sorted
// by ModTime. These backup log files are stored in the same
// directory as the current log file.
func (l *Logs) oldFilesList() ([]logFile, error) {
	logFiles, err := ioutil.ReadDir(l.getDir())
	if err != nil {
		return nil, fmt.Errorf("unable to open the log file directory: %v", err)
	}

	prefix, extension := l.getPreAndExt()

	var oldFiles []logFile

	for _, file := range logFiles {
		if file.IsDir() {
			continue
		}
		if timeStamp, err := l.extractTime(file.Name(), prefix, extension); err == nil {
			oldFiles = append(oldFiles, logFile{fileInfo: file, timeStamp: timeStamp})
			continue
		}
	}
	sort.Slice(oldFiles, func(i, j int) bool {
		if i < 0 || i > len(oldFiles) || j < 0 || j > len(oldFiles) {
			return false
		}
		return oldFiles[i].timeStamp.After(oldFiles[j].timeStamp)
	})

	return oldFiles, nil
}

// extractTime extracts the formatted time from file name by
// stripping the prefix and extension of the file name. This
// prevents fileName from being confused with time.parse.
func (l *Logs) extractTime(name, prefix, extension string) (time.Time, error) {
	if !strings.HasSuffix(name, extension) {
		return time.Time{}, errors.New("unmatched extension")
	}

	if !strings.HasPrefix(name, prefix) {
		return time.Time{}, errors.New("unmatched prefix")
	}

	timeStamp := name[len(prefix) : len(name)-len(extension)]
	return time.Parse(timeFormat, timeStamp)
}

// checkParam checks whether the parameters are correct
func checkParam(volume int, time int) error {
	if volume != 0 {
		if volume < minSaveVolume || volume > maxSaveVolume {
			return fmt.Errorf("the value of savevolume is incorrect")
		}
	}
	if time != 0 {
		if time < minSaveTime || time > maxSaveTime {
			return fmt.Errorf("the value of savetime is incorrect")
		}
	}
	return nil
}
