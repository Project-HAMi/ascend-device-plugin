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

import "errors"

// ContextKey especially for context value
// to solve problem of "should not use basic type untyped string as key in context.WithValue"
type ContextKey string

// String  the implement of String method
func (c ContextKey) String() string {
	return string(c)
}

const (
	// UserID used for context value key of "ID"
	UserID ContextKey = "UserID"
	// ReqID used for context value key of "requestID"
	ReqID ContextKey = "RequestID"
	// extraDeepKey used for context value key of "extraDeepKey"
	extraDeepKey ContextKey = "extraDeepKey"
)

// SelfLogWriter used this to replace some opensource log
type SelfLogWriter struct {
}

// Write  implement the interface of io.writer
func (l *SelfLogWriter) Write(p []byte) (int, error) {
	if RunLog == nil {
		return -1, errors.New("hwlog is not initialized")
	}
	RunLog.Info(string(p))
	return len(p), nil
}
