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

// Package limiter implement a writer limiter
package limiter

import (
	"bytes"
	"errors"

	"ascend-common/common-utils/hwlog"
)

const defaultLimit = 1024

// LimitedWriter limit the size of written data
type LimitedWriter struct {
	buffer *bytes.Buffer
	limit  int
	size   int
}

// NewLimitedWriter create a LimitedWriter
func NewLimitedWriter(limit int) *LimitedWriter {
	if limit <= 0 {
		hwlog.RunLog.Warnf("limit: %v is invalid, set default limit: %v", limit, defaultLimit)
		limit = defaultLimit
	}
	return &LimitedWriter{
		buffer: &bytes.Buffer{},
		limit:  limit,
	}
}

// Write write bytes to buffer
func (lw *LimitedWriter) Write(p []byte) (int, error) {
	if lw.size+len(p) > lw.limit {
		return 0, errors.New("buffer limit exceeded")
	}
	n, err := lw.buffer.Write(p)
	if err == nil {
		lw.size += n
	}
	return n, err
}

// GetBufferBytes get buffer bytes
func (lw *LimitedWriter) GetBufferBytes() []byte {
	if lw.buffer == nil {
		return []byte{}
	}
	return lw.buffer.Bytes()
}
