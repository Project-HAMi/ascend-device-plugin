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

// Package metrics for general collector
package metrics

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/smartystreets/goconvey/convey"

	"ascend-common/api"
	"ascend-common/common-utils/hwlog"
	"ascend-common/devmanager"
	"ascend-common/devmanager/common"
	"ascend-common/devmanager/hccn"
	colcommon "huawei.com/npu-exporter/v6/collector/common"
	"huawei.com/npu-exporter/v6/collector/container"
	"huawei.com/npu-exporter/v6/utils/logger"
)

const (
	maxMetricsCount         = 2000
	num5                    = 5
	mockContainerName       = "mockContainerName"
	maxChipNum        int32 = 8
)

var (
	collectorChain []colcommon.MetricsCollector
)

// TestDescribe test Describe
func TestDescribe(t *testing.T) {

	convey.Convey("test prometheus desc ", t, func() {
		ch := make(chan *prometheus.Desc, maxMetricsCount)
		for _, c := range collectorChain {
			c.Describe(ch)
		}
		t.Logf("Describe len(ch):%v", len(ch))
		convey.So(ch, convey.ShouldNotBeEmpty)
	})
}

type testCase struct {
	name          string
	collectorType colcommon.MetricsCollector
	deviceType    string
	expectValue   bool
}

func buildTestCase(name string, collectorType colcommon.MetricsCollector, deviceType string,
	expectValue bool) testCase {
	return testCase{
		name:          name,
		collectorType: collectorType,
		deviceType:    deviceType,
		expectValue:   expectValue,
	}
}

// testIsSupported test IsSupported
func TestIsSupported(t *testing.T) {
	n := mockNewNpuCollector()
	cases := []testCase{
		buildTestCase("DdrCollector: testIsSupported on Ascend310", &DdrCollector{}, api.Ascend310, true),
		buildTestCase("DdrCollector: testIsSupported on Ascend310P", &DdrCollector{}, api.Ascend310P, true),
		buildTestCase("DdrCollector: testIsSupported on Ascend910", &DdrCollector{}, api.Ascend910, true),
		buildTestCase("DdrCollector: testIsSupported on Ascend910B", &DdrCollector{}, api.Ascend910B, false),
		buildTestCase("DdrCollector: testIsSupported on Ascend910A3", &DdrCollector{}, api.Ascend910A3, false),

		buildTestCase("HccsCollector: testIsSupported on Ascend310", &HccsCollector{}, api.Ascend310, false),
		buildTestCase("HccsCollector: testIsSupported on Ascend310P", &HccsCollector{}, api.Ascend310P, false),
		buildTestCase("HccsCollector: testIsSupported on Ascend910", &HccsCollector{}, api.Ascend910, false),
		buildTestCase("HccsCollector: testIsSupported on Ascend910B", &HccsCollector{}, api.Ascend910B, true),
		buildTestCase("HccsCollector: testIsSupported on Ascend910A3", &HccsCollector{}, api.Ascend910A3, true),

		buildTestCase("SioCollector: testIsSupported on Ascend310", &SioCollector{}, api.Ascend310, false),
		buildTestCase("SioCollector: testIsSupported on Ascend310P", &SioCollector{}, api.Ascend310P, false),
		buildTestCase("SioCollector: testIsSupported on Ascend910", &SioCollector{}, api.Ascend910, false),
		buildTestCase("SioCollector: testIsSupported on Ascend910B", &SioCollector{}, api.Ascend910B, false),
		buildTestCase("SioCollector: testIsSupported on Ascend910A3", &SioCollector{}, api.Ascend910A3, true),

		buildTestCase("VnpuCollector: testIsSupported on Ascend310", &VnpuCollector{}, api.Ascend310, false),
		buildTestCase("VnpuCollector: testIsSupported on Ascend310P", &VnpuCollector{}, api.Ascend310P, true),
		buildTestCase("VnpuCollector: testIsSupported on Ascend910", &VnpuCollector{}, api.Ascend910, false),
		buildTestCase("VnpuCollector: testIsSupported on Ascend910B", &VnpuCollector{}, api.Ascend910B, false),
		buildTestCase("VnpuCollector: testIsSupported on Ascend910A3", &VnpuCollector{}, api.Ascend910A3, false),
	}

	for _, c := range cases {
		patches := gomonkey.NewPatches()
		convey.Convey(c.name, t, func() {
			defer patches.Reset()
			patches.ApplyMethodReturn(n.Dmgr, "GetDevType", c.deviceType)
			isSupported := c.collectorType.IsSupported(n)
			convey.So(isSupported, convey.ShouldEqual, c.expectValue)
		})
	}
}

