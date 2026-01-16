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

// Package config for general collector
package config

import (
	"encoding/json"
	"fmt"
	"reflect"

	"huawei.com/npu-exporter/v6/collector/common"
	"huawei.com/npu-exporter/v6/collector/metrics"
	"huawei.com/npu-exporter/v6/utils/logger"

	"ascend-common/common-utils/utils"
)

var (
	// singleGoroutineMap metrics in this map will be collected in single goroutine
	singleGoroutineMap = map[string]common.MetricsCollector{
		groupHccs:    &metrics.HccsCollector{},
		groupNpu:     &metrics.BaseInfoCollector{},
		groupSio:     &metrics.SioCollector{},
		groupVersion: &metrics.VersionCollector{},
		groupHbm:     &metrics.HbmCollector{},
		groupDDR:     &metrics.DdrCollector{},
		groupVnpu:    &metrics.VnpuCollector{},
		groupPcie:    &metrics.PcieCollector{},
	}
	// multiGoroutineMap metrics in this map will be collected in multi goroutine
	multiGoroutineMap = map[string]common.MetricsCollector{
		groupNetwork: &metrics.NetworkCollector{},
		groupRoce:    &metrics.RoceCollector{},
		groupOptical: &metrics.OpticalCollector{},
	}
	// pluginCollectorMap metrics in this map will be collected in plugin goroutine
	pluginCollectorMap = map[string]common.MetricsCollector{}
	presetConfigs      = make([]map[string]string, 0)
	pluginConfigs      = make([]map[string]string, 0)

	defaultPresetConfigs = []map[string]string{
		{metricsGroup: groupDDR, state: stateOn},
		{metricsGroup: groupHccs, state: stateOn},
		{metricsGroup: groupNpu, state: stateOn},
		{metricsGroup: groupNetwork, state: stateOn},
		{metricsGroup: groupPcie, state: stateOn},
		{metricsGroup: groupRoce, state: stateOn},
		{metricsGroup: groupSio, state: stateOn},
		{metricsGroup: groupVnpu, state: stateOn},
		{metricsGroup: groupVersion, state: stateOn},
		{metricsGroup: groupOptical, state: stateOn},
		{metricsGroup: groupHbm, state: stateOn},
	}
	defaultPluginConfigs = []map[string]string{
		{metricsGroup: groupText, state: stateOn},
	}
)

const (
	metricsGroup = "metricsGroup"
	state        = "state"

	groupDDR     = "ddr"
	groupHccs    = "hccs"
	groupNpu     = "npu"
	groupNetwork = "network"
	groupPcie    = "pcie"
	groupRoce    = "roce"
	groupSio     = "sio"
	groupVnpu    = "vnpu"
	groupVersion = "version"
	groupOptical = "optical"
	groupHbm     = "hbm"
	groupText    = "text"

	stateOn  = "ON"
	stateOFF = "OFF"
)

const (
	PresetConfigPath = "/usr/local/metricConfiguration.json"
	PluginConfigPath = "/usr/local/pluginConfiguration.json"
)

func loadConfiguration() {
	if fileBytes := loadFromFile(PresetConfigPath); fileBytes == nil {
		logger.Warnf("load config from file %s failed, use default config", PresetConfigPath)
		presetConfigs = defaultPresetConfigs
	} else {
		initConfiguration(fileBytes, &presetConfigs)
	}
	if fileBytes := loadFromFile(PluginConfigPath); fileBytes == nil {
		logger.Warnf("load config from file %s failed, use default config", PluginConfigPath)
		pluginConfigs = defaultPluginConfigs
	} else {
		initConfiguration(fileBytes, &pluginConfigs)
	}
}

func loadFromFile(filePath string) []byte {
	fileBytes, err := utils.LoadFile(filePath)
	if err != nil {
		return nil
	}
	return fileBytes
}

func initConfiguration(fileBytes []byte, configs *[]map[string]string) {
	if err := json.Unmarshal(fileBytes, configs); err != nil {
		logger.Errorf("unmarshal config byte failed: %v", err)
		return
	}
}

// AddPluginCollector add plugin collector to cache
func AddPluginCollector(name string, collector common.MetricsCollector) error {
	if _, exist := pluginCollectorMap[name]; exist {
		logger.Errorf("plugin collector %v already exist", name)
		return fmt.Errorf("plugin collector %v already exist", name)
	}
	logger.Infof("add plugin collector %v ok", name)
	pluginCollectorMap[name] = collector
	return nil
}

// DeletePluginCollector delete plugin collector from cache
func DeletePluginCollector(name string) {
	if _, exist := pluginCollectorMap[name]; !exist {
		logger.Warnf("plugin collector %v does not exist", name)
		return
	}
	logger.Infof("delete plugin collector %v ok", name)
	delete(pluginCollectorMap, name)
}

// Register register collector to cache
func Register(n *common.NpuCollector) {
	loadConfiguration()

	for _, config := range presetConfigs {
		metricsGroupName := config[metricsGroup]

		if config[state] != stateOn {
			logger.Infof("metricsGroup [%v] is off", metricsGroupName)
			continue
		}
		logger.Infof("metricsGroup [%v] is on", metricsGroupName)
		collector, exist := singleGoroutineMap[metricsGroupName]
		if exist && collector.IsSupported(n) {
			common.ChainForSingleGoroutine = append(common.ChainForSingleGoroutine, collector)
		}

		collector, exist = multiGoroutineMap[metricsGroupName]
		if exist && collector.IsSupported(n) {
			common.ChainForMultiGoroutine = append(common.ChainForMultiGoroutine, collector)
		}
	}

	for _, config := range pluginConfigs {
		metricsGroupName := config[metricsGroup]

		if config[state] != stateOn {
			logger.Infof("plugin collector [%v] is off", metricsGroupName)
			continue
		}
		logger.Infof("plugin collector [%v] is on", metricsGroupName)
		collector, exist := pluginCollectorMap[metricsGroupName]
		if exist && collector.IsSupported(n) {
			logger.Infof("add plugin collector:%v", metricsGroupName)
			common.ChainForCustomPlugin = append(common.ChainForCustomPlugin, collector)
		}

	}

	logger.Infof("ChainForSingleGoroutine:%#v", common.ChainForSingleGoroutine)
	logger.Infof("ChainForMultiGoroutine:%#v", common.ChainForMultiGoroutine)
	logger.Infof("ChainForCustomPlugin:%#v", common.ChainForCustomPlugin)
}

// UnRegister delete collector from chain
func UnRegister(worker reflect.Type) {
	logger.Debugf("unRegister collector:%v", worker)
	unRegisterChain(worker, &common.ChainForSingleGoroutine)
	unRegisterChain(worker, &common.ChainForMultiGoroutine)
	unRegisterChain(worker, &common.ChainForCustomPlugin)
}

func unRegisterChain(worker reflect.Type, chain *[]common.MetricsCollector) {
	newChain := make([]common.MetricsCollector, 0)
	for _, collector := range *chain {
		if reflect.TypeOf(collector) != worker {
			newChain = append(newChain, collector)
		}
	}
	*chain = newChain
}
