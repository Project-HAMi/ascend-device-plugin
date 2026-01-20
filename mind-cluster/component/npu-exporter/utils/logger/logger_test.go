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
	"errors"
	"testing"

	"ascend-common/common-utils/hwlog"
)

// TestInitLogger tests the InitLogger function
func TestInitLogger(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		expected error
	}{
		{
			name:     "Telegraf Platform",
			platform: TelegrafPlatform,
			expected: nil,
		},
		{
			name:     "Prometheus Platform",
			platform: PrometheusPlatform,
			expected: nil,
		},
		{
			name:     "Unsupported Platform",
			platform: "Unsupported",
			expected: errors.New("platform is not supported:Unsupported"),
		},
	}

	HwLogConfig.LogLevel = 0
	HwLogConfig.MaxBackups = hwlog.DefaultMaxBackups
	HwLogConfig.LogFileName = defaultLogFile
	HwLogConfig.MaxAge = hwlog.DefaultMinSaveAge

	var noExistLevel Level = 5
	var args = "mock"
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := InitLogger(tt.platform)
			if tt.expected == nil && err != nil {
				t.Errorf("InitLogger(%s) = %v, want %v", tt.platform, err, tt.expected)
			} else if tt.expected != nil && err.Error() != tt.expected.Error() {
				t.Errorf("InitLogger(%s) = %v, want %v", tt.platform, err, tt.expected)
			}

			logger.log(nil, DebugLevel, args)
			logger.log(nil, InfoLevel, args)
			logger.log(nil, WarnLevel, args)
			logger.log(nil, noExistLevel, args)
			logger.logfWithOptions(nil, DebugLevel, LogOptions{}, "test logf with options %s", "arg")

			logger.logf(nil, DebugLevel, args)
			logger.logf(nil, InfoLevel, args)
			logger.logf(nil, WarnLevel, args)
			logger.logf(nil, noExistLevel, args)
			logger.logfWithOptions(nil, DebugLevel, LogOptions{}, "test logf with options %s", "arg")

		})
	}
}

func TestLoggerMethods(t *testing.T) {

	tests := []struct {
		name   string
		method func(...interface{})
		level  Level
		args   []interface{}
	}{
		{"test Debug", Debug, DebugLevel, []interface{}{"debug message"}},
		{"test Info", Info, InfoLevel, []interface{}{"info message"}},
		{"test Warn", Warn, WarnLevel, []interface{}{"warn message"}},
		{"test Error", Error, ErrorLevel, []interface{}{"error message"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.method(test.args...)
		})
	}

	testsF := []struct {
		name   string
		method func(string, ...interface{})
		level  Level
		format string
		args   []interface{}
	}{
		{"test Debugf", Debugf, DebugLevel, "debug message %d", []interface{}{1}},
		{"test Infof", Infof, InfoLevel, "info message %d", []interface{}{1}},
		{"test Warnf", Warnf, WarnLevel, "warn message %d", []interface{}{1}},
		{"test Errorf", Errorf, ErrorLevel, "error message %d", []interface{}{1}},
	}

	for _, test := range testsF {
		t.Run(test.name, func(t *testing.T) {
			test.method(test.format, test.args...)
		})
	}
}
