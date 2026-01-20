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

// Package common for general collector
package common

import (
	"reflect"
	"sync"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/smartystreets/goconvey/convey"

	"ascend-common/api"
)

// TestCopyMap test copyMap
func TestCopyMap(t *testing.T) {
	type testStruct struct {
		name string
		age  int
	}
	mockString := "mock"
	tests := []struct {
		name     string
		input    map[int32]testStruct
		validate func(*testing.T, interface{})
	}{
		{name: "NilInput", input: (map[int32]testStruct)(nil),
			validate: func(t *testing.T, got interface{}) {
				g, ok := got.(map[int32]testStruct)
				if !ok || g == nil || len(g) != 0 {
					t.Errorf("should return empty map for nil input")
				}
			}},
		{name: "EmptyMap", input: map[int32]testStruct{},
			validate: func(t *testing.T, got interface{}) {
				if len(got.(map[int32]testStruct)) != 0 {
					t.Errorf("expected empty map")
				}
			}},
		{name: "SingleElement", input: map[int32]testStruct{1: {name: mockString, age: 1}},
			validate: func(t *testing.T, got interface{}) {
				g, ok := got.(map[int32]testStruct)
				if !ok || g[1].name != mockString || g[1].age != 1 || len(g) != 1 {
					t.Errorf("element mismatch")
				}
			}},
		{name: "MultipleElements", input: map[int32]testStruct{1: {name: mockString, age: 1}, 2: {name: mockString, age: 1}},
			validate: func(t *testing.T, got interface{}) {
				expected := map[int32]testStruct{1: {name: mockString, age: 1}, 2: {name: mockString, age: 1}}
				if !reflect.DeepEqual(got, expected) {
					t.Errorf("deepEqual failed")
				}
			}},
	}

	for _, tt := range tests {
		convey.Convey(tt.name, t, func() {
			got := copyMap[testStruct](tt.input)
			tt.validate(t, got)
		})
	}
}

func TestPreCollect(t *testing.T) {
	tests := []struct {
		name       string
		deviceType string
		expected   bool
	}{
		{name: "TestPreCollect_" + api.Ascend910,
			deviceType: api.Ascend910,
			expected:   true,
		},
		{name: "TestPreCollect_" + api.Ascend310,
			deviceType: api.Ascend310,
			expected:   false,
		},
	}
	convey.Convey("TestPreCollect", t, func() {
		n := mockNewNpuCollector()
		adapter := MetricsCollectorAdapter{
			Is910Series:  false,
			ContainerMap: nil,
			Chips:        nil,
		}
		for _, tt := range tests {
			convey.Convey(tt.name, func() {
				patches := gomonkey.NewPatches()
				defer patches.Reset()
				patches.ApplyMethodReturn(n.Dmgr, "GetDevType", tt.deviceType)
				adapter.PreCollect(n, nil)
				convey.So(adapter.Is910Series, convey.ShouldEqual, tt.expected)
			})
		}
	})
}

type cacheCase struct {
	name      string
	cacheKey  string
	preHandle func()
	expected  int
}

func buildTestsForUpdateCache(expected int) []cacheCase {
	tests := []cacheCase{
		{name: "TestUpdateCache_save info to cache",
			cacheKey:  "mockKey1",
			preHandle: func() {},
			expected:  expected,
		},
		{name: "TestUpdateCache_update old cache",
			cacheKey: "mockKey2",
			preHandle: func() {
				noNeedToPrintUpdateLog["mockKey2"] = true
			},
			expected: expected,
		},
		{name: "TestUpdateCache_old cache is in incorrect type",
			cacheKey:  "mockKey3",
			preHandle: func() {},
			expected:  expected,
		},
	}
	return tests
}

func TestUpdateCache(t *testing.T) {
	const key = int32(0)
	const expected = 1
	tests := buildTestsForUpdateCache(expected)

	n := mockNewNpuCollector()
	// data init
	n.cache.Set("mockKey2", map[int32]string{key: "0"}, n.cacheTime)
	n.cache.Set("mockKey3", map[int32]int{key: 0}, n.cacheTime)

	convey.Convey("TestUpdateCache", t, func() {

		for _, tt := range tests {
			convey.Convey(tt.name, func() {
				localCache := sync.Map{}
				localCache.Store(key, "mockValue")
				tt.preHandle()
				UpdateCache[string](n, tt.cacheKey, &localCache)

				data, err := n.cache.Get(tt.cacheKey)
				convey.So(err, convey.ShouldBeNil)
				map2, ok := data.(map[int32]string)
				convey.So(ok, convey.ShouldBeTrue)
				convey.So(len(map2), convey.ShouldEqual, tt.expected)
			})
		}

	})
}

func TestGetInfoFromCache(t *testing.T) {
	const key = int32(0)
	tests := []struct {
		name     string
		cacheKey string
		expected int
	}{
		{name: "TestGetInfoFromCache_no info in cache",
			cacheKey: "mockKey1",
			expected: 0,
		},
		{name: "TestGetInfoFromCache_correct",
			cacheKey: "mockKey2",
			expected: 1,
		},
		{name: "TestGetInfoFromCache_info in cache is in incorrect type",
			cacheKey: "mockKey3",
			expected: 0,
		},
	}
	n := mockNewNpuCollector()
	// data init
	n.cache.Set("mockKey2", map[int32]string{key: "mockValue"}, n.cacheTime)
	n.cache.Set("mockKey3", map[int32]int{key: 0}, n.cacheTime)
	for _, tt := range tests {
		convey.Convey(tt.name, t, func() {
			cache := GetInfoFromCache[string](n, tt.cacheKey)
			convey.So(len(cache), convey.ShouldEqual, tt.expected)
		})
	}
}

func TestGetCacheKey(t *testing.T) {
	tests := []struct {
		name     string
		args     interface{}
		expected string
	}{
		{name: "TestGetCacheKey_ptr",
			args:     &MetricsCollectorAdapter{},
			expected: "MetricsCollectorAdapter",
		},
		{name: "TestGetCacheKey_int",
			args:     0,
			expected: "",
		},
		{name: "TestGetCacheKey_struct",
			args:     MetricsCollectorAdapter{},
			expected: "",
		},
	}

	convey.Convey("TestGetCacheKey", t, func() {
		for _, tt := range tests {
			convey.Convey(tt.name, func() {
				convey.So(GetCacheKey(tt.args), convey.ShouldEqual, tt.expected)
			})
		}
	})
}
