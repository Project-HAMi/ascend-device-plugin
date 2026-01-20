## 自定义插件开发说明

用户可参考提供的demo，或将代码拷贝到plugins目录下，重新编译部署，下面对demo中各文件进行说明

- `dcmi.go` 、`dcmi_interface_api.h`：用户自定义NPU指标的接口声明与cgo实现，用于对接驱动dcmi接口，具体可参考demo实现，全部dcmi接口续参考驱动的dcmi接口文档。
- `custom_metrics.go` 实现`MetricCollector`的接口，用于指标采集与上报，需要实现下面的接口，具体可参考demo实现：
    - Describe：prometheus上报指标前，需要先定义指标的，该接口用于prometheus的指标定义
    - CollectToCache: 指标采集方法，每个采集周期都会执行，从外部获取数据，并传入到内部缓存中
    - UpdatePrometheus: 按照prometheus的格式，将缓存中的数据返回
    - UpdateTelagraf：按照telagraf的格式，将缓存中的数据返回。
    - IsSupporterd：检测当前环境，判断是否支持当前设备的检测。
    - PreCollect：正式开始采集前执行一次，可用于设备初始化。可以为空。
    - PostCollect：采集结束后执行一次，可用于数据的回收。可以为空。
- `register.go`，提供插件注册函数，在npu-exporter启动时完成插件注册并完成dcmi接口初始化，**RegisterPlugin函数签名不要修改**，自定义插件通过`AddPluginCollector`接口注册，指标名称需要与`pluginConfiguration.json`中的指标组名称保持一致

对于插件指标组内定义的指标名称，不要与现有代码中已定义的插件指标（当前NPU指标、插件指标）重名

自定义插件采集时间超过10s后，npu-exporter会打印日志，提示插件采集时间过长，执行下一个插件采集。

### 编译部署

插件开发完后，执行Npu-exporter代码目录下的`build/build.sh`完成编译，需要提前准备go开发环境。

编译完成后，会在output目录下生成新的二进制文件与相关配置文件，根据需要打开或关闭相应开关，根据安装部署章节的安装指导，重新作镜像部署即可



`dcmi.go`

```go
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

// Package plugins this for dcmi interface
package plugins

// #cgo LDFLAGS: -ldl
/*
   #include <stddef.h>
   #include <dlfcn.h>
   #include <stdlib.h>
   #include <stdio.h>

   #include "dcmi_interface_api.h"

   static void *dcmiHandle;
   #define SO_NOT_FOUND  -99999
   #define FUNCTION_NOT_FOUND  -99998
   #define SUCCESS  0
   #define ERROR_UNKNOWN  -99997
   #define	CALL_FUNC(name,...) if(name##_func==NULL){return FUNCTION_NOT_FOUND;}return name##_func(__VA_ARGS__);

   static int (*dcmi_get_device_health_func)(int card_id, int device_id, unsigned int *health);
   int dcmi_get_device_health(int card_id, int device_id, unsigned int *health){
   	CALL_FUNC(dcmi_get_device_health,card_id,device_id,health)
   }

   // load .so files and functions
   static int dcmiLoad_dl(const char* dcmiLibPath){
   	if (dcmiLibPath == NULL) {
   	   	fprintf (stderr,"lib path is null\n");
   	   	return SO_NOT_FOUND;
   	}
   	dcmiHandle = dlopen(dcmiLibPath,RTLD_LAZY | RTLD_GLOBAL);
   	if (dcmiHandle == NULL){
   		fprintf (stderr,"%s\n",dlerror());
   		return SO_NOT_FOUND;
   	}

	dcmi_get_device_health_func = dlsym(dcmiHandle,"dcmi_get_device_health");

   	return SUCCESS;
   }

   static int dcmiShutDown(void){
   	if (dcmiHandle == NULL) {
   		return SUCCESS;
   	}
   	return (dlclose(dcmiHandle) ? ERROR_UNKNOWN : SUCCESS);
   }
*/
import "C"
import (
	"fmt"

	"unsafe"

	"ascend-common/common-utils/utils"
	"ascend-common/devmanager/common"
)

const (
	dcmiLibraryName = "libdcmi.so"
)

// DcLoad load dcmi symbol
func DcLoad() error {
	dcmiLibPath, err := utils.GetDriverLibPath(dcmiLibraryName)
	if err != nil {
		return err
	}
	cDcmiTemplateName := C.CString(dcmiLibPath)
	defer C.free(unsafe.Pointer(cDcmiTemplateName))
	if retCode := C.dcmiLoad_dl(cDcmiTemplateName); retCode != C.SUCCESS {
		return fmt.Errorf("dcmi lib load failed, error code: %d", int32(retCode))
	}
	return nil
}

// DcShutDown clean the dynamically loaded resource
func DcShutDown() error {
	if retCode := C.dcmiShutDown(); retCode != C.SUCCESS {
		return fmt.Errorf("dcmi shut down failed, error code: %d", int32(retCode))
	}

	return nil
}

// DcGetDeviceHealth get device health
func DcGetDeviceHealth(cardID, deviceID int32) (int32, error) {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return common.RetError, fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}
	var health C.uint
	if retCode := C.dcmi_get_device_health(C.int(cardID), C.int(deviceID),
		&health); int32(retCode) != common.Success {
		return common.RetError, fmt.Errorf("get device (cardID: %d, deviceID: %d) health state failed, ret "+
			"code: %d, health code: %d", cardID, deviceID, int32(retCode), int64(health))
	}
	if common.IsGreaterThanOrEqualInt32(int64(health)) {
		return common.RetError, fmt.Errorf("get wrong health state , device (cardID: %d, deviceID: %d) "+
			"health: %d", cardID, deviceID, int64(health))
	}
	return int32(health), nil
}

```