// TestIsSupported2 test IsSupported
func TestIsSupported2(t *testing.T) {
	n := mockNewNpuCollector()
	convey.Convey("TestIsSupported ", t, func() {
		for _, c := range collectorChain {
			c.IsSupported(n)
		}
	})

}

// TestCollectToCache test CollectToCache
func TestCollectToCache(t *testing.T) {
	n := mockNewNpuCollector()

	convey.Convey("TestCollectToCache", t, func() {

		patches := gomonkey.NewPatches()
		defer patches.Reset()
		patches.ApplyMethodReturn(n.Dmgr, "GetDeviceMemoryInfo", mockMemoryInfo(), nil)
		patches.ApplyMethodReturn(n.Dmgr, "GetDeviceHbmInfo", mockHbmAggregateInfo().HbmInfo, nil)
		patches.ApplyMethodReturn(n.Dmgr, "GetDeviceEccInfo", mockHbmAggregateInfo().ECCInfo, nil)
		patches.ApplyMethodReturn(n.Dmgr, "GetHccsStatisticInfo", mockHccsStaticsInfo(), nil)
		patches.ApplyMethodReturn(n.Dmgr, "GetHccsStatisticInfoInU64", mockHccsStaticsInfo(), nil)
		patches.ApplyMethodReturn(n.Dmgr, "GetHccsBandwidthInfo", mockHccsBWInfo(), nil)
		patches.ApplyMethodReturn(n.Dmgr, "GetPCIEBandwidth", mockPcieInfo(), nil)
		patches.ApplyMethodReturn(n.Dmgr, "GetSioInfo", mockSioInfo(), nil)
		patches.ApplyFuncReturn(hccn.GetNPULinkStatus, "UP", nil)
		patches.ApplyFuncReturn(hccn.GetNPUInterfaceTraffic, float64(0), float64(0), nil)
		patches.ApplyFuncReturn(hccn.GetNPULinkUpNum, 0, nil)
		patches.ApplyFuncReturn(hccn.GetNPULinkSpeed, 0, nil)
		patches.ApplyFuncReturn(hccn.GetNPUOpticalInfo, mockOpticalInfo(), nil)
		patches.ApplyFuncReturn(hccn.GetNPUStatInfo, mockRoceInfoMap(), nil)
		patches.ApplyMethodReturn(n.Dmgr, "GetDeviceFrequency", uint32(0), nil)
		patches.ApplyMethodReturn(n.Dmgr, "GetDeviceTemperature", int32(0), nil)
		patches.ApplyMethodReturn(n.Dmgr, "GetDeviceVoltage", float32(0), nil)
		patches.ApplyMethodReturn(n.Dmgr, "GetDeviceAllErrorCode", int32(1), []int64{0}, nil)
		patches.ApplyMethodReturn(n.Dmgr, "GetDeviceHealth", uint32(0), nil)
		patches.ApplyMethodReturn(n.Dmgr, "GetDevicePowerInfo", float32(0), nil)
		patches.ApplyMethodReturn(n.Dmgr, "GetDeviceUtilizationRate", uint32(0), nil)
		patches.ApplyMethodReturn(n.Dmgr, "GetDevProcessInfo", mockProcessInfo(), nil)

		chips := mockGetNPUChipList()
		for _, c := range collectorChain {
			c.PreCollect(n, chips)
			c.CollectToCache(n, chips)
		}

		convey.So(colcommon.GetInfoFromCache[ddrCache](n, colcommon.GetCacheKey(&DdrCollector{})),
			convey.ShouldNotBeEmpty)
		convey.So(colcommon.GetInfoFromCache[hbmCache](n, colcommon.GetCacheKey(&HbmCollector{})),
			convey.ShouldNotBeEmpty)
		convey.So(colcommon.GetInfoFromCache[hccsCache](n, colcommon.GetCacheKey(&HccsCollector{})),
			convey.ShouldNotBeEmpty)
		convey.So(colcommon.GetInfoFromCache[netInfoCache](n, colcommon.GetCacheKey(&NetworkCollector{})),
			convey.ShouldNotBeEmpty)
		convey.So(colcommon.GetInfoFromCache[chipCache](n, colcommon.GetCacheKey(&BaseInfoCollector{})),
			convey.ShouldNotBeEmpty)
		convey.So(colcommon.GetInfoFromCache[opticalCache](n, colcommon.GetCacheKey(&OpticalCollector{})),
			convey.ShouldNotBeEmpty)
		convey.So(colcommon.GetInfoFromCache[pcieCache](n, colcommon.GetCacheKey(&PcieCollector{})),
			convey.ShouldNotBeEmpty)
		convey.So(colcommon.GetInfoFromCache[roceCache](n, colcommon.GetCacheKey(&RoceCollector{})),
			convey.ShouldNotBeEmpty)
		convey.So(colcommon.GetInfoFromCache[sioCache](n, colcommon.GetCacheKey(&SioCollector{})),
			convey.ShouldNotBeEmpty)

	})
}

