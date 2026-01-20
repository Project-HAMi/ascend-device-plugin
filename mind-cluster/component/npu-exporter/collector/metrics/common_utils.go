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

// Package metrics offer common utils for collector
package metrics

import (
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"ascend-common/common-utils/hwlog"
	"ascend-common/devmanager/common"
	colcommon "huawei.com/npu-exporter/v6/collector/common"
	"huawei.com/npu-exporter/v6/collector/container"
	"huawei.com/npu-exporter/v6/utils"
	"huawei.com/npu-exporter/v6/utils/logger"
)

func validateNum(num float64) bool {
	if num == -1 || num == math.MaxUint32 || float32(num) == math.MaxUint32 {
		return false
	}

	return true
}

func doUpdateTelegrafWithValidateNum(fieldMap map[string]interface{}, desc *prometheus.Desc,
	value float64, extInfo string) {
	if validateNum(value) {
		doUpdateTelegraf(fieldMap, desc, value, extInfo)
	}
}

func doUpdateTelegraf(fieldMap map[string]interface{}, desc *prometheus.Desc, value interface{}, extInfo string) {
	fieldMap[utils.GetDescName(desc)+extInfo] = value
}

func doUpdateMetricWithValidateNum(ch chan<- prometheus.Metric, timestamp time.Time, value float64,
	cardLabel []string, desc *prometheus.Desc) {
	if validateNum(value) {
		doUpdateMetric(ch, timestamp, value, cardLabel, desc)
	}
}
func doUpdateMetric(ch chan<- prometheus.Metric, timestamp time.Time, value interface{},
	cardLabel []string, desc *prometheus.Desc) {
	var finalValue float64

	switch value.(type) {
	case int:
		finalValue = float64(value.(int))
	case int32:
		finalValue = float64(value.(int32))
	case int64:
		finalValue = float64(value.(int64))
	case uint32:
		finalValue = float64(value.(uint32))
	case uint64:
		finalValue = float64(value.(uint64))
	case float32:
		finalValue = float64(value.(float32))
	case float64:
		finalValue = value.(float64)
	default:
		logger.Errorf("invalid param in function doUpdateMetric,"+
			"metrics name is (%v), value type is (%T),value is (%v)", utils.GetDescName(desc), value, value)
	}
	// collect failed, set value to -1
	if finalValue == common.FailedValue {
		finalValue = common.FailedMetricValue
	}
	ch <- prometheus.NewMetricWithTimestamp(timestamp,
		prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, finalValue, cardLabel...))
}

func getContainerInfoWithDefault(cNameArray []string) (containerName, namespaceValue, podNameValue string) {
	if len(cNameArray) == colcommon.ContainerNameLen {
		namespaceValue = cNameArray[colcommon.NameSpaceIdx]
		podNameValue = cNameArray[colcommon.PodNameIdx]
		containerName = cNameArray[colcommon.ConNameIdx]
	}
	return containerName, namespaceValue, podNameValue
}

func geenGeneralCardLabel(chip *colcommon.HuaWeiAIChip, containerMap map[int32]container.DevicesInfo) []string {

	containerInfo := geenContainerInfo(chip, containerMap)

	containerName, namespaceValue, podNameValue := getContainerInfoWithDefault(getContainerNameArray(containerInfo))
	cardLabel := collectCardLabelValue(chip, namespaceValue, podNameValue, containerName)
	return cardLabel
}

func geenContainerInfo(chip *colcommon.HuaWeiAIChip, containerMap map[int32]container.DevicesInfo) container.DevicesInfo {
	deviceID := chip.DeviceID
	if chip.VDevActivityInfo != nil && chip.VDevActivityInfo.IsVirtualDev {
		deviceID = int32(chip.VDevActivityInfo.VDevID)
	}
	containerInfo, ok := containerMap[deviceID]
	if !ok {
		containerInfo = container.DevicesInfo{}
	}
	return containerInfo
}
func collectCardLabelValue(chip *colcommon.HuaWeiAIChip, namespaceValue, podNameValue, containerName string) []string {

	return []string{strconv.FormatInt(int64(chip.DeviceID), colcommon.Base), common.GetNpuName(chip.ChipInfo), chip.VDieID,
		chip.PCIeBusInfo, namespaceValue, podNameValue, containerName}
}

func getContainerNameArray(devInfo container.DevicesInfo) []string {
	if devInfo.Name == "" {
		return nil
	}

	return strings.Split(devInfo.Name, "_")
}

func getFieldMap(fieldsMap map[string]map[string]interface{}, devTagKey int32) map[string]interface{} {
	devTagKeyStr := strconv.Itoa(int(devTagKey))
	if fieldsMap[devTagKeyStr] == nil {
		fieldsMap[devTagKeyStr] = make(map[string]interface{})
	}
	return fieldsMap[devTagKeyStr]
}

func handleErr(err error, domain string, logicID int32) {
	if err != nil {
		logErrMetricsWithLimit(domain, logicID, err)
	} else {
		hwlog.ResetErrCnt(domain, logicID)
	}
}

func logErrMetricsWithLimit(metric string, logicID int32, err error) {
	logger.LogfWithOptions(logger.ErrorLevel, logger.LogOptions{
		Domain: metric,
		ID:     logicID},
		"logicID(%d),%v", logicID, err)
}

func validateNotNilForEveryElement(objs ...interface{}) bool {
	for _, v := range objs {
		val := reflect.ValueOf(v)
		if val.Kind() != reflect.Ptr {
			return false
		}
		if val.IsNil() {
			return false
		}
	}
	return true
}
func logForUnSupportDevice(isSupport bool, devType string, group string, extInfo string) {
	if !isSupport {
		logger.Infof("devType %v does not support [%v], %v", devType, group, extInfo)
	}
}

func updateFrame[T any](cacheKey string, n *colcommon.NpuCollector, containerMap map[int32]container.DevicesInfo,
	chips []colcommon.HuaWeiAIChip, callBack func(chipWithVnpu colcommon.HuaWeiAIChip, cache T, cardLabel []string)) {

	caches := colcommon.GetInfoFromCache[T](n, cacheKey)
	if len(caches) == 0 {
		logger.Debugf("cacheKey(%v) not found", cacheKey)
		return
	}
	for _, chip := range chips {
		cardLabel := geenGeneralCardLabel(&chip, containerMap)
		cache, ok := caches[chip.PhyId]
		if !ok {
			logger.Warnf("cacheKey(%v) not found, chip.PhyId(%v)", cacheKey, chip.PhyId)
			continue
		}

		callBack(chip, cache, cardLabel)
	}
}
