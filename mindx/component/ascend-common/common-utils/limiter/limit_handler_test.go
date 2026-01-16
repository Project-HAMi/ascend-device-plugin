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
	"net/http"
	"net/url"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/smartystreets/goconvey/convey"

	"ascend-common/common-utils/hwlog"
)

func init() {
	config := hwlog.LogConfig{
		OnlyToStdout: true,
	}
	hwlog.InitRunLogger(&config, context.TODO())
}
func TestServeHTTP(t *testing.T) {
	convey.Convey("test limitHandler serveHTTP", t, func() {
		h, w, r := initVarable()
		convey.Convey("header contains reqID and userID,", func() {
			mock := gomonkey.ApplyMethodFunc(h.httpHandler, "ServeHTTP", func(http.ResponseWriter,
				*http.Request) {
				return
			})
			defer mock.Reset()
			h.ServeHTTP(w.ResponseWriter, r)
			convey.So(len(h.concurrency), convey.ShouldEqual, 1)
		})
		convey.Convey("token channel close,", func() {
			mock := gomonkey.ApplyFunc(http.Error, func(http.ResponseWriter, string, int) {
				return
			})
			defer mock.Reset()
			_, ok := <-h.concurrency
			if !ok {
				return
			}
			h.ServeHTTP(w.ResponseWriter, r)
			convey.So(len(h.concurrency), convey.ShouldEqual, 0)
		})
	})
}

func initVarable() (*limitHandler, StatusResponseWriter, *http.Request) {
	lh, err := NewLimitHandler(1, len2, http.DefaultServeMux, false)
	if err != nil {
		return nil, StatusResponseWriter{}, nil
	}
	v, ok := lh.(*limitHandler)
	if !ok {
		return nil, StatusResponseWriter{}, nil
	}
	w := StatusResponseWriter{
		ResponseWriter: nil,
		Status:         0,
	}
	r := &http.Request{
		URL: &url.URL{
			Path: "test.com",
		},
		Header: map[string][]string{"userID": {"1"}, "reqID": {"requestIDxxxx"}},
		Method: "GET",
	}
	return v, w, r
}

func TestNewLimitHandlerV2(t *testing.T) {
	conf := &HandlerConfig{
		PrintLog:         false,
		Method:           "",
		LimitBytes:       DefaultDataLimit,
		TotalConCurrency: defaultMaxConcurrency,
		IPConCurrency:    "2/1",
		CacheSize:        DefaultCacheSize,
	}
	convey.Convey("normal situation,no err return", t, func() {
		_, err := NewLimitHandlerV2(http.DefaultServeMux, conf)
		convey.So(err, convey.ShouldEqual, nil)
	})
	convey.Convey("IPConCurrency parameter error", t, func() {
		conf.IPConCurrency = "2021/1"
		_, err := NewLimitHandlerV2(http.DefaultServeMux, conf)
		convey.So(err, convey.ShouldNotEqual, nil)
	})
	convey.Convey("cacheSize parameter error", t, func() {
		conf.CacheSize = 0
		_, err := NewLimitHandlerV2(http.DefaultServeMux, conf)
		convey.So(err, convey.ShouldNotEqual, nil)
	})
	convey.Convey("method parameter error", t, func() {
		conf.Method = "20/iajsdkjas2jhjdklsjkldjsdfasd1"
		_, err := NewLimitHandlerV2(http.DefaultServeMux, conf)
		convey.So(err, convey.ShouldNotEqual, nil)
	})
	convey.Convey("TotalConCurrency parameter error", t, func() {
		conf.TotalConCurrency = 0
		_, err := NewLimitHandlerV2(http.DefaultServeMux, conf)
		convey.So(err, convey.ShouldNotEqual, nil)
	})
}