// TestUpdatePrometheus test UpdatePrometheus
func TestUpdatePrometheus(t *testing.T) {
	n := mockNewNpuCollector()

	convey.Convey("TestUpdatePrometheus", t, func() {

		ch := make(chan prometheus.Metric, maxMetricsCount)

		patches := gomonkey.NewPatches()
		defer patches.Reset()
		containerInfos := mockGetContainerNPUInfo()
		chips := mockGetNPUChipList()

		mockDdrCache(n, chips, colcommon.GetCacheKey(&DdrCollector{}))
		mockHbmCache(n, chips, colcommon.GetCacheKey(&HbmCollector{}))
		mockHccsCache(n, chips, colcommon.GetCacheKey(&HccsCollector{}))
		mockNetInfoCache(n, chips, colcommon.GetCacheKey(&NetworkCollector{}))
		mockChipCache(n, chips, colcommon.GetCacheKey(&BaseInfoCollector{}))
		mockOpticalCache(n, chips, colcommon.GetCacheKey(&OpticalCollector{}))
		mockPcieCache(n, chips, colcommon.GetCacheKey(&PcieCollector{}))
		mockRoceCache(n, chips, colcommon.GetCacheKey(&RoceCollector{}))
		mockSioCache(n, chips, colcommon.GetCacheKey(&SioCollector{}))

		for _, c := range collectorChain {
			c.UpdatePrometheus(ch, n, containerInfos, chips)
		}

		t.Logf("TestUpdatePrometheus len(ch):%v", len(ch))
		convey.So(ch, convey.ShouldNotBeEmpty)
	})
}

// TestUpdateTelegraf test UpdateTelegraf
func TestUpdateTelegraf(t *testing.T) {
	n := mockNewNpuCollector()

	convey.Convey("TestUpdatePrometheus", t, func() {

		patches := gomonkey.NewPatches()
		defer patches.Reset()
		containerInfos := mockGetContainerNPUInfo()
		chips := mockGetNPUChipList()

		mockDdrCache(n, chips, colcommon.GetCacheKey(&DdrCollector{}))
		mockHbmCache(n, chips, colcommon.GetCacheKey(&HbmCollector{}))
		mockHccsCache(n, chips, colcommon.GetCacheKey(&HccsCollector{}))
		mockNetInfoCache(n, chips, colcommon.GetCacheKey(&NetworkCollector{}))
		mockChipCache(n, chips, colcommon.GetCacheKey(&BaseInfoCollector{}))
		mockOpticalCache(n, chips, colcommon.GetCacheKey(&OpticalCollector{}))
		mockPcieCache(n, chips, colcommon.GetCacheKey(&PcieCollector{}))
		mockRoceCache(n, chips, colcommon.GetCacheKey(&RoceCollector{}))
		mockSioCache(n, chips, colcommon.GetCacheKey(&SioCollector{}))
		fieldsMap := make(map[string]map[string]interface{})

		for _, c := range collectorChain {
			c.UpdateTelegraf(fieldsMap, n, containerInfos, chips)
		}

		t.Logf("fieldsMap len(ch):%v", len(fieldsMap))
		convey.So(fieldsMap, convey.ShouldNotBeEmpty)
	})
}

