/* Copyright(C) 2025-2025. Huawei Technologies Co.,Ltd. All rights reserved.
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
	"context"
	"strings"
	"sync"
	"time"

	"ascend-common/common-utils/hwlog"
	"huawei.com/npu-exporter/v6/collector/container"
	"huawei.com/npu-exporter/v6/utils/logger"
)

// StartContainerInfoCollect start collect container info
func StartContainerInfoCollect(ctx context.Context, cancelFunc context.CancelFunc, group *sync.WaitGroup,
	n *NpuCollector) {
	group.Add(1)

	go func() {
		defer group.Done()
		retryCount := 0
		collectContainerInfo := func() {
			logger.Info("start to collect container info")
			n.devicesParser.FetchAndParse(nil)
			select {
			case result := <-n.devicesParser.RecvResult():
				if err := n.cache.Set(containersDevicesCacheKey, result, n.cacheTime); err != nil {
					logger.Error(err)
				}
				logger.Infof(UpdateCachePattern, containersDevicesCacheKey)
				retryCount = 0
			case err := <-n.devicesParser.RecvErr():
				logger.Errorf("received error from device parser: %v", err)
				if strings.Contains(err.Error(), "connection refused") {
					retryCount++
					if retryCount == connectRefusedMaxRetry {
						logger.Error("connection refused, task shutdown")
						cancelFunc()
					}
				}
			}
		}
		ticker := time.NewTicker(n.updateTime)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				logger.Info("received the stop signal,stop container info collect")
				return
			default:
				collectContainerInfo()
				if _, ok := <-ticker.C; !ok {
					logger.Errorf(tickerFailedPattern, containersDevicesCacheKey)
					return
				}
			}
		}
	}()
}

// GetContainerNPUInfo get container npu info
func GetContainerNPUInfo(n *NpuCollector) map[int32]container.DevicesInfo {
	obj, err := n.cache.Get(containersDevicesCacheKey)
	// only run once to prevent wait when container info get failed
	npuContainerInfoInit.Do(func() {
		if err != nil {
			logger.Warn("containers' devices info not found in cache, rebuilding")
			resultChan := make(chan container.DevicesInfos, 1)
			n.devicesParser.FetchAndParse(resultChan)
			select {
			case obj = <-resultChan:
			case <-time.After(time.Second):
				logger.Warn("rebuild container info cache timeout")
				return
			}
			logger.Info("rebuild cache successfully")
		}
	})
	cntNpuInfos, ok := obj.(container.DevicesInfos)
	if !ok {
		logger.LogfWithOptions(logger.ErrorLevel, logger.LogOptions{Domain: DomainForContainerInfo, ID: 0},
			"error container npu info cache and convert failed")
		return nil
	}
	hwlog.ResetErrCnt(DomainForContainerInfo, 0)
	res := make(map[int32]container.DevicesInfo, initSize)
	for _, v := range cntNpuInfos {
		for _, deviceID := range v.Devices {
			res[int32(deviceID)] = v
		}
	}
	return res
}
