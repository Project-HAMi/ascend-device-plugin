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

// Package limiter implement a token bucket limiter
package limiter

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"ascend-common/common-utils/cache"
	"ascend-common/common-utils/hwlog"
	"ascend-common/common-utils/utils"
)

const (
	kilo = 1000.0
	// DefaultDataLimit  default http body limit size
	DefaultDataLimit      = 1024 * 1024 * 10
	defaultMaxConcurrency = 1024
	maxStringLen          = 20
	// DefaultCacheSize  default cache size
	DefaultCacheSize = 1024 * 100
	arrLen           = 2
	// IPReqLimitReg  ip request limit regex string
	IPReqLimitReg = "^[1-9]\\d{0,2}/[1-9]\\d{0,2}$"
)

type limitHandler struct {
	concurrency   chan struct{}
	httpHandler   http.Handler
	log           bool
	method        string
	limitBytes    int64
	ipExpiredTime time.Duration
	ipCache       *cache.ConcurrencyLRUCache
}

// HandlerConfig the configuration of the limitHandler
type HandlerConfig struct {
	// PrintLog whether you need print access log, when use gin framework, suggest to set false,otherwise set true
	PrintLog bool
	// Method only allow setting  http method pass
	Method string
	// LimitBytes set the max http body size
	LimitBytes int64
	// TotalConCurrency set the program total concurrent http request
	TotalConCurrency int
	// IPConCurrency set the signle IP concurrent http request "2/1sec"
	IPConCurrency string
	// CacheSize the local cacheSize
	CacheSize int
}

// StatusResponseWriter the writer record the http status
type StatusResponseWriter struct {
	http.ResponseWriter
	http.Hijacker
	Status int
}

// WriteHeader override the WriteHeader method
func (w *StatusResponseWriter) WriteHeader(status int) {
	w.ResponseWriter.WriteHeader(status)
	w.Status = status
}

// ServeHTTP implement http.Handler
func (h *limitHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	req.Body = http.MaxBytesReader(w, req.Body, h.limitBytes)
	ctx := initContext(req)
	path := req.URL.Path
	clientUserAgent := req.UserAgent()
	clientIP := utils.ClientIP(req)
	if clientIP != "" && h.ipCache != nil {
		if !h.ipCache.SetIfNX(fmt.Sprintf("key-%s", clientIP), "v", h.ipExpiredTime) {
			hwlog.RunLog.WarnfWithCtx(ctx, "Single IP request reject:%s: %s <%3d> |%15s |%s |%d ", req.Method,
				path, http.StatusServiceUnavailable, clientIP, clientUserAgent, syscall.Getuid())
			http.Error(w, "503 too busy", http.StatusServiceUnavailable)
			return
		}
	}
	select {
	case _, ok := <-h.concurrency:
		if !ok {
			//  channel closed and no need return token
			return
		}
		if h.method != "" && req.Method != h.method {
			http.NotFound(w, req)
			//  recover token to the bucket
			h.concurrency <- struct{}{}
			return
		}
		hwlog.RunLog.Debugf("token count:%d", len(h.concurrency))
		start := time.Now()
		statusRes := newResponse(w)
		h.httpHandler.ServeHTTP(statusRes, req)
		stop := time.Since(start)
		h.concurrency <- struct{}{}
		latency := int(math.Ceil(float64(stop.Nanoseconds()) / kilo / kilo))
		if h.log {
			hwlog.RunLog.InfofWithCtx(ctx, "%s %s: %s <%3d> (%dms) |%15s |%s |%d", req.Proto, req.Method, path,
				statusRes.Status, latency, clientIP, clientUserAgent, syscall.Getuid())
		}
	default:
		hwlog.RunLog.WarnfWithCtx(ctx, "Total reject request:%s: %s <%3d> |%15s |%s |%d ", req.Method, path,
			http.StatusServiceUnavailable, clientIP, clientUserAgent, syscall.Getuid())
		http.Error(w, "503 too busy", http.StatusServiceUnavailable)
	}
}