func mockRoceCache(n *colcommon.NpuCollector, chips []colcommon.HuaWeiAIChip, cacheKey string) {
	localCache := sync.Map{}
	for _, chip := range chips {
		localCache.Store(chip.PhyId, roceCache{chip: chip, timestamp: time.Now(),
			extInfo: getMainStatInfo(mockRoceInfoMap())})
	}
	colcommon.UpdateCache[roceCache](n, cacheKey, &localCache)
}

func mockRoceInfoMap() map[string]int {
	return map[string]int{
		macRxMacPauseNum:       0,
		macTxMacPauseNum:       0,
		macRxPfcPktNum:         0,
		macTxPfcPktNum:         0,
		macRxBadPktNum:         0,
		macTxBadPktNum:         0,
		roCERxAllPktNum:        0,
		roCETxAllPktNum:        0,
		roCERxErrPktNum:        0,
		roCETxErrPktNum:        0,
		roCERxCnpPktNum:        0,
		roCETxCnpPktNum:        0,
		macRxBadOctNum:         0,
		macTxBadOctNum:         0,
		roCEUnexpectedAckNum:   0,
		roCEOutOfOrderNum:      0,
		roCEVerificationErrNum: 0,
		roCEQpStatusErrNum:     0,
		roCENewPktRtyNum:       0,
		roCEEcnDBNum:           0,
		macRXFcsErrPktNum:      0,
	}
}

func mockDdrCache(n *colcommon.NpuCollector, chips []colcommon.HuaWeiAIChip, cacheKey string) {
	localCache := sync.Map{}
	for _, chip := range chips {
		localCache.Store(chip.PhyId, ddrCache{chip: chip, timestamp: time.Now(), extInfo: mockMemoryInfo()})
	}
	colcommon.UpdateCache[ddrCache](n, cacheKey, &localCache)
}

func mockHccsCache(n *colcommon.NpuCollector, chips []colcommon.HuaWeiAIChip, cacheKey string) {
	localCache := sync.Map{}
	for _, chip := range chips {
		localCache.Store(chip.PhyId, hccsCache{chip: chip, timestamp: time.Now(),
			hccsStat: mockHccsStaticsInfo(), hccsBW: mockHccsBWInfo()})
	}
	colcommon.UpdateCache[hccsCache](n, cacheKey, &localCache)
}

func mockHccsBWInfo() *common.HccsBandwidthInfo {
	return &common.HccsBandwidthInfo{
		ProfilingTime: 0,
		RxBandwidth:   []float64{0, 0, 0, 0, 0, 0, 0, 0},
		TxBandwidth:   []float64{0, 0, 0, 0, 0, 0, 0, 0},
		TotalRxbw:     0,
		TotalTxbw:     0,
	}
}

func mockHccsStaticsInfo() *common.HccsStatisticInfo {
	return &common.HccsStatisticInfo{
		TxCnt:     []uint64{0, 0, 0, 0, 0, 0, 0, 0},
		RxCnt:     []uint64{0, 0, 0, 0, 0, 0, 0, 0},
		CrcErrCnt: []uint64{0, 0, 0, 0, 0, 0, 0, 0},
	}
}

func mockSioCache(n *colcommon.NpuCollector, chips []colcommon.HuaWeiAIChip, cacheKey string) {
	localCache := sync.Map{}
	for _, chip := range chips {
		localCache.Store(chip.PhyId, sioCache{chip: chip, timestamp: time.Now(), extInfo: mockSioInfo()})
	}
	colcommon.UpdateCache[sioCache](n, cacheKey, &localCache)
}

