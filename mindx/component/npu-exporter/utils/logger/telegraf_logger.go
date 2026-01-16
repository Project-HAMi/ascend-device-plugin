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
	"strings"

	"github.com/influxdata/telegraf"

	"ascend-common/common-utils/hwlog"
)

var defaultTelegrafLogPath = "/var/log/mindx-dl/npu-exporter/npu-plugin.log"
var dangerousChars = map[string]string{
	"\n": "\\n",
	"\r": "\\r",
	"\t": "\\t",
}

type telegrafLogger struct {
	acc telegraf.Accumulator
}

// dynamicConfigure configures the logger
func (c *telegrafLogger) dynamicConfigure(config Config) {
	c.acc = config.Acc
}

// log logs with specified level
func (c *telegrafLogger) log(ctx context.Context, level Level, args ...interface{}) {
	c.logf(hwlog.DeepIncrease(ctx), level, "%s", args...)
}

// logf logs with specified level and format
func (c *telegrafLogger) logf(ctx context.Context, level Level, format string, args ...interface{}) {
	sanitized := format
	for char, replacement := range dangerousChars {
		sanitized = strings.ReplaceAll(sanitized, char, replacement)
	}
	if level < InfoLevel || c.acc == nil {
		fn, ok := logfFuncs[level]
		if !ok {
			hwlog.RunLog.Warnf("unknown log level: %v", level)
			return
		}

		fn(hwlog.DeepIncrease(ctx), sanitized, args...)
		return
	}

	c.acc.AddError(errors.New(fmt.Sprintf(sanitized, args...)))
}

// LogfWithOptions print log info with options
func (c *telegrafLogger) logfWithOptions(ctx context.Context, level Level, opts LogOptions, format string,
	args ...interface{}) {

	if opts.MaxCounts == 0 {
		opts.MaxCounts = hwlog.ProblemOccurMaxNumbers
	}

	if needPrint, extraErrLog := hwlog.IsNeedPrintWithSpecifiedCounts(opts.Domain, opts.ID, opts.MaxCounts); needPrint {
		format = fmt.Sprintf("%s %s", format, extraErrLog)
		c.logf(hwlog.DeepIncrease(ctx), level, format, args...)
	}
}