func newResponse(w http.ResponseWriter) *StatusResponseWriter {
	jk, ok := w.(http.Hijacker)
	if !ok {
		hwlog.RunLog.Warn("hijack not implement")
	}
	statusRes := &StatusResponseWriter{
		ResponseWriter: w,
		Status:         http.StatusOK,
		Hijacker:       jk,
	}
	return statusRes
}

func initContext(req *http.Request) context.Context {
	ctx := context.Background()
	reqID := req.Header.Get(hwlog.ReqID.String())
	if reqID != "" {
		ctx = context.WithValue(context.Background(), hwlog.ReqID, reqID)
	}
	id := req.Header.Get(hwlog.UserID.String())
	if id != "" {
		ctx = context.WithValue(ctx, hwlog.UserID, id)
	}
	return ctx
}

// NewLimitHandler new a bucket-token limiter
func NewLimitHandler(maxConcur, maxConcurrency int, handler http.Handler, printLog bool) (http.Handler, error) {
	return NewLimitHandlerWithMethod(maxConcur, maxConcurrency, handler, printLog, "")
}

// NewLimitHandlerWithMethod  new a bucket-token limiter with specific http method
func NewLimitHandlerWithMethod(maxConcur, maxConcurrency int, handler http.Handler, printLog bool,
	httpMethod string) (http.Handler, error) {
	if maxConcur < 1 || maxConcur > maxConcurrency {
		return nil, errors.New("maxConcurrency parameter error")
	}
	conchan := make(chan struct{}, maxConcur)
	return createHandler(conchan, handler, printLog, httpMethod, DefaultDataLimit), nil
}

func createHandler(ch chan struct{}, handler http.Handler, printLog bool,
	httpMethod string, bodySizeLimit int64) *limitHandler {
	h := &limitHandler{
		concurrency:   ch,
		httpHandler:   handler,
		log:           printLog,
		method:        httpMethod,
		limitBytes:    bodySizeLimit,
		ipExpiredTime: time.Duration(-1),
	}
	for i := 0; i < cap(ch); i++ {
		h.concurrency <- struct{}{}
	}
	return h
}

// NewLimitHandlerV2 new a bucket-token limiter which contains limit request by IP
func NewLimitHandlerV2(handler http.Handler, conf *HandlerConfig) (http.Handler, error) {
	if conf == nil {
		return nil, errors.New("parameter error")
	}
	if conf.TotalConCurrency < 1 || conf.TotalConCurrency > defaultMaxConcurrency {
		return nil, errors.New("totalConCurrency parameter error")
	}
	if len(conf.Method) > maxStringLen {
		return nil, errors.New("method parameter error")
	}
	if conf.CacheSize <= 0 {
		hwlog.RunLog.Info("use default cache size")
		conf.CacheSize = DefaultCacheSize
	}
	reg := regexp.MustCompile(IPReqLimitReg)
	if !reg.Match([]byte(conf.IPConCurrency)) {
		return nil, errors.New("IPConCurrency parameter error")
	}
	conchan := make(chan struct{}, conf.TotalConCurrency)
	h := createHandler(conchan, handler, conf.PrintLog, conf.Method, conf.LimitBytes)
	arr := strings.Split(conf.IPConCurrency, "/")
	if len(arr) != arrLen || arr[0] == "0" {
		return nil, errors.New("IPConCurrency parameter error")
	}
	arr1, err := strconv.ParseInt(arr[1], 0, 0)
	if err != nil {
		return nil, fmt.Errorf("IPConCurrency parameter(%s) error, parse to int failed: %v", arr[1], err)
	}
	arr0, err := strconv.ParseInt(arr[0], 0, 0)
	if err != nil || arr0 == 0 {
		return nil, fmt.Errorf("IPConCurrency parameter(%s) error,parse to int failed: %v", arr[0], err)
	}
	h.ipExpiredTime = time.Duration(arr1 * int64(time.Second) / arr0)
	h.ipCache = cache.New(DefaultCacheSize)
	return h, nil

}