func mockSioInfo() *common.SioCrcErrStatisticInfo {
	return &common.SioCrcErrStatisticInfo{
		TxErrCnt: 0,
		RxErrCnt: 0,
	}
}
func mockPcieCache(n *colcommon.NpuCollector, chips []colcommon.HuaWeiAIChip, cacheKey string) {
	localCache := sync.Map{}
	for _, chip := range chips {
		pcieInfo := mockPcieInfo()
		localCache.Store(chip.PhyId, pcieCache{chip: chip, timestamp: time.Now(), extInfo: &pcieInfo})
	}
	colcommon.UpdateCache[pcieCache](n, cacheKey, &localCache)
}

func mockPcieInfo() common.PCIEBwStat {
	return common.PCIEBwStat{
		PcieRxPBw:   common.PcieStatValue{PcieMinBw: int32(0), PcieMaxBw: int32(0), PcieAvgBw: int32(0)},
		PcieRxNPBw:  common.PcieStatValue{PcieMinBw: int32(0), PcieMaxBw: int32(0), PcieAvgBw: int32(0)},
		PcieRxCPLBw: common.PcieStatValue{PcieMinBw: int32(0), PcieMaxBw: int32(0), PcieAvgBw: int32(0)},
		PcieTxPBw:   common.PcieStatValue{PcieMinBw: int32(0), PcieMaxBw: int32(0), PcieAvgBw: int32(0)},
		PcieTxNPBw:  common.PcieStatValue{PcieMinBw: int32(0), PcieMaxBw: int32(0), PcieAvgBw: int32(0)},
		PcieTxCPLBw: common.PcieStatValue{PcieMinBw: int32(0), PcieMaxBw: int32(0), PcieAvgBw: int32(0)},
	}
}

func mockOpticalCache(n *colcommon.NpuCollector, chips []colcommon.HuaWeiAIChip, cacheKey string) {
	localCache := sync.Map{}
	for _, chip := range chips {
		localCache.Store(chip.PhyId, opticalCache{chip: chip, timestamp: time.Now(),
			extInfo: getMainOptInfo(mockOpticalInfo())})
	}
	colcommon.UpdateCache[opticalCache](n, cacheKey, &localCache)
}

func mockOpticalInfo() map[string]string {
	return map[string]string{
		txPower0:    "1 mW",
		txPower1:    "1 mW",
		txPower2:    "1 mW",
		txPower3:    "1 mW",
		rxPower0:    "1 mW",
		rxPower1:    "1 mW",
		rxPower2:    "1 mW",
		rxPower3:    "1 mW",
		voltage:     "1 mV",
		temperature: "50 C",
		present:     "1.0",
	}
}

func mockHbmCache(n *colcommon.NpuCollector, chips []colcommon.HuaWeiAIChip, cacheKey string) {
	localCache := sync.Map{}
	for _, chip := range chips {
		localCache.Store(chip.PhyId, hbmCache{chip: chip, timestamp: time.Now(), extInfo: mockHbmAggregateInfo(),
			hbmUtilization: 0},
		)
	}
	colcommon.UpdateCache[hbmCache](n, cacheKey, &localCache)
}

func mockNetInfoCache(n *colcommon.NpuCollector, chips []colcommon.HuaWeiAIChip, cacheKey string) {
	localCache := sync.Map{}
	for _, chip := range chips {
		localCache.Store(chip.PhyId, netInfoCache{chip: chip, timestamp: time.Now(), extInfo: mockNetInfo()})
	}
	colcommon.UpdateCache[netInfoCache](n, cacheKey, &localCache)
}

func mockNetInfo() *common.NpuNetInfo {
	return &common.NpuNetInfo{
		LinkStatusInfo: &common.LinkStatusInfo{LinkState: "0"},
		BandwidthInfo:  &common.BandwidthInfo{RxValue: 0, TxValue: 0},
		LinkStatInfo:   &common.LinkStatInfo{LinkUPNum: 0},
		LinkSpeedInfo:  &common.LinkSpeedInfo{Speed: 0},
	}
}

