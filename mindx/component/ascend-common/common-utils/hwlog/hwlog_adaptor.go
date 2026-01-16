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
	"context"
	"errors"
)

// RunLog run logger
var RunLog *logger

// InitRunLogger initialize run logger
func InitRunLogger(config *LogConfig, ctx context.Context) error {
	if config == nil {
		return errors.New("run logger config is nil")
	}
	if RunLog != nil && RunLog.isInit() {
		RunLog.Warn("run logger is been initialized")
		return nil
	}
	RunLog = new(logger)
	if RunLog == nil {
		return errors.New("malloc new logger flied")
	}
	if err := RunLog.setLogger(config); err != nil {
		return err
	}
	if !RunLog.isInit() {
		return errors.New("run logger init failed")
	}
	return nil
}

// OpLog operate logger
var OpLog *logger

// InitOperateLogger initialize operate logger
func InitOperateLogger(config *LogConfig, ctx context.Context) error {
	if config == nil {
		return errors.New("operate logger config is nil")
	}
	if OpLog != nil && OpLog.isInit() {
		OpLog.Warn("operate logger is been initialized")
		return nil
	}
	OpLog = new(logger)
	if OpLog == nil {
		return errors.New("malloc new logger flied")
	}
	if err := OpLog.setLogger(config); err != nil {
		return err
	}
	if !OpLog.isInit() {
		return errors.New("operate logger init failed")
	}
	return nil
}

// SecLog security logger
var SecLog *logger

// InitSecurityLogger initialize security logger
func InitSecurityLogger(config *LogConfig, ctx context.Context) error {
	if config == nil {
		return errors.New("security logger config is nil")
	}
	if SecLog != nil && SecLog.isInit() {
		SecLog.Warn("security logger is been initialized")
		return nil
	}
	SecLog = new(logger)
	if SecLog == nil {
		return errors.New("malloc new logger flied")
	}
	if err := SecLog.setLogger(config); err != nil {
		return err
	}
	if !SecLog.isInit() {
		return errors.New("security logger init failed")
	}
	return nil
}

// UserLog user logger
var UserLog *logger

// InitUserLogger initialize user logger
func InitUserLogger(config *LogConfig, ctx context.Context) error {
	if config == nil {
		return errors.New("user logger config is nil")
	}
	if UserLog != nil && UserLog.isInit() {
		UserLog.Warn("user logger is been initialized")
		return nil
	}
	UserLog = new(logger)
	if UserLog == nil {
		return errors.New("malloc new logger flied")
	}
	if err := UserLog.setLogger(config); err != nil {
		return err
	}
	if !UserLog.isInit() {
		return errors.New("user logger init failed")
	}
	return nil
}

// DebugLog debug logger
var DebugLog *logger

// InitDebugLogger initialize debug logger
func InitDebugLogger(config *LogConfig, ctx context.Context) error {
	if config == nil {
		return errors.New("debug logger config is nil")
	}
	if DebugLog != nil && DebugLog.isInit() {
		DebugLog.Warn("debug logger is been initialized")
		return nil
	}
	DebugLog = new(logger)
	if DebugLog == nil {
		return errors.New("malloc new logger flied")
	}
	if err := DebugLog.setLogger(config); err != nil {
		return err
	}
	if !DebugLog.isInit() {
		return errors.New("debug logger init failed")
	}
	return nil
}

// CustomLogger custom logger
type CustomLogger struct {
	*logger
}

// NewCustomLogger create a new custom logger
func NewCustomLogger(config *LogConfig, ctx context.Context) (*CustomLogger, error) {
	if config == nil {
		return nil, errors.New("custom logger config is nil")
	}
	log := new(logger)
	if err := log.setLogger(config); err != nil {
		return nil, err
	}
	if !log.isInit() {
		return nil, errors.New("logger init failed")
	}
	return &CustomLogger{logger: log}, nil
}

// SetCustomLogger set custom logger
func SetCustomLogger(log *logger) *CustomLogger {
	if log == nil {
		return nil
	}
	return &CustomLogger{logger: log}
}
