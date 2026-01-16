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

// Package logger for general collector
package logger

import (
	"context"
	"errors"
	"fmt"

	"github.com/influxdata/telegraf"

	"ascend-common/common-utils/hwlog"
)

// the method mapping table (avoid rebuilding with every call)
var (
	logFuncs  = map[Level]logFunc{}
	logfFuncs = map[Level]logfFunc{}
)

const (
	// DebugLevel Debug level
	DebugLevel Level = iota - 1
	// InfoLevel Info level
	InfoLevel
	// WarnLevel Warn level
	WarnLevel
	// ErrorLevel Error level
	ErrorLevel

	// PrometheusPlatform Prometheus platform
	PrometheusPlatform = "Prometheus"
	// TelegrafPlatform Telegraf platform
	TelegrafPlatform = "Telegraf"
)

// HwLogConfig default log file
var HwLogConfig = &hwlog.LogConfig{
	LogFileName:   defaultLogFile,
	ExpiredTime:   hwlog.DefaultExpiredTime,
	CacheSize:     hwlog.DefaultCacheSize,
	MaxLineLength: maxLogLineLength,
}

// Level log level
type Level int

// logFunc log function
type logFunc func(ctx context.Context, args ...interface{})

// logfFunc logf function
type logfFunc func(ctx context.Context, format string, args ...interface{})

var (
	// logger Unified log printer
	logger UnifiedLogger
)

// InitLogger initialize the log manager
func InitLogger(platform string) error {

	if platform == TelegrafPlatform {
		logger = &telegrafLogger{}
		HwLogConfig.LogFileName = defaultTelegrafLogPath
		HwLogConfig.OnlyToFile = true
	} else if platform == PrometheusPlatform {
		logger = &generalLogger{}
	} else {
		return errors.New("platform is not supported:" + platform)
	}

	if err := hwlog.InitRunLogger(HwLogConfig, context.Background()); err != nil {
		fmt.Printf("hwlog init failed, error is %v\n", err)
		return err
	}

	logFuncs = map[Level]logFunc{
		DebugLevel: hwlog.RunLog.DebugWithCtx,
		InfoLevel:  hwlog.RunLog.InfoWithCtx,
		WarnLevel:  hwlog.RunLog.WarnWithCtx,
		ErrorLevel: hwlog.RunLog.ErrorWithCtx,
	}

	logfFuncs = map[Level]logfFunc{
		DebugLevel: hwlog.RunLog.DebugfWithCtx,
		InfoLevel:  hwlog.RunLog.InfofWithCtx,
		WarnLevel:  hwlog.RunLog.WarnfWithCtx,
		ErrorLevel: hwlog.RunLog.ErrorfWithCtx,
	}
	return nil
}

// LogOptions options for log
type LogOptions struct {
	Domain    string
	ID        interface{}
	MaxCounts int
}

// Config config for telegraf
type Config struct {
	Acc telegraf.Accumulator
}

// UnifiedLogger unified logger interface
type UnifiedLogger interface {
	dynamicConfigure(Config)
	log(ctx context.Context, level Level, args ...interface{})
	logf(ctx context.Context, level Level, format string, args ...interface{})
	logfWithOptions(ctx context.Context, level Level, opts LogOptions, format string, args ...interface{})
}

// Debug print log info with debug level
func Debug(args ...interface{}) {
	logger.log(hwlog.DeepIncrease(context.Background()), DebugLevel, args...)
}

// Info print log info with info level
func Info(args ...interface{}) {
	logger.log(hwlog.DeepIncrease(context.Background()), InfoLevel, args...)
}

// Warn print log info with warn level
func Warn(args ...interface{}) {
	logger.log(hwlog.DeepIncrease(context.Background()), WarnLevel, args...)
}

// Error print log info with error level
func Error(args ...interface{}) {
	logger.log(hwlog.DeepIncrease(context.Background()), ErrorLevel, args...)
}

// Debugf print log info with debug level
func Debugf(format string, args ...interface{}) {
	logger.logf(hwlog.DeepIncrease(context.Background()), DebugLevel, format, args...)
}

// Infof print log info with info level
func Infof(format string, args ...interface{}) {
	logger.logf(hwlog.DeepIncrease(context.Background()), InfoLevel, format, args...)
}

// Warnf print log info with warn level
func Warnf(format string, args ...interface{}) {
	logger.logf(hwlog.DeepIncrease(context.Background()), WarnLevel, format, args...)
}

// Errorf print log info with error level
func Errorf(format string, args ...interface{}) {
	logger.logf(hwlog.DeepIncrease(context.Background()), ErrorLevel, format, args...)
}

// LogfWithOptions print log info with error level
func LogfWithOptions(level Level, opts LogOptions, format string, args ...interface{}) {
	logger.logfWithOptions(hwlog.DeepIncrease(context.Background()), level, opts, format, args...)
}

// DynamicConfigure configure the logger
func DynamicConfigure(config Config) {
	logger.dynamicConfigure(config)
}