func mockChipCache(n *colcommon.NpuCollector, chips []colcommon.HuaWeiAIChip, cacheKey string) {
	localCache := sync.Map{}
	for _, chip := range chips {
		localCache.Store(chip.PhyId, chipCache{chip: chip, timestamp: time.Now(),
			HealthStatus:       "Healthy",
			ErrorCodes:         []int64{0},
			Utilization:        0,
			OverallUtilization: 0,
			VectorUtilization:  0,
			Temperature:        0,
			Power:              0,
			Voltage:            0,
			AICoreCurrentFreq:  0,
			NetHealthStatus:    "Healthy",
			DevProcessInfo:     mockProcessInfo(),
		})
	}
	colcommon.UpdateCache[chipCache](n, cacheKey, &localCache)
}

func mockProcessInfo() *common.DevProcessInfo {
	return &common.DevProcessInfo{
		ProcNum:      1,
		DevProcArray: []common.DevProcInfo{{Pid: 0, MemUsage: 0}},
	}
}

func mockMemoryInfo() *common.MemoryInfo {
	return &common.MemoryInfo{
		MemorySize:      0,
		MemoryAvailable: 0,
		Frequency:       0,
		Utilization:     0,
	}
}

func mockHbmAggregateInfo() *common.HbmAggregateInfo {
	return &common.HbmAggregateInfo{
		HbmInfo: &common.HbmInfo{
			MemorySize:        1,
			Frequency:         1,
			Usage:             1,
			Temp:              1,
			BandWidthUtilRate: 1,
		},
		ECCInfo: &common.ECCInfo{
			EnableFlag: 1,
		},
	}
}

func mockNewNpuCollector() *colcommon.NpuCollector {
	tc := newNpuCollectorTestCase{
		cacheTime:    time.Duration(num5) * time.Second,
		updateTime:   time.Duration(num5) * time.Second,
		deviceParser: &container.DevicesParser{},
		dmgr:         &devmanager.DeviceManager{},
	}
	c := colcommon.NewNpuCollector(tc.cacheTime, tc.updateTime, tc.deviceParser, tc.dmgr)
	return c
}

type newNpuCollectorTestCase struct {
	cacheTime    time.Duration
	updateTime   time.Duration
	deviceParser *container.DevicesParser
	dmgr         *devmanager.DeviceManager
}

func mockGetNPUChipList() []colcommon.HuaWeiAIChip {
	chips := make([]colcommon.HuaWeiAIChip, 0)
	for id := int32(0); id < maxChipNum; id++ {
		chip := colcommon.HuaWeiAIChip{
			CardId:   id,
			PhyId:    id,
			DeviceID: id,
			LogicID:  id,
			ChipInfo: &common.ChipInfo{
				Name:    api.Ascend910,
				Type:    "Ascend",
				Version: "V1",
			},
		}

		chips = append(chips, chip)
	}
	return chips
}

func mockGetContainerNPUInfo() map[int32]container.DevicesInfo {
	containsInfo := make(map[int32]container.DevicesInfo)
	for id := int32(0); id < maxChipNum; id++ {

		containerInfo := container.DevicesInfo{
			ID:      strconv.Itoa(int(id)),
			Name:    mockContainerName,
			Devices: []int{int(id)},
		}
		containsInfo[id] = containerInfo
	}
	return containsInfo
}

func init() {
	logger.HwLogConfig = &hwlog.LogConfig{
		OnlyToStdout: true,
	}
	logger.InitLogger("Prometheus")

	initChain()
}

func initChain() {
	collectorChain = []colcommon.MetricsCollector{
		&HccsCollector{},
		&BaseInfoCollector{},
		&SioCollector{},
		&VersionCollector{},
		&HbmCollector{},
		&DdrCollector{},
		&VnpuCollector{},
		&PcieCollector{},
		&NetworkCollector{},
		&RoceCollector{},
		&OpticalCollector{},
	}
}

func createChip() colcommon.HuaWeiAIChip {
	return colcommon.HuaWeiAIChip{
		CardId:   0,
		PhyId:    0,
		DeviceID: 0,
		LogicID:  0,
		ChipInfo: &common.ChipInfo{
			Name:    api.Ascend910,
			Type:    "Ascend",
			Version: "V1",
		},
	}
}
