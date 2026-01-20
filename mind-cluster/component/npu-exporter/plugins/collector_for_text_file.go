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

// Package plugins for custom metrics
package plugins

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"ascend-common/common-utils/hwlog"
	"ascend-common/common-utils/utils"
	"huawei.com/npu-exporter/v6/collector/common"
	"huawei.com/npu-exporter/v6/collector/config"
	"huawei.com/npu-exporter/v6/collector/container"
	npuutils "huawei.com/npu-exporter/v6/utils"
	"huawei.com/npu-exporter/v6/utils/logger"
)

var (
	metricDesc     *prometheus.Desc
	labelKeys      []string // a list of tag keys extracted from the datalist
	jsonFilePath   string
	isSupported    bool
	currentVersion versionInfo
)

const (
	size100k                 = 100 * 1024
	maxLabelSize             = 10
	num1000                  = 1000
	maxDataListSize          = 128
	maxMetricNameSize        = 128
	maxDescSize              = 1024
	fileMetricsDisabledMsg   = "file metrics collection will be disabled"
	skipCurrentCollectionMsg = "will skip current collection and report cached metrics"
	excludedPermission       = 0111 // file should not have any execute permission
)

type versionInfo struct {
	name    string
	desc    string
	version string
}

// TextMetricData represents the JSON structure
type TextMetricData struct {
	Version   string     `json:"version"`
	Desc      string     `json:"desc"`
	Name      string     `json:"name"`
	Timestamp int64      `json:"timestamp"`
	DataList  []DataItem `json:"data_list"`
}

// DataItem represents each item in data_list
type DataItem struct {
	Label map[string]string `json:"label"`
	Value float64           `json:"value"`
}

// InitTextMetricsDesc init text metric
func InitTextMetricsDesc(filePath string) {
	if filePath == "" {
		return
	}
	paths := strings.Split(filePath, ",")
	if len(paths) > 1 {
		logger.Warnf("multiple file paths detected in filePath: %s, only the first file will be used", filePath)
		jsonFilePath = strings.TrimSpace(paths[0])
	} else {
		jsonFilePath = filePath
	}
	if utils.IsDir(jsonFilePath) {
		logger.Errorf("file path %s is a directory, only support specify file path", filePath)
		return
	}
	fileData, err := waitForFile(jsonFilePath, time.Minute)
	if err != nil {
		logger.Warnf("read json file %s failed, %s: %v", jsonFilePath, fileMetricsDisabledMsg, err)
		return
	}
	var metricsData TextMetricData
	if err := json.Unmarshal(fileData, &metricsData); err != nil {
		logger.Warnf("unmarshal json file %s failed, %s: %v, "+
			"Possible causes:\n1. The file is not in JSON format\n2. File size is more than 100KB ", jsonFilePath, fileMetricsDisabledMsg, err)
		return
	}

	if err := isDataOk(&metricsData); err != nil {
		logger.Warnf("%v, %s", err, fileMetricsDisabledMsg)
		return
	}

	desc := metricsData.Desc
	labelKeys = make([]string, 0, len(metricsData.DataList[0].Label))
	for key := range metricsData.DataList[0].Label {
		labelKeys = append(labelKeys, key)
	}
	sort.Strings(labelKeys)
	logger.Infof("init text metric succeeded, metricName: %v, version: %v, desc: %v, labels: %v",
		metricsData.Name, metricsData.Version, desc, labelKeys)

	metricDesc = prometheus.NewDesc(metricsData.Name, desc, labelKeys, nil)
	isSupported = true
	currentVersion = versionInfo{
		name:    metricsData.Name,
		desc:    desc,
		version: metricsData.Version,
	}
	err = config.AddPluginCollector("text", &TextMetricsInfoCollector{})
	if err != nil {
		logger.Errorf("%v", err)
	}
}

