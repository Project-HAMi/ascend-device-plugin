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
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/fsnotify/fsnotify"

	"ascend-common/common-utils/utils"
)

const (
	// DefaultFileMaxSize  the default maximum size of a single log file is 20 MB
	DefaultFileMaxSize = 20
	// DefaultMinSaveAge the minimum storage duration of backup logs is 7 days
	DefaultMinSaveAge = 7
	// DefaultMaxSaveAge the maximum storage duration of backup logs is 700 days
	DefaultMaxSaveAge = 700
	// DefaultMaxBackups the default number of backup log
	DefaultMaxBackups = 30
	// LogFileMode log file mode
	LogFileMode os.FileMode = 0640
	// BackupLogFileMode backup log file mode
	BackupLogFileMode os.FileMode = 0400
	// LogDirMode log dir mode
	LogDirMode            = 0750
	backUpLogRegex        = `^.+-[0-9]{4}-[0-9]{2}-[0-9T]{5}-[0-9]{2}-[0-9]{2}\.[0-9]{2,4}`
	bitsize               = 64
	stackDeep             = 3
	pathLen               = 2
	minLogLevel           = -1
	maxLogLevel           = 3
	maxEachLineLen        = 1048576
	defaultMaxEachLineLen = 256
)

// LogConfig log module config
type LogConfig struct {
	// log file path
	LogFileName string
	// only write to std out, default value: false
	OnlyToStdout bool
	// only write to file, default value: false
	OnlyToFile bool
	// log level, -1-debug, 0-info, 1-warning, 2-error 3-critical default value: 0
	LogLevel int
	// size of a single log file (MB), default value: 20MB
	FileMaxSize int
	// MaxLineLength Max length of each log line, default value: 256
	MaxLineLength int
	// maximum number of backup log files, default value: 30
	MaxBackups int
	// maximum number of days for backup log files, default value: 7
	MaxAge int
	// whether backup files need to be compressed, default value: false
	IsCompress bool
	// expiration time for log cache, default value: 1s
	ExpiredTime int
	// Size of log cache space, default: 10240
	CacheSize int
}

var reg = regexp.MustCompile(backUpLogRegex)

type validateFunc func(config *LogConfig) error

func checkDir(fileDir string) error {
	if !utils.IsExist(fileDir) {
		if err := os.MkdirAll(fileDir, LogDirMode); err != nil {
			return fmt.Errorf("create dirs failed")
		}
		return nil
	}
	if err := os.Chmod(fileDir, LogDirMode); err != nil {
		return fmt.Errorf("change log dir mode failed")
	}
	return nil
}

func createFile(filePath string) error {
	fileName := path.Base(filePath)
	if !utils.IsExist(filePath) {
		f, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, LogFileMode)
		if err != nil {
			return fmt.Errorf("create file(%s) failed", fileName)
		}
		defer func() {
			if err := f.Close(); err != nil {
				fmt.Printf("close file failed: %v\n", err)
				return
			}
		}()
	}
	return nil
}

func checkAndCreateLogFile(filePath string) error {
	if !utils.IsFile(filePath) {
		return fmt.Errorf("config path is not file")
	}
	fileDir := path.Dir(filePath)
	if err := checkDir(fileDir); err != nil {
		return err
	}
	if err := createFile(filePath); err != nil {
		return err
	}
	return nil
}

func validateLogConfigFileMaxSize(config *LogConfig) error {
	if config.FileMaxSize == 0 {
		config.FileMaxSize = DefaultFileMaxSize
		return nil
	}
	if config.FileMaxSize < 0 || config.FileMaxSize > DefaultFileMaxSize {
		return fmt.Errorf("the size of a single log file range is (0, 20] MB")
	}

	return nil
}

func validateLogConfigBackups(config *LogConfig) error {
	if config.MaxBackups <= 0 || config.MaxBackups > DefaultMaxBackups {
		return fmt.Errorf("the number of backup log file range is (0, 30]")
	}
	return nil
}

func validateLogConfigMaxAge(config *LogConfig) error {
	fmt.Printf("MaxAge %s", config.MaxAge)
	if config.MaxAge < DefaultMinSaveAge || config.MaxAge > DefaultMaxSaveAge {
		return fmt.Errorf("the maxage of backup logs range is [7,700]")
	}
	return nil
}

func validateLogLevel(config *LogConfig) error {
	if config.LogLevel < minLogLevel || config.LogLevel > maxLogLevel {
		return fmt.Errorf("the log level range should be [-1, 3]")
	}
	return nil
}

func validateMaxLineLength(config *LogConfig) error {
	if config.MaxLineLength == 0 {
		config.MaxLineLength = defaultMaxEachLineLen
		return nil
	}
	if config.MaxLineLength < 0 || config.MaxLineLength > maxEachLineLen {
		return fmt.Errorf("the max length of each log line should be in the range (0, 1048576]")
	}
	return nil
}

func getValidateFuncList() []validateFunc {
	var funcList []validateFunc
	funcList = append(funcList, validateLogConfigFileMaxSize, validateLogConfigBackups, validateMaxLineLength,
		validateLogConfigMaxAge, validateLogLevel, validateLogConfigLimiter)
	return funcList
}

func validateLogConfigFiled(config *LogConfig) error {
	if config.OnlyToStdout {
		return nil
	}
	if _, err := utils.CheckPath(config.LogFileName); err != nil && err != os.ErrNotExist {
		return fmt.Errorf("config log path is not absolute path: %v", err)
	}
	if strings.Contains(config.LogFileName, "..") || strings.Contains(config.LogFileName, "./") {
		return errors.New("log path include invalid char")
	}

	if err := checkAndCreateLogFile(config.LogFileName); err != nil {
		return err
	}
	validateFuncList := getValidateFuncList()
	for _, vaFunc := range validateFuncList {
		if err := vaFunc(config); err != nil {
			return err
		}
	}

	return nil
}

func validateLogConfigLimiter(config *LogConfig) error {
	if config.ExpiredTime < 0 || config.ExpiredTime > MaxExpiredTime {
		return fmt.Errorf("the expired time of log cache range is [0, 3600], the value 0 disables the limiter")
	}
	if config.CacheSize < 0 || config.CacheSize > MaxCacheSize {
		return fmt.Errorf("the size of log cache range is [0, 102400], the value 0 disables the limiter")
	}
	return nil
}

func changeFileMode(l *logger, event fsnotify.Event, logFileFullPath string) {
	if l == nil {
		fmt.Println("changeFileMode logger is nil")
		return
	}
	var logMode = LogFileMode
	logPath := path.Dir(logFileFullPath)
	changedFileName := path.Base(event.Name)
	if isTargetLog(changedFileName) {
		logMode = BackupLogFileMode
	}
	changedLogFilePath := path.Join(logPath, changedFileName)
	if !utils.IsExist(changedLogFilePath) {
		return
	}
	fPath, err := utils.CheckPath(changedLogFilePath)
	if err != nil {
		l.Errorf("wrong file path: %v", err)
		return
	}
	if errChmod := os.Chmod(fPath, logMode); errChmod != nil {
		l.Errorf("set file mode failed, filename: %s", changedFileName)
	}
}
func isTargetLog(fileName string) bool {
	return reg.MatchString(fileName)
}
