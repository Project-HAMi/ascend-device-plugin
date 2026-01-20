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
	"fmt"
	"sync"
	"time"

	"ascend-common/common-utils/cache"
)

const (
	// MaxCacheSize indicates the maximum log cache size
	MaxCacheSize = 100 * 1024
	// MaxExpiredTime indicates the maximum log cache expired time
	MaxExpiredTime = 60 * 60
	// DefaultCacheSize indicates the default log cache size
	DefaultCacheSize = 10 * 1024
	// DefaultExpiredTime indicates the default log cache expired time
	DefaultExpiredTime = 1
	cutPreLen          = 46
	// ProblemOccurMaxNumbers indicates the maximum number of times that the same problem can occur
	ProblemOccurMaxNumbers = 3
)

var (
	errorMap sync.Map
)

// LogLimiter encapsulates Logs and provides the log traffic limiting capability
// to prevent too many duplicate logs.
type LogLimiter struct {
	// Logs is a log rotate instance
	Logs     *Logs
	logCache *cache.ConcurrencyLRUCache
	logMu    sync.Mutex
	doOnce   sync.Once

	logExpiredTime time.Duration
	// CacheSize indicates the size of log cache
	CacheSize int
	// ExpiredTime indicates the expired time of log cache
	ExpiredTime int
}

// Write implements io.Writer. It encapsulates the Write method of Los and uses
// the lru cache to prevent duplicate log writing.
func (l *LogLimiter) Write(d []byte) (int, error) {
	if l == nil {
		return 0, fmt.Errorf("log limiter pointer does not exist")
	}

	l.logMu.Lock()
	defer l.logMu.Unlock()

	if l.ExpiredTime == 0 || l.CacheSize == 0 {
		return l.Logs.Write(d)
	}

	l.doOnce.Do(func() {
		l.validateLimiterConf()
		l.logCache = cache.New(l.CacheSize)
		l.logExpiredTime = time.Duration(int64(l.ExpiredTime) * int64(time.Second))
	})

	if l.logCache == nil {
		l.logCache = cache.New(DefaultCacheSize)
	}
	if !l.logCache.SetIfNX(string(d[cutPreLen:]), "v", l.logExpiredTime) {
		return 0, nil
	}

	return l.Logs.Write(d)
}

// Close implements io.Closer. It encapsulates the Close method of Logs.
func (l *LogLimiter) Close() error {
	if l == nil {
		return fmt.Errorf("log limiter pointer does not exist")
	}

	l.logMu.Lock()
	defer l.logMu.Unlock()

	return l.Logs.Close()
}

// Flush encapsulates the Flush method of Logs.
func (l *LogLimiter) Flush() error {
	if l == nil {
		return fmt.Errorf("log limiter pointer does not exist")
	}

	l.logMu.Lock()
	defer l.logMu.Unlock()

	return l.Logs.Flush()
}

// validateLimiterConf verifies the external input parameters in the LogLimiter.
func (l *LogLimiter) validateLimiterConf() {
	if l.CacheSize < 0 || l.CacheSize > MaxCacheSize {
		l.CacheSize = DefaultCacheSize
	}
	if l.ExpiredTime < 0 || l.ExpiredTime > MaxExpiredTime {
		l.ExpiredTime = DefaultExpiredTime
	}
}

func getKey(domain string, id interface{}) string {
	return fmt.Sprintf("%d_%s", id, domain)
}

// IsNeedPrintWithSpecifiedCounts check whether print the error message,
// if the error message (domain_id as a unique identifier) has been printed
// for problemOccurMaxNumbers times, return false
func IsNeedPrintWithSpecifiedCounts(domain string, id interface{}, problemOccurMaxNumbers int) (bool, string) {
	key := getKey(domain, id)
	cnt, _ := errorMap.LoadOrStore(key, 0)
	intCnt, ok := cnt.(int)
	extraErrLog := ""
	if !ok {
		// the counter type is abnormal, print by default
		return true, extraErrLog
	}
	if intCnt >= problemOccurMaxNumbers {
		return false, extraErrLog
	}
	intCnt += 1
	errorMap.Store(key, intCnt)
	if intCnt == problemOccurMaxNumbers {
		extraErrLog = fmt.Sprintf(".The error log has been printed for %v times "+
			"and will not be printed any more", problemOccurMaxNumbers)
	}
	return true, extraErrLog

}

// ResetErrCnt reset the error count
func ResetErrCnt(domain string, id interface{}) {
	errorMap.Delete(getKey(domain, id))
}