func isDataOk(metricsData *TextMetricData) error {
	if len(metricsData.DataList) == 0 {
		return fmt.Errorf("dataList is empty in json file %s", jsonFilePath)
	}
	if len(metricsData.DataList) > maxDataListSize {
		return fmt.Errorf("size of dataList(%d) is more than max allowed dataList size(%d) in json file %s",
			len(metricsData.DataList), maxDataListSize, jsonFilePath)
	}
	if len(metricsData.DataList[0].Label) > maxLabelSize {
		return fmt.Errorf("size of first item's Label(%d) is more than max allowed label size(%d) in json file %s",
			len(metricsData.DataList[0].Label), maxLabelSize, jsonFilePath)
	}
	if metricsData.Name == "" {
		return fmt.Errorf("name field is empty in json file %s", jsonFilePath)
	}
	if len(metricsData.Name) > maxMetricNameSize {
		return fmt.Errorf("length of metric name should not larger than %d, but current is %d",
			maxMetricNameSize, len(metricsData.Name))
	}
	if metricsData.Desc == "" {
		return fmt.Errorf("desc field is empty in json file %s", jsonFilePath)
	}
	if len(metricsData.Desc) > maxDescSize {
		return fmt.Errorf("length of metric desc should not larger than %d, but current is %d",
			maxDescSize, len(metricsData.Desc))
	}
	if metricsData.Version == "" {
		return fmt.Errorf("version field is empty in json file %s", jsonFilePath)
	}
	// only support 1.0 version currently
	if metricsData.Version != "1.0" {
		return fmt.Errorf("version should be 1.0, but current is %s", metricsData.Version)
	}
	if metricsData.Timestamp <= 0 {
		return fmt.Errorf("timestamp field is empty or not correct in json file %s", jsonFilePath)
	}
	return nil
}

// waitForFile wait for file to exist
func waitForFile(filePath string, timeout time.Duration) ([]byte, error) {
	const tickerDuration = 100
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(tickerDuration * time.Millisecond)
	defer ticker.Stop()
	once := sync.Once{}

	for {
		fileData, err := utils.ReadLimitBytes(filePath, size100k)
		err2 := checkFile(filePath)
		if err2 != nil {
			hwlog.RunLog.Errorf("check file err, %s: %v", filePath, err2)
		}
		if err2 != nil && !os.IsNotExist(err2) {
			return nil, err2
		}

		if err == nil && err2 == nil && len(fileData) > 0 {
			logger.Infof("successfully read json file %s", filePath)
			return fileData, nil
		}
		if os.IsNotExist(err) || len(fileData) == 0 {
			once.Do(func() {
				logger.Warnf("file [%v] is not exist or file is empty, will wait 1 minute", filePath)
			})
			if time.Now().After(deadline) {
				return nil, fmt.Errorf("file %s does not exist or file is empty after waiting %v", filePath, timeout)
			}
			select {
			case <-ticker.C:
				continue
			}
		}
		return nil, err
	}
}

func checkFile(filePath string) error {
	absFilePath, err := utils.CheckPath(filePath)
	if err != nil {
		return err
	}
	if err = utils.DoCheckOwnerAndPermission(absFilePath, excludedPermission, 0); err != nil {
		logger.Errorf("file permission should not included %04o: %v", excludedPermission, err)
		return err
	}
	return nil
}

// TextMetricsInfoCollector collect custom plugin info
type TextMetricsInfoCollector struct {
	common.MetricsCollectorAdapter
	Cache sync.Map
}

// Describe description of the metric
func (c *TextMetricsInfoCollector) Describe(ch chan<- *prometheus.Desc) {
	// add desc
	if metricDesc != nil {
		ch <- metricDesc
	}
}

