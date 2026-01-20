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
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"ascend-common/api"
	"huawei.com/npu-exporter/v6/collector/container"
	"huawei.com/npu-exporter/v6/utils/logger"
)

var (
	// CardLabel general card label
	CardLabel = []string{npuID, modelName, npuUUID, npuPCIEInfo, namespace, podName, cntrName}

	noNeedToPrintUpdateLog = map[string]bool{
		"NetworkCollector": true,
		"RoceCollector":    true,
		"OpticalCollector": true,
	}
)

// BuildDescSlice build desc slice
func BuildDescSlice(slice *[]*prometheus.Desc, name string, help string) {
	*slice = append(*slice, BuildDesc(name, help))
}

// BuildDesc build desc
func BuildDesc(name string, help string) *prometheus.Desc {
	return prometheus.NewDesc(name, help, CardLabel, nil)
}

// BuildDescWithLabel build desc with label
func BuildDescWithLabel(name string, help string, label []string) *prometheus.Desc {
	return prometheus.NewDesc(name, help, label, nil)
}

// MetricsCollector metrics collector
type MetricsCollector interface {
	// Describe report metrics to prometheus
	Describe(ch chan<- *prometheus.Desc)

	// CollectToCache collect data to cache
	CollectToCache(n *NpuCollector, chipList []HuaWeiAIChip)

	// UpdatePrometheus update prometheus
	UpdatePrometheus(ch chan<- prometheus.Metric, n *NpuCollector, containerMap map[int32]container.DevicesInfo,
		chips []HuaWeiAIChip)

	// UpdateTelegraf update telegraf
	UpdateTelegraf(fieldsMap map[string]map[string]interface{}, n *NpuCollector,
		containerMap map[int32]container.DevicesInfo, chips []HuaWeiAIChip) map[string]map[string]interface{}

	// PreCollect pre handle before collect
	PreCollect(*NpuCollector, []HuaWeiAIChip)

	// PostCollect post handle after collect
	PostCollect(*NpuCollector)

	// IsSupported Check whether the current hardware supports this metric
	IsSupported(*NpuCollector) bool
}

// MetricsCollectorAdapter base collector for metrics collector
type MetricsCollectorAdapter struct {
	LocalCache   sync.Map
	Is910Series  bool
	ContainerMap map[int32]container.DevicesInfo
	Chips        []HuaWeiAIChip
}

// Describe report metrics to prometheus
func (c *MetricsCollectorAdapter) Describe(ch chan<- *prometheus.Desc) {
}

// CollectToCache collect data to cache
func (c *MetricsCollectorAdapter) CollectToCache(n *NpuCollector, chipList []HuaWeiAIChip) {
}

// UpdatePrometheus update prometheus
func (c *MetricsCollectorAdapter) UpdatePrometheus(ch chan<- prometheus.Metric, n *NpuCollector,
	containerMap map[int32]container.DevicesInfo, chips []HuaWeiAIChip) {
}

// UpdateTelegraf update telegraf
func (c *MetricsCollectorAdapter) UpdateTelegraf(fieldsMap map[string]map[string]interface{}, n *NpuCollector,
	containerMap map[int32]container.DevicesInfo, chips []HuaWeiAIChip) map[string]map[string]interface{} {
	return fieldsMap
}

// PreCollect pre handle before collect
func (c *MetricsCollectorAdapter) PreCollect(n *NpuCollector, chipList []HuaWeiAIChip) {
	if strings.Contains(n.Dmgr.GetDevType(), api.Ascend910A) {
		c.Is910Series = true
	}
}

// PostCollect post handle after collect
func (c *MetricsCollectorAdapter) PostCollect(*NpuCollector) {
}

// IsSupported Check whether the current hardware supports this metric
func (c *MetricsCollectorAdapter) IsSupported(*NpuCollector) bool {
	return true
}

// UpdateCache update cache
func UpdateCache[T any](n *NpuCollector, cacheKey string, localCache *sync.Map) {
	var cacheInfo = make(map[int32]T)
	obj, err := n.cache.Get(cacheKey)
	if err != nil {
		logger.Debugf("get info of %s failed: %v, use initial data", cacheKey, err)
	} else {
		if oldCacheInfo, ok := obj.(map[int32]T); ok {
			cacheInfo = copyMap(oldCacheInfo)
		} else {
			logger.Debug("cache format invalid, reset")
		}
	}

	localCache.Range(func(key, value interface{}) bool {
		finalKey, okKey := key.(int32)
		finalValue, okValue := value.(T)
		if okKey && okValue {
			cacheInfo[finalKey] = finalValue
		}
		return true
	})

	err = n.cache.Set(cacheKey, cacheInfo, n.cacheTime)
	if noNeedToPrintUpdateLog[cacheKey] {
		return
	}
	if err != nil {
		logger.Error(err)
	}
}

func copyMap[T any](oldCacheInfo map[int32]T) map[int32]T {
	var cacheInfo = make(map[int32]T)
	for key, value := range oldCacheInfo {
		cacheInfo[key] = value
	}
	return cacheInfo
}

// GetInfoFromCache get info from cache
func GetInfoFromCache[T any](n *NpuCollector, cacheKey string) map[int32]T {
	res := make(map[int32]T)
	obj, err := n.cache.Get(cacheKey)
	if err != nil {
		logger.Warn("cache not found, please wait for rebuild")
		return res
	}

	if data, ok := obj.(map[int32]T); ok {
		return data
	}
	logger.Error("cache type mismatch")
	return res
}

// GetCacheKey Obtain the name of the struct pointer as the key of the cache
func GetCacheKey(ptr interface{}) string {
	v := reflect.ValueOf(ptr)
	if v.Kind() != reflect.Ptr {
		return ""
	}
	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return ""
	}
	return v.Type().Name()
}