`dcmi_interface_api.h`

```c++
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

#ifndef __DCMI_INTERFACE_API_H__
#define __DCMI_INTERFACE_API_H__

#ifdef __cplusplus
#if __cplusplus
extern "C" {
#endif
#endif /* __cplusplus */

#define DCMIDLLEXPORT static

DCMIDLLEXPORT int dcmi_get_device_health(int card_id, int device_id, unsigned int *health);

#ifdef __cplusplus
#if __cplusplus
}
#endif
#endif /* __cplusplus */

#endif /* __DCMI_INTERFACE_API_H__ */
```



`custom_metrics.go`

```go
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
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"huawei.com/npu-exporter/v6/collector/common"
	"huawei.com/npu-exporter/v6/collector/container"
	"huawei.com/npu-exporter/v6/utils/logger"
)

var (
	PluginInfoDesc = prometheus.NewDesc("plugin_info", "exporter custom plugin info",
		[]string{"plugin_label"}, nil)

	PluginNpuInfoDesc = prometheus.NewDesc("npu_plugin_info", "exporter custom npu plugin info",
		[]string{"npu_plugin_label"}, nil)
)

const (
	pluginInfoKey    = "pluginInfoKey"
	pluginInfoValue  = 1.11111
	pluginLabel      = "pluginLabel"
	npuPluginLabel   = "npuPluginInfoKey"
	npuPluginInfoKey = "npuPluginInfoKey"
	pluginName       = "MyPlugin"
)

// PluginInfoCollector collect custom plugin info
type PluginInfoCollector struct {
	common.MetricsCollectorAdapter
	Cache sync.Map
}

// Describe description of the metric
func (c *PluginInfoCollector) Describe(ch chan<- *prometheus.Desc) {
	// add desc
	logger.Debug("PluginInfoCollector Describe")
	ch <- PluginInfoDesc
	ch <- PluginNpuInfoDesc
}

// CollectToCache collect the metric to cache
func (c *PluginInfoCollector) CollectToCache(n *common.NpuCollector, chipList []common.HuaWeiAIChip) {
	// collect metric to cache
	logger.Debug("PluginInfoCollector CollectToCache")
	c.Cache.Store(pluginInfoKey, pluginInfoValue)
	health, err := DcGetDeviceHealth(0, 0)
	if err != nil {
		logger.Error(err)
		return
	}
	c.Cache.Store(npuPluginInfoKey, health)
}

// UpdatePrometheus update prometheus metric
func (c *PluginInfoCollector) UpdatePrometheus(ch chan<- prometheus.Metric, n *common.NpuCollector,
	containerMap map[int32]container.DevicesInfo, chips []common.HuaWeiAIChip) {
	logger.Debug("PluginInfoCollector UpdatePrometheus")
	// get metric from cache
	pluginCache, _ := c.Cache.Load(pluginInfoKey)
	npuPluginCache, _ := c.Cache.Load(npuPluginInfoKey)
	// update plugin info
	ch <- prometheus.NewMetricWithTimestamp(time.Now(),
		prometheus.MustNewConstMetric(PluginInfoDesc, prometheus.GaugeValue, pluginCache.(float64), pluginLabel))
	// update npu plugin info
	value := float64(npuPluginCache.(int32))
	ch <- prometheus.NewMetricWithTimestamp(time.Now(),
		prometheus.MustNewConstMetric(PluginNpuInfoDesc, prometheus.GaugeValue, value, npuPluginLabel))

}

// UpdateTelegraf update telegraf metric
func (c *PluginInfoCollector) UpdateTelegraf(fieldsMap map[string]map[string]interface{}, n *common.NpuCollector,
	containerMap map[int32]container.DevicesInfo, chips []common.HuaWeiAIChip) map[string]map[string]interface{} {
	logger.Debug("PluginInfoCollector UpdateTelegraf")
	// get metric from cache
	pluginCache, _ := c.Cache.Load(pluginInfoKey)
	npuPluginCache, _ := c.Cache.Load(npuPluginInfoKey)
	// update plugin info
	if fieldsMap[common.GeneralDevTagKey] == nil {
		fieldsMap[common.GeneralDevTagKey] = make(map[string]interface{})
	}
	doUpdateTelegraf(fieldsMap[common.GeneralDevTagKey], PluginInfoDesc, pluginCache.(float64), "")
	// update npu plugin info
	const NpuLogicID = "1"
	value := float64(npuPluginCache.(int32))
	if fieldsMap[NpuLogicID] == nil {
		fieldsMap[NpuLogicID] = make(map[string]interface{})
	}
	doUpdateTelegraf(fieldsMap[NpuLogicID], PluginNpuInfoDesc, value, "")
	return fieldsMap
}

// PreCollect pre handle before collect
func (c *PluginInfoCollector) PreCollect(n *common.NpuCollector, chipList []common.HuaWeiAIChip) {
	logger.Debug("PluginInfoCollector PreCollect")
}

// PostCollect post handle after collect
func (c *PluginInfoCollector) PostCollect(n *common.NpuCollector) {
	logger.Debug("PluginInfoCollector PostCollect")
}

// IsSupported Check whether the current hardware supports this metric
func (c *PluginInfoCollector) IsSupported(n *common.NpuCollector) bool {
	logger.Debug("PluginInfoCollector IsSupported")
	return true
}

// getDescName parse metrics name from prometheus.Desc object
func getDescName(desc *prometheus.Desc) string {
	str := desc.String()
	startIndex := strings.Index(str, "fqName: ") + len("fqName: ")
	readfqName := str[startIndex:]

	endIndex := strings.Index(readfqName, ",")
	if endIndex != -1 {
		readfqName = readfqName[:endIndex]
	}

	readfqName = strings.Trim(readfqName, "\"")
	return readfqName
}

func doUpdateTelegraf(fieldMap map[string]interface{}, desc *prometheus.Desc, value interface{}, extInfo string) {
	fieldMap[getDescName(desc)+extInfo] = value
}


```



`register.go`

```go
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
	"huawei.com/npu-exporter/v6/collector/config"
	"huawei.com/npu-exporter/v6/utils/logger"
)

// RegisterPlugin register plugin collector
func RegisterPlugin() {
	err := config.AddPluginCollector(pluginName, &PluginInfoCollector{})
	if err != nil {
		logger.Errorf("add plugin failed: %v\n", err)
	}
	logger.Infof("add plugin ok: %v\n", pluginName)
	err = DcLoad()
	if err != nil {
		logger.Errorf("dcmi init failed: %v\n", err)
		return
	}
}

```