// CollectToCache collect the metric to cache
func (c *TextMetricsInfoCollector) CollectToCache(n *common.NpuCollector, chipList []common.HuaWeiAIChip) {
	// collect metric to cache
	logger.Debugf("TextMetricsInfoCollector CollectToCache")

	fileData, err := utils.ReadLimitBytes(jsonFilePath, size100k)
	if err != nil {
		logger.LogfWithOptions(logger.WarnLevel, logger.LogOptions{Domain: "textMetrics", ID: "readFileErr"},
			"read json file %s failed: %v", jsonFilePath, err)
		return
	}
	hwlog.ResetErrCnt("textMetrics", "readFileErr")

	var metricsData TextMetricData
	if err := json.Unmarshal(fileData, &metricsData); err != nil {
		logger.LogfWithOptions(logger.WarnLevel, logger.LogOptions{Domain: "textMetrics", ID: "unmarshalFileErr"},
			"unmarshal json file %s failed: %v", jsonFilePath, err)
		return
	}
	hwlog.ResetErrCnt("textMetrics", "unmarshalFileErr")

	if err := isDataOk(&metricsData); err != nil {
		logger.LogfWithOptions(logger.WarnLevel, logger.LogOptions{Domain: "textMetrics", ID: "dataNotOk"},
			"%v, %s", err, skipCurrentCollectionMsg)
		return
	}
	hwlog.ResetErrCnt("textMetrics", "dataNotOk")

	if versionChanged(metricsData) {
		logger.LogfWithOptions(logger.ErrorLevel, logger.LogOptions{Domain: "textMetrics", ID: "versionChanged"},
			"json file base info changed, old: %v, new: %v", currentVersion,
			versionInfo{name: metricsData.Name, desc: metricsData.Desc, version: metricsData.Version})
		return
	}
	hwlog.ResetErrCnt("textMetrics", "versionChanged")

	c.Cache.Store(common.GetCacheKey(c), metricsData)
}

func versionChanged(data TextMetricData) bool {
	if currentVersion.name != data.Name || currentVersion.desc != data.Desc ||
		currentVersion.version != data.Version {
		return true
	}
	return false
}

// UpdatePrometheus update prometheus metric
func (c *TextMetricsInfoCollector) UpdatePrometheus(ch chan<- prometheus.Metric, n *common.NpuCollector,
	containerMap map[int32]container.DevicesInfo, chips []common.HuaWeiAIChip) {
	logger.Debug("TextMetricsInfoCollector UpdatePrometheus")
	if metricDesc == nil {
		logger.Warnf("metricDesc is not initialized, skip UpdatePrometheus")
		return
	}
	cacheKey := common.GetCacheKey(c)
	data, ok := c.Cache.Load(cacheKey)
	if !ok {
		logger.Debugf("cache key %s not found", cacheKey)
		return
	}

	textMetricsData, ok := data.(TextMetricData)
	if !ok {
		logger.Warnf("cache data type mismatch for key %s", cacheKey)
		return
	}

	timestamp := time.Unix(0, textMetricsData.Timestamp*num1000)

	for _, item := range textMetricsData.DataList {
		labelValues := make([]string, len(labelKeys))
		for i, key := range labelKeys {
			if value, ok := item.Label[key]; ok {
				labelValues[i] = value
			} else {
				labelValues[i] = ""
			}
		}

		ch <- prometheus.NewMetricWithTimestamp(timestamp,
			prometheus.MustNewConstMetric(metricDesc, prometheus.GaugeValue, item.Value, labelValues...))
	}
}

// UpdateTelegraf update telegraf metric
func (c *TextMetricsInfoCollector) UpdateTelegraf(fieldsMap map[string]map[string]interface{}, n *common.NpuCollector,
	containerMap map[int32]container.DevicesInfo, chips []common.HuaWeiAIChip) map[string]map[string]interface{} {
	logger.Debug("TextMetricsInfoCollector UpdateTelegraf")

	if metricDesc == nil {
		logger.Warnf("metricDesc is not initialized, skip UpdateTelegraf")
		return fieldsMap
	}

	cacheKey := common.GetCacheKey(c)
	data, ok := c.Cache.Load(cacheKey)
	if !ok {
		logger.Debugf("cache key %s not found", cacheKey)
		return fieldsMap
	}

	textMetricData, ok := data.(TextMetricData)
	if !ok {
		logger.Warnf("cache data type mismatch for key %s", cacheKey)
		return fieldsMap
	}

	for _, item := range textMetricData.DataList {
		if fieldsMap[common.GeneralDevTagKey] == nil {
			fieldsMap[common.GeneralDevTagKey] = make(map[string]interface{})
		}
		npuutils.DoUpdateTelegraf(fieldsMap[common.GeneralDevTagKey], metricDesc, item.Value, "")
	}

	return fieldsMap
}

// IsSupported Check whether the current hardware supports this metric
func (c *TextMetricsInfoCollector) IsSupported(n *common.NpuCollector) bool {
	return isSupported
}
