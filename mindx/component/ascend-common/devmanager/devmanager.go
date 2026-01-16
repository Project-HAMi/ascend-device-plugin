/* Copyright(C) 2021-2023. Huawei Technologies Co.,Ltd. All rights reserved.
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

// Package devmanager this for device driver manager
package devmanager

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"ascend-common/api"
	"ascend-common/common-utils/hwlog"
	"ascend-common/devmanager/common"
	"ascend-common/devmanager/dcmi"
)

// DeviceInterface for common device interface
type DeviceInterface interface {
	Init() error
	ShutDown() error
	GetDcmiVersion() string
	GetDeviceCount() (int32, error)
	GetCardList() (int32, []int32, error)
	GetDeviceNumInCard(cardID int32) (int32, error)
	GetDeviceList() (int32, []int32, error)
	GetChipBaseInfos() ([]*common.ChipBaseInfo, error)
	GetDeviceHealth(logicID int32) (uint32, error)
	GetDeviceNetWorkHealth(logicID int32) (uint32, error)
	GetDeviceUtilizationRate(logicID int32, deviceType common.DeviceType) (uint32, error)
	GetDeviceTemperature(logicID int32) (int32, error)
	GetDeviceVoltage(logicID int32) (float32, error)
	GetDevicePowerInfo(logicID int32) (float32, error)
	GetMcuPowerInfo(cardID int32) (float32, error)
	GetDeviceFrequency(logicID int32, deviceType common.DeviceType) (uint32, error)
	GetDeviceMemoryInfo(logicID int32) (*common.MemoryInfo, error)
	GetDeviceHbmInfo(logicID int32) (*common.HbmInfo, error)
	GetDeviceErrorCode(logicID int32) (int32, int64, error)
	GetChipInfo(logicID int32) (*common.ChipInfo, error)
	GetPhysicIDFromLogicID(logicID int32) (int32, error)
	GetLogicIDFromPhysicID(physicID int32) (int32, error)
	GetDeviceLogicID(cardID, deviceID int32) (int32, error)
	GetCardIDDeviceID(logicID int32) (int32, int32, error)
	GetDeviceIPAddress(logicID, ipType int32) (string, error)
	CreateVirtualDevice(logicID int32, vDevInfo common.CgoCreateVDevRes) (common.CgoCreateVDevOut, error)
	GetVirtualDeviceInfo(logicID int32) (common.VirtualDevInfo, error)
	DestroyVirtualDevice(logicID int32, vDevID uint32) error
	GetDevType() string
	GetProductTypeArray() []string
	GetProductType(cardID, deviceID int32) (string, error)
	GetAllProductType() ([]string, error)
	GetNpuWorkMode() string
	SetDeviceReset(cardID, deviceID int32) error
	GetBrotherCardID(int32, int32) (int32, error)
	PreResetSoc(int32, int32) error
	GetOutBandChannelState(int32, int32) error
	SetDeviceResetOutBand(int32, int32) error
	RescanSoc(int32, int32) error
	GetDeviceBootStatus(logicID int32) (int, error)
	GetDeviceAllErrorCode(logicID int32) (int32, []int64, error)
	SubscribeDeviceFaultEvent(logicID int32) error
	SetFaultEventCallFunc(func(common.DevFaultInfo)) error
	GetDieID(logicID int32, dcmiDieType dcmi.DieType) (string, error)
	GetDevProcessInfo(logicID int32) (*common.DevProcessInfo, error)
	GetPCIeBusInfo(logicID int32) (string, error)
	GetBoardInfo(logicID int32) (common.BoardInfo, error)
	GetCardElabelV2(cardID int32) (common.ElabelInfo, error)
	GetPCIEBandwidth(logicID int32, profilingTime int) (common.PCIEBwStat, error)
	SetIsTrainingCard() error
	IsTrainingCard() bool
	GetValidChipInfo() (common.ChipInfo, error)
	GetDeviceEccInfo(logicID int32, dcmiDeviceType common.DcmiDeviceType) (*common.ECCInfo, error)
	GetSuperPodInfo(int32) (common.CgoSuperPodInfo, error)
	GetSioInfo(logicID int32) (*common.SioCrcErrStatisticInfo, error)
	GetHccsStatisticInfo(logicID int32) (*common.HccsStatisticInfo, error)
	GetHccsStatisticInfoInU64(logicID int32) (*common.HccsStatisticInfo, error)
	GetMainBoardId() uint32
	GetHccsBandwidthInfo(logicID int32) (*common.HccsBandwidthInfo, error)

	DcStartHccsPingMesh(int32, int32, int, common.HccspingMeshOperate) error
	DcStopHccsPingMesh(int32, int32, int, uint) error
	DcGetHccsPingMeshInfo(int32, int32, int, uint) (*common.HccspingMeshInfo, error)
	DcGetHccsPingMeshState(int32, int32, int, uint) (int, error)
	DcGetSuperPodStatus(int32, int32, uint32) (int, error)
	DcSetSuperPodStatus(int32, int32, uint32, uint32) error
}

const (
	// init dcmi interface max retry times
	maxRetries = 6
	// init dcmi interface retry delay
	defaultRetryDelay = 10
)

var (
	devManager     *DeviceManager = nil
	devManagerOnce sync.Once
	idCache        sync.Map
)

// npuIdMapping the mapping between the three IDs
type npuIdMapping struct {
	logicId  int32
	cardId   int32
	deviceId int32
}

// GetDeviceManager singleton to init global device manager and init dcmi interface
func GetDeviceManager(resetTimeout int) (*DeviceManager, error) {
	devManagerOnce.Do(func() {
		// a common dcmi Manager is initiated for init dcmi interface, you can specify an specific manager in later
		dcMgr := dcmi.DcManager{}
		var retryDelay time.Duration = defaultRetryDelay
		hwlog.RunLog.Infof("get card list from dcmi reset timeout is %d", resetTimeout)
		for currentTime, retryCount := 0, 0; currentTime <= resetTimeout; currentTime += int(retryDelay) {
			if err := dcMgr.DcInit(); err != nil {
				hwlog.RunLog.Errorf("deviceManager init failed, prepare dcmi failed, err: %v", err)
				return
			}
			cardNum, cardList, err := dcMgr.DcGetCardList()
			if err == nil && int(cardNum) == len(cardList) {
				hwlog.RunLog.Infof("deviceManager get cardList is %v, cardList length equal to cardNum: %v",
					cardList, cardNum)
				break
			}
			if diffTime := float64(resetTimeout - currentTime); diffTime > 0 {
				retryDelay = time.Duration(math.Min(float64(defaultRetryDelay), diffTime))
			}
			retryCount++
			hwlog.RunLog.Warnf("deviceManager get card list failed (attempt %d), cardNum=%d, cardList=%v, "+
				"err: %v", retryCount, cardNum, cardList, err)
			if currentTime+int(retryDelay) <= resetTimeout {
				if err = dcMgr.DcShutDown(); err != nil {
					hwlog.RunLog.Errorf("deviceManager shut down failed, err: %v", err)
					return
				}
				time.Sleep(retryDelay * time.Second)
				continue
			}
			if int(cardNum) != len(cardList) {
				hwlog.RunLog.Warnf("deviceManager get cardList is %v, but cardNum is %v, "+
					"please check whether the real number of npu matches the cardList", cardList, cardNum)
			}
		}
		devManager = &DeviceManager{}
		devManager.DcMgr = &dcMgr
		dcmiVer, err := dcMgr.DcGetDcmiVersion()
		if err != nil {
			hwlog.RunLog.Warnf("deviceManager get dcmi version failed, err: %v", err)
		}
		hwlog.RunLog.Infof("the dcmi version is %s", dcmiVer)
		devManager.dcmiVersion = dcmiVer
	})
	if devManager == nil {
		return nil, errors.New("device Manager is nil, may encounter an exception during initialization. " +
			"You can check the system log to confirm")
	}
	return devManager, nil
}

// DeviceManager common device manager for Ascend910/310P/310
type DeviceManager struct {
	// DcMgr for common dev manager
	DcMgr dcmi.DcDriverInterface
	// DevType the value is the same as the device type corresponding to the DcMgr variable.
	// Options: api.Ascend310,api.Ascend310P,api.Ascend910
	DevType string
	// ProductTypes product type in server, multi type will be in 310P mix scene
	ProductTypes []string
	// isTrainingCard whether the device is used for training
	isTrainingCard bool
	dcmiVersion    string
	// mainBoardId used to distinguish between A900A3SuperPod and A9000A3SuperPod
	mainBoardId uint32
}

// GetProductTypeArray return product types
func (d *DeviceManager) GetProductTypeArray() []string {
	return d.ProductTypes
}

// GetDevType return dev type
func (d *DeviceManager) GetDevType() string {
	return d.DevType
}

// AutoInit auto detect npu chip type and return the corresponding processing object
func AutoInit(dType string, resetTimeout int) (*DeviceManager, error) {
	chipInfo, boardInfo, err := getDeviceInfoForInit(resetTimeout)
	if err != nil {
		return nil, fmt.Errorf("auto init failed, err: %s", err)
	}
	var devMgr *DeviceManager
	if devMgr, err = GetDeviceManager(resetTimeout); err != nil || devMgr == nil {
		return nil, err
	}
	mainBoardId, err := getValidMainBoardInfo(devMgr.DcMgr)
	if err != nil {
		// Non-blocking when the main board ID is not found
		hwlog.RunLog.Warn(err)
	}
	devMgr.mainBoardId = mainBoardId
	var devType = common.GetDevType(chipInfo.Name, boardInfo.BoardId)

	switch devType {
	case api.Ascend910A, api.Ascend910B, api.Ascend910A3:
		devMgr.DcMgr = &A910Manager{}
	case api.Ascend310P:
		devMgr.DcMgr = &A310PManager{}
	case api.Ascend310, api.Ascend310B:
		devMgr.DcMgr = &A310Manager{}
	default:
		return nil, fmt.Errorf("unsupport device type (%s)", devType)
	}
	hwlog.RunLog.Infof("chipName: %v, devType: %v", chipInfo.Name, devType)
	if dType != "" && devType != dType {
		return nil, fmt.Errorf("the value of dType(%s) is inconsistent with the actual chip type(%s)",
			dType, devType)
	}
	devMgr.DevType = devType
	if err := devMgr.SetIsTrainingCard(); err != nil {
		hwlog.RunLog.Errorf("auto recognize training card failed, err: %s", err)
	}

	pTypes, err := devMgr.GetAllProductType()
	if err != nil {
		hwlog.RunLog.Debugf("auto init product types failed, err: %s", err)
	}
	devMgr.ProductTypes = pTypes
	return devMgr, nil
}

func getDeviceInfoForInit(resetTimeout int) (common.ChipInfo, common.BoardInfo, error) {
	var mgr *DeviceManager
	var err error
	if mgr, err = GetDeviceManager(resetTimeout); err != nil || mgr == nil {
		return common.ChipInfo{}, common.BoardInfo{}, fmt.Errorf("get chip info failed, err: %v", err)
	}
	dcMgr := mgr.DcMgr
	chipInfo, err := getValidChipInfo(dcMgr)
	if err != nil {
		hwlog.RunLog.Error(err)
		return common.ChipInfo{}, common.BoardInfo{}, err
	}
	boardInfo, err := getValidBoardInfo(dcMgr)
	if err != nil {
		hwlog.RunLog.Error(err)
		return chipInfo, common.BoardInfo{}, err
	}

	return chipInfo, boardInfo, nil
}

func getValidChipInfo(dcMgr dcmi.DcDriverInterface) (common.ChipInfo, error) {
	// get card list
	cardNum, cardList, err := dcMgr.DcGetCardList()
	if err != nil {
		hwlog.RunLog.Error(err)
		return common.ChipInfo{}, fmt.Errorf(common.ErrMsgInitCardListFailed)
	}
	if cardNum == 0 {
		return common.ChipInfo{}, fmt.Errorf("get chip info failed, no card found")
	}
	// get device in card, then get chip info by cardID and deviceID
	for _, cardID := range cardList {
		devNum, err := dcMgr.DcGetDeviceNumInCard(cardID)
		if err != nil || devNum == 0 {
			hwlog.RunLog.Debugf("get device num by cardID(%d) failed, error: %v", cardID, err)
			continue
		}
		for devID := int32(0); devID < devNum; devID++ {
			chipInfo, err := dcMgr.DcGetChipInfo(cardID, devID)
			if err != nil {
				hwlog.RunLog.Debugf("get chip info failed by cardID(%d), deviceID(%d), error: %v", cardID, devID,
					err)
				continue
			}
			if !common.IsValidChipInfo(chipInfo) {
				hwlog.RunLog.Debugf("invalid chip info by cardID(%d), deviceID(%d), error: %v", cardID, devID,
					err)
				continue
			}
			return *chipInfo, nil
		}
	}
	return common.ChipInfo{}, errors.New("cannot get valid chip info")
}

func getValidBoardInfo(dcMgr dcmi.DcDriverInterface) (common.BoardInfo, error) {
	// get card list
	cardNum, cardList, err := dcMgr.DcGetCardList()
	if err != nil {
		hwlog.RunLog.Error(err)
		return common.BoardInfo{}, fmt.Errorf(common.ErrMsgInitCardListFailed)
	}
	if cardNum == 0 {
		return common.BoardInfo{}, fmt.Errorf(common.ErrMsgGetBoardInfoFailed)
	}
	// get device in card, then get board info by cardID and deviceID
	for _, cardID := range cardList {
		devNum, err := dcMgr.DcGetDeviceNumInCard(cardID)
		if err != nil || devNum == 0 {
			hwlog.RunLog.Debugf("get device num by cardID %d failed, error is: %v", cardID, err)
			continue
		}
		for devID := int32(0); devID < devNum; devID++ {
			boardInfo, err := dcMgr.DcGetDeviceBoardInfo(cardID, devID)
			if err != nil {
				hwlog.RunLog.Debugf("get board info failed by cardID(%d), deviceID(%d), error: %v", cardID, devID,
					err)
				continue
			}
			if !common.IsValidBoardInfo(&boardInfo) {
				hwlog.RunLog.Debugf("invalid board info by cardID(%d), deviceID(%d), error: %v", cardID, devID,
					err)
				continue
			}
			return boardInfo, nil
		}
	}
	return common.BoardInfo{}, errors.New("cannot get valid board info")
}
func getValidMainBoardInfo(dcMgr dcmi.DcDriverInterface) (uint32, error) {
	// get card list
	cardNum, cardList, err := dcMgr.DcGetCardList()
	if err != nil {
		hwlog.RunLog.Error(err)
		return 0, fmt.Errorf(common.ErrMsgInitCardListFailed)
	}
	if cardNum == 0 {
		return 0, fmt.Errorf(common.ErrMsgGetBoardInfoFailed)
	}
	// get device in card, then get board info by cardID and deviceID
	for _, cardID := range cardList {
		devNum, err := dcMgr.DcGetDeviceNumInCard(cardID)
		if err != nil || devNum == 0 {
			hwlog.RunLog.Debugf("get device num by cardID %d failed, error is: %v", cardID, err)
			continue
		}
		for devID := int32(0); devID < devNum; devID++ {
			mainBoardId, err := dcMgr.DcGetDeviceMainBoardInfo(cardID, devID)
			if err != nil {
				hwlog.RunLog.Debug(err)
				continue
			}
			if !common.IsValidMainBoardInfo(mainBoardId) {
				hwlog.RunLog.Warnf("invalid mainBoardId info by cardID(%d), deviceID(%d), error: %v", cardID, devID, err)
				continue
			}
			return mainBoardId, nil
		}
	}
	return 0, errors.New("cannot get main board id")
}

// Init load symbol and initialize dcmi
func (d *DeviceManager) Init() error {
	return d.DcMgr.DcInit()
}

// ShutDown clean the dynamically loaded resource
func (d *DeviceManager) ShutDown() error {
	return d.DcMgr.DcShutDown()
}

// GetDeviceCount get npu device count
func (d *DeviceManager) GetDeviceCount() (int32, error) {
	return d.DcMgr.DcGetDeviceCount()
}

// GetCardList  get all card list
func (d *DeviceManager) GetCardList() (int32, []int32, error) {
	return d.DcMgr.DcGetCardList()
}

// GetDeviceNumInCard  get all device list in one card
func (d *DeviceManager) GetDeviceNumInCard(cardID int32) (int32, error) {
	return d.DcMgr.DcGetDeviceNumInCard(cardID)
}

// GetDeviceList get all device logicID list
func (d *DeviceManager) GetDeviceList() (int32, []int32, error) {
	return d.DcMgr.DcGetLogicIDList()
}

// GetDeviceHealth query npu device health status
func (d *DeviceManager) GetDeviceHealth(logicID int32) (uint32, error) {
	cardID, deviceID, err := d.getCardIdAndDeviceId(logicID)
	if err != nil {
		hwlog.RunLog.Error(err)
		return common.UnRetError, fmt.Errorf("failed to get health code by logicID(%d)", logicID)
	}
	healthCode, err := d.DcMgr.DcGetDeviceHealth(cardID, deviceID)
	if err != nil {
		hwlog.RunLog.Error(err)
		return common.UnRetError, err
	}

	return uint32(healthCode), nil
}

// GetDeviceNetWorkHealth query npu device network health status
func (d *DeviceManager) GetDeviceNetWorkHealth(logicID int32) (uint32, error) {
	cardID, deviceID, err := d.getCardIdAndDeviceId(logicID)
	if err != nil {
		hwlog.RunLog.Error(err)
		return common.UnRetError, fmt.Errorf("failed to get network health code by logicID(%d)", logicID)
	}
	healthCode, err := d.DcMgr.DcGetDeviceNetWorkHealth(cardID, deviceID)
	if err != nil {
		hwlog.RunLog.Error(err)
		return common.UnRetError, err
	}

	return healthCode, nil
}

// GetDeviceUtilizationRate get npu device utilization
func (d *DeviceManager) GetDeviceUtilizationRate(logicID int32, deviceType common.DeviceType) (uint32, error) {
	cardID, deviceID, err := d.getCardIdAndDeviceId(logicID)
	if err != nil {
		hwlog.RunLog.Error(err)
		return common.UnRetError, fmt.Errorf("failed to get utilization by logicID(%d)", logicID)
	}
	rate, err := d.DcMgr.DcGetDeviceUtilizationRate(cardID, deviceID, deviceType)
	if err != nil {
		return common.UnRetError, err
	}

	return uint32(rate), nil
}

// GetDeviceTemperature get npu device temperature
func (d *DeviceManager) GetDeviceTemperature(logicID int32) (int32, error) {
	cardID, deviceID, err := d.getCardIdAndDeviceId(logicID)
	if err != nil {
		hwlog.RunLog.Error(err)
		return common.RetError, fmt.Errorf("failed to get temperature by logicID(%d)", logicID)
	}
	temp, err := d.DcMgr.DcGetDeviceTemperature(cardID, deviceID)
	if err != nil {
		hwlog.RunLog.Error(err)
		return common.RetError, fmt.Errorf("failed to get temperature by logicID(%d)", logicID)
	}

	return temp, nil
}

// GetDeviceVoltage get npu device voltage
func (d *DeviceManager) GetDeviceVoltage(logicID int32) (float32, error) {
	cardID, deviceID, err := d.getCardIdAndDeviceId(logicID)
	if err != nil {
		hwlog.RunLog.Error(err)
		return common.UnRetError, fmt.Errorf("failed to get voltage by logicID(%d)", logicID)
	}
	voltage, err := d.DcMgr.DcGetDeviceVoltage(cardID, deviceID)
	if err != nil {
		hwlog.RunLog.Error(err)
		return common.UnRetError, fmt.Errorf("failed to get voltage by logicID(%d)", logicID)
	}

	return voltage, nil
}

// GetDevicePowerInfo get npu device power info
func (d *DeviceManager) GetDevicePowerInfo(logicID int32) (float32, error) {
	cardID, deviceID, err := d.getCardIdAndDeviceId(logicID)
	if err != nil {
		hwlog.RunLog.Error(err)
		return common.UnRetError, fmt.Errorf("failed to get power by logicID(%d)", logicID)
	}
	power, err := d.DcMgr.DcGetDevicePowerInfo(cardID, deviceID)
	if err != nil {
		hwlog.RunLog.Error(err)
		return common.UnRetError, fmt.Errorf("failed to get power by logicID(%d)", logicID)
	}

	return power, nil
}

// GetDeviceFrequency get npu device work frequency
func (d *DeviceManager) GetDeviceFrequency(logicID int32, deviceType common.DeviceType) (uint32, error) {
	cardID, deviceID, err := d.getCardIdAndDeviceId(logicID)
	if err != nil {
		hwlog.RunLog.Error(err)
		return common.UnRetError, fmt.Errorf("failed to get frequency by logicID(%d)", logicID)
	}
	frequency, err := d.DcMgr.DcGetDeviceFrequency(cardID, deviceID, deviceType)
	if err != nil {
		hwlog.RunLog.Error(err)
		return common.UnRetError, fmt.Errorf("failed to get frequency by logicID(%d)", logicID)
	}

	return frequency, nil
}

// GetDeviceMemoryInfo get npu memory information
func (d *DeviceManager) GetDeviceMemoryInfo(logicID int32) (*common.MemoryInfo, error) {
	cardID, deviceID, err := d.getCardIdAndDeviceId(logicID)
	if err != nil {
		hwlog.RunLog.Error(err)
		return nil, fmt.Errorf("failed to get memory info by logicID(%d)", logicID)
	}

	// 910B and 910A3 don't have DDR module. Therefore, DDR information cannot be queried.
	if d.DevType == api.Ascend910B || d.DevType == api.Ascend910A3 {
		hwlog.RunLog.Debugf("%v doesn't have DDR module. Therefore, DDR information cannot be queried", d.DevType)
		return nil, nil
	}

	memInfo, err := d.DcMgr.DcGetMemoryInfo(cardID, deviceID)
	if err != nil {
		hwlog.RunLog.Error(err)
		return nil, fmt.Errorf("failed to get memory info by logicID(%d)", logicID)
	}

	return memInfo, nil
}

// GetDeviceHbmInfo get npu HBM module memory and frequency information
func (d *DeviceManager) GetDeviceHbmInfo(logicID int32) (*common.HbmInfo, error) {
	cardID, deviceID, err := d.getCardIdAndDeviceId(logicID)
	if err != nil {
		hwlog.RunLog.Error(err)
		return nil, fmt.Errorf("failed to get hbm info by logicID(%d)", logicID)
	}
	hbmInfo, err := d.DcMgr.DcGetHbmInfo(cardID, deviceID)
	if err != nil {
		return nil, err
	}

	return hbmInfo, nil
}

// GetDeviceErrorCode get npu device error code
func (d *DeviceManager) GetDeviceErrorCode(logicID int32) (int32, int64, error) {
	cardID, deviceID, err := d.getCardIdAndDeviceId(logicID)
	if err != nil {
		hwlog.RunLog.Error(err)
		return common.RetError, common.RetError, fmt.Errorf("failed to get device error code by logicID(%d)",
			logicID)
	}
	errCount, errCode, err := d.DcMgr.DcGetDeviceErrorCode(cardID, deviceID)
	if err != nil {
		hwlog.RunLog.Error(err)
		return common.RetError, common.RetError, fmt.Errorf("failed to get device error code by logicID(%d)",
			logicID)
	}

	return errCount, errCode, nil
}

// GetChipInfo get npu device error code
func (d *DeviceManager) GetChipInfo(logicID int32) (*common.ChipInfo, error) {
	cardID, deviceID, err := d.getCardIdAndDeviceId(logicID)
	if err != nil {
		hwlog.RunLog.Error(err)
		return nil, fmt.Errorf("failed to get cardID and deviceID by logicID(%d), error: %v", logicID, err)
	}
	chipInfo, err := d.DcMgr.DcGetChipInfo(cardID, deviceID)
	if err != nil {
		hwlog.RunLog.Error(err)
		return nil, fmt.Errorf("failed to get chip info code by logicID(%d)", logicID)
	}

	return chipInfo, nil
}

// GetPhysicIDFromLogicID get device physic id from logic id
func (d *DeviceManager) GetPhysicIDFromLogicID(logicID int32) (int32, error) {
	physicID, err := d.DcMgr.DcGetPhysicIDFromLogicID(logicID)
	if err != nil {
		hwlog.RunLog.Error(err)
		return common.RetError, fmt.Errorf("failed to get physicID by logicID(%d)", logicID)
	}

	return physicID, nil
}

// GetLogicIDFromPhysicID get device logic id from physic id
func (d *DeviceManager) GetLogicIDFromPhysicID(physicID int32) (int32, error) {
	logicID, err := d.DcMgr.DcGetLogicIDFromPhysicID(physicID)
	if err != nil {
		hwlog.RunLog.Error(err)
		return common.RetError, fmt.Errorf("failed to get logicID by physicID(%d)", physicID)
	}

	return logicID, nil
}

// GetDeviceLogicID get device logic id from card id and device id
func (d *DeviceManager) GetDeviceLogicID(cardID, deviceID int32) (int32, error) {
	return d.DcMgr.DcGetDeviceLogicID(cardID, deviceID)
}

// GetDeviceIPAddress get device ip address
func (d *DeviceManager) GetDeviceIPAddress(logicID, ipType int32) (string, error) {
	cardID, deviceID, err := d.getCardIdAndDeviceId(logicID)
	if err != nil {
		return "", fmt.Errorf("failed to get cardID and deviceID by logicID(%d), %w", logicID, err)
	}
	return d.DcMgr.DcGetDeviceIPAddress(cardID, deviceID, ipType)
}

// CreateVirtualDevice create virtual device
func (d *DeviceManager) CreateVirtualDevice(
	logicID int32, vDevInfo common.CgoCreateVDevRes) (common.CgoCreateVDevOut, error) {
	if !common.IsValidTemplateName(d.DevType, vDevInfo.TemplateName) {
		return common.CgoCreateVDevOut{}, fmt.Errorf("input invalid template name: %s", vDevInfo.TemplateName)
	}
	return d.DcMgr.DcCreateVDevice(logicID, vDevInfo)
}

// GetVirtualDeviceInfo get virtual device info
func (d *DeviceManager) GetVirtualDeviceInfo(logicID int32) (common.VirtualDevInfo, error) {
	cgoVDevInfo, err := d.DcMgr.DcGetVDeviceInfo(logicID)
	if err != nil {
		hwlog.RunLog.Debug(err)
		return common.VirtualDevInfo{}, fmt.Errorf("get virtual device info failed, error is: %v "+
			"and vdev num is: %d", err, int32(cgoVDevInfo.TotalResource.VDevNum))
	}
	for _, vDevInfo := range cgoVDevInfo.VDevInfo {
		if !common.IsValidTemplateName(d.DevType, vDevInfo.QueryInfo.Name) {
			return common.VirtualDevInfo{}, fmt.Errorf("vdevice id %d, it's template name is invalid: %s",
				vDevInfo.VDevID, vDevInfo.QueryInfo.Name)
		}
	}
	return cgoVDevInfo, nil
}

// DestroyVirtualDevice destroy virtual device
func (d *DeviceManager) DestroyVirtualDevice(logicID int32, vDevID uint32) error {
	return d.DcMgr.DcDestroyVDevice(logicID, vDevID)
}

// GetMcuPowerInfo get mcu power info for cardID
func (d *DeviceManager) GetMcuPowerInfo(cardID int32) (float32, error) {
	return d.DcMgr.DcGetMcuPowerInfo(cardID)
}

// GetCardIDDeviceID get cardID and deviceID by logicID
func (d *DeviceManager) GetCardIDDeviceID(logicID int32) (int32, int32, error) {
	return d.getCardIdAndDeviceId(logicID)
}

// GetProductType get product type by cardID and deviceID
func (d *DeviceManager) GetProductType(cardID, deviceID int32) (string, error) {
	return d.DcMgr.DcGetProductType(cardID, deviceID)
}

// GetAllProductType get all product type
func (d *DeviceManager) GetAllProductType() ([]string, error) {
	productTypes := make([]string, 0)
	cardNum, cardList, err := d.GetCardList()
	if err != nil || cardNum == 0 {
		hwlog.RunLog.Errorf("failed to get card list, err: %v", err)
		return productTypes, err
	}
	for _, cardID := range cardList {
		devNum, err := d.GetDeviceNumInCard(cardID)
		if err != nil {
			hwlog.RunLog.Debugf("get device num by cardID(%d) failed, error: %v", cardID, err)
			continue
		}
		if devNum == 0 {
			hwlog.RunLog.Debugf("not found device on card %d", cardID)
			continue
		}
		for devID := int32(0); devID < devNum; devID++ {
			productType, err := d.GetProductType(cardID, devID)
			if err != nil {
				hwlog.RunLog.Debugf("get product type by card %d deviceID %d failed, err: %v", cardID, devID, err)
				continue
			}
			productTypes = append(productTypes, productType)
			break
		}
	}
	if len(productTypes) != 0 {
		productTypes = common.RemoveDuplicate(&productTypes)
	}
	return productTypes, nil
}

// GetNpuWorkMode get work mode of NPU
func (d *DeviceManager) GetNpuWorkMode() string {
	if d.DevType == api.Ascend910B || d.DevType == api.Ascend910A3 {
		hwlog.RunLog.Warnf("only AMP mode is available on %s", d.DevType)
		return common.AMPMode
	}

	_, cardList, err := d.DcMgr.DcGetCardList()
	if err != nil {
		hwlog.RunLog.Error(err)
		return ""
	}
	if len(cardList) > 0 {
		mode, err := d.DcMgr.DcGetNpuWorkMode(cardList[0])
		if err != nil {
			hwlog.RunLog.Error(err)
			return ""
		}
		if mode == 0 {
			return common.AMPMode
		}
		return common.SMPMode
	}
	return ""
}

// SetDeviceReset reset spec device
func (d *DeviceManager) SetDeviceReset(cardID, deviceID int32) error {
	return d.DcMgr.DcSetDeviceReset(cardID, deviceID)
}

// GetBrotherCardID get brother card id
func (d *DeviceManager) GetBrotherCardID(cardID, deviceID int32) (int32, error) {
	return d.DcMgr.DcGetBrotherCardID(cardID, deviceID)
}

// GetOutBandChannelState get out band channel state
func (d *DeviceManager) GetOutBandChannelState(cardID, deviceID int32) error {
	return d.DcMgr.DcGetOutBandChannelState(cardID, deviceID)
}

// PreResetSoc pre reset soc, used before reset out band
func (d *DeviceManager) PreResetSoc(cardID, deviceID int32) error {
	return d.DcMgr.DcPreResetSoc(cardID, deviceID)
}

// SetDeviceResetOutBand reset spec device out band
func (d *DeviceManager) SetDeviceResetOutBand(cardID, deviceID int32) error {
	return d.DcMgr.DcSetDeviceResetOutBand(cardID, deviceID)
}

// RescanSoc trigger soc rescan, non-blocking
func (d *DeviceManager) RescanSoc(cardID, deviceID int32) error {
	return d.DcMgr.DcRescanSoc(cardID, deviceID)
}

// GetDeviceBootStatus get device boot status
func (d *DeviceManager) GetDeviceBootStatus(logicID int32) (int, error) {
	return d.DcMgr.DcGetDeviceBootStatus(logicID)
}

// GetDeviceAllErrorCode get npu device all error code
func (d *DeviceManager) GetDeviceAllErrorCode(logicID int32) (int32, []int64, error) {
	cardID, deviceID, err := d.getCardIdAndDeviceId(logicID)
	if err != nil {
		hwlog.RunLog.Error(err)
		return common.RetError, nil, fmt.Errorf("failed to get cardID in get device error code by logicID(%d)",
			logicID)
	}
	errCount, errCodes, err := d.DcMgr.DcGetDeviceAllErrorCode(cardID, deviceID)
	if err != nil {
		hwlog.RunLog.Error(err)
		return common.RetError, nil, fmt.Errorf("failed to get device error code by logicID(%d)", logicID)
	}
	return errCount, errCodes, nil
}

// SubscribeDeviceFaultEvent get npu device error code by subscribe
func (d *DeviceManager) SubscribeDeviceFaultEvent(logicID int32) error {
	var cardID, deviceID int32
	if logicID == common.SubscribeAllDevice {
		cardID = common.SubscribeAllDevice
		deviceID = common.SubscribeAllDevice
	} else {
		var err error
		cardID, deviceID, err = d.getCardIdAndDeviceId(logicID)
		if err != nil {
			hwlog.RunLog.Error(err)
			return fmt.Errorf("failed to get cardID in subscribe device error code by logicID(%d)", logicID)
		}
	}
	if err := d.DcMgr.DcSubscribeDeviceFaultEvent(cardID, deviceID); err != nil {
		hwlog.RunLog.Error(err)
		return fmt.Errorf("failed to subscribe device error code by logicID(%d)", logicID)
	}
	return nil
}

// SetFaultEventCallFunc set fault event call func
func (d *DeviceManager) SetFaultEventCallFunc(businessFunc func(common.DevFaultInfo)) error {
	if businessFunc == nil {
		return errors.New("business func can't be nil")
	}
	d.DcMgr.DcSetFaultEventCallFunc(businessFunc)
	return nil
}

// GetDieID return die id by dcmi die type, vdie id or ndie id
func (d *DeviceManager) GetDieID(logicID int32, dcmiDieType dcmi.DieType) (string, error) {
	cardID, deviceID, err := d.getCardIdAndDeviceId(logicID)
	if err != nil {
		hwlog.RunLog.Error(err)
		return "", fmt.Errorf("failed to get cardID in get device error code by logicID(%d)", logicID)
	}

	return d.DcMgr.DcGetDieID(cardID, deviceID, dcmiDieType)
}

// GetDevProcessInfo get process and process memory in device side
func (d *DeviceManager) GetDevProcessInfo(logicID int32) (*common.DevProcessInfo, error) {
	cardID, deviceID, err := d.getCardIdAndDeviceId(logicID)
	if err != nil {
		hwlog.RunLog.Error(err)
		return nil, fmt.Errorf("failed to get cardID in get device error code by logicID(%d)", logicID)
	}

	return d.DcMgr.DcGetDevProcessInfo(cardID, deviceID)
}

// GetPCIeBusInfo pcie bus info
func (d *DeviceManager) GetPCIeBusInfo(logicID int32) (string, error) {
	cardID, deviceID, err := d.getCardIdAndDeviceId(logicID)
	if err != nil {
		hwlog.RunLog.Error(err)
		return "", fmt.Errorf("failed to get cardID in get device error code by logicID(%d)", logicID)
	}

	return d.DcMgr.DcGetPCIeBusInfo(cardID, deviceID)
}

// GetBoardInfo return board info of device
func (d *DeviceManager) GetBoardInfo(logicID int32) (common.BoardInfo, error) {
	cardID, deviceID, err := d.getCardIdAndDeviceId(logicID)
	if err != nil {
		hwlog.RunLog.Error(err)
		return common.BoardInfo{}, fmt.Errorf("failed to get cardID in "+
			"get device error code by logicID(%d)", logicID)
	}

	return d.DcMgr.DcGetDeviceBoardInfo(cardID, deviceID)
}

// GetCardElabelV2 get card elabel information
func (d *DeviceManager) GetCardElabelV2(cardID int32) (common.ElabelInfo, error) {
	return d.DcMgr.DcGetCardElabelV2(cardID)
}

// GetPCIEBandwidth get pcie bandwidth
func (d *DeviceManager) GetPCIEBandwidth(logicID int32, profilingTime int) (common.PCIEBwStat, error) {
	cardID, deviceID, err := d.getCardIdAndDeviceId(logicID)
	if err != nil {
		hwlog.RunLog.Error(err)
		return common.PCIEBwStat{}, fmt.Errorf("get cardID(deviceID) failed, error by logicID(%d)", logicID)
	}
	pciePCIEBw, err := d.DcMgr.DcGetPCIEBandwidth(cardID, deviceID, profilingTime)
	if err != nil {
		return common.PCIEBwStat{}, err
	}
	return pciePCIEBw, nil
}

// SetIsTrainingCard identifies whether it is a training card according to the usage of card
func (d *DeviceManager) SetIsTrainingCard() error {
	devType := d.GetDevType()
	if strings.HasPrefix(devType, api.Ascend310) {
		d.isTrainingCard = false
		return nil
	}

	boardInfo := common.BoardInfo{}
	cardNum, cardList, err := d.GetCardList()
	if err != nil || cardNum == 0 {
		hwlog.RunLog.Errorf("failed to get card list when set 'IsTrainingCard' err: %v", err)
		return err
	}
	for _, cardID := range cardList {
		devNum, err := d.GetDeviceNumInCard(cardID)
		if err != nil {
			hwlog.RunLog.Warnf("get device num by cardID(%d) failed when set 'IsTrainingCard', error: %v", cardID, err)
			continue
		}
		if devNum == 0 {
			hwlog.RunLog.Warnf("not found device on card %d when set 'IsTrainingCard'", cardID)
			continue
		}

		for devID := int32(0); devID < devNum; devID++ {
			boardInfo, err = d.DcMgr.DcGetDeviceBoardInfo(cardID, devID)
			if err != nil {
				hwlog.RunLog.Warnf("get board info by card %d deviceID %d failed, err: %v", cardID, devID, err)
				continue
			}
			break
		}
		if err == nil {
			break
		}
	}

	if devType == api.Ascend910B &&
		(boardInfo.BoardId == common.A300IA2BoardId || boardInfo.BoardId == common.A300IA2GB64BoardId) {
		d.isTrainingCard = false
		return nil
	}

	d.isTrainingCard = true
	return nil
}

// IsTrainingCard return true if it is a training card
func (d *DeviceManager) IsTrainingCard() bool {
	return d.isTrainingCard
}

// GetDcmiVersion  get dcmi version
func (d *DeviceManager) GetDcmiVersion() string {
	return d.dcmiVersion
}

// GetMainBoardId  get mainBoardId
func (d *DeviceManager) GetMainBoardId() uint32 {
	return d.mainBoardId
}

// GetValidChipInfo find a valid chip info from all cards
func (d *DeviceManager) GetValidChipInfo() (common.ChipInfo, error) {
	chipInfo, err := getValidChipInfo(d.DcMgr)
	if err != nil {
		hwlog.RunLog.Error("failed to get valid chip info")
		return common.ChipInfo{}, err
	}
	return chipInfo, nil
}

// GetDeviceEccInfo query device ECC info
func (d *DeviceManager) GetDeviceEccInfo(logicID int32, dcmiDeviceType common.DcmiDeviceType) (*common.ECCInfo, error) {
	cardID, deviceID, err := d.getCardIdAndDeviceId(logicID)
	if err != nil {
		hwlog.RunLog.Errorf("get cardID and deviceID by logicID(%d) failed, error: %v", logicID, err)
		return nil, err
	}
	return d.DcMgr.DcGetDeviceEccInfo(cardID, deviceID, dcmiDeviceType)
}

// GetSuperPodInfo  get 910A3 super pod info
func (d *DeviceManager) GetSuperPodInfo(logicID int32) (common.CgoSuperPodInfo, error) {
	if !common.IsValidLogicIDOrPhyID(logicID) {
		return common.CgoSuperPodInfo{}, fmt.Errorf("input invalid logicID: %d", logicID)
	}

	cardID, deviceID, err := d.getCardIdAndDeviceId(logicID)
	if err != nil {
		return common.CgoSuperPodInfo{}, fmt.Errorf("failed to get cardID and deviceID by logicID(%d) "+
			"when get super pod info, error: %v", logicID, err)
	}
	cgoSuperPodInfo, err := d.DcMgr.DcGetSuperPodInfo(cardID, deviceID)
	if err != nil {
		return common.CgoSuperPodInfo{}, fmt.Errorf("failed to get super pod info by logicID(%d), error: %v",
			logicID, err)
	}

	return cgoSuperPodInfo, nil
}

// GetSioInfo get SIO info
func (d *DeviceManager) GetSioInfo(logicID int32) (*common.SioCrcErrStatisticInfo, error) {
	if !common.IsValidLogicIDOrPhyID(logicID) {
		return nil, fmt.Errorf("input invalid logicID when get sio info: %d", logicID)
	}
	cardID, deviceID, err := d.getCardIdAndDeviceId(logicID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cardID and deviceID by logicID(%d) when get sio info , error: %v", logicID, err)
	}
	cgoSPodSioInfo, err := d.DcMgr.DcGetSioInfo(cardID, deviceID)
	if err != nil {
		return nil, err
	}

	return &cgoSPodSioInfo, nil
}

// GetHccsStatisticInfo get HCCS statistic info
func (d *DeviceManager) GetHccsStatisticInfo(logicID int32) (*common.HccsStatisticInfo, error) {
	if !common.IsValidLogicIDOrPhyID(logicID) {
		return buildFailedHccsInfo(), fmt.Errorf("input invalid logicID when get hccs statistic info: %d", logicID)
	}
	cardID, deviceID, err := d.getCardIdAndDeviceId(logicID)
	if err != nil {
		return buildFailedHccsInfo(), fmt.Errorf("failed to get cardID and deviceID by logicID(%d) "+
			"when get hccs statistic info, error: %v", logicID, err)
	}
	cgoHccsStatusInfo, err := d.DcMgr.DcGetHccsStatisticInfo(cardID, deviceID)
	if err != nil {
		return buildFailedHccsInfo(), err

	}

	return &cgoHccsStatusInfo, nil
}

// GetHccsStatisticInfoInU64 get hccs statistic info in u64
func (d *DeviceManager) GetHccsStatisticInfoInU64(logicID int32) (*common.HccsStatisticInfo, error) {
	if !common.IsValidLogicIDOrPhyID(logicID) {
		return buildFailedHccsInfo(), fmt.Errorf("input invalid logicID when get hccs statistic info: %d", logicID)
	}
	cardID, deviceID, err := d.getCardIdAndDeviceId(logicID)
	if err != nil {
		return buildFailedHccsInfo(), fmt.Errorf("failed to get cardID and deviceID by logicID(%d) "+
			"when get hccs statistic info, error: %v", logicID, err)
	}
	cgoHccsStatusInfo, err := d.DcMgr.DcGetHccsStatisticInfoU64(cardID, deviceID)
	if err != nil {
		return buildFailedHccsInfo(), err
	}
	return &cgoHccsStatusInfo, nil
}

// GetHccsBandwidthInfo get hccs bandwidth info
func (d *DeviceManager) GetHccsBandwidthInfo(logicID int32) (*common.HccsBandwidthInfo, error) {

	if !common.IsValidLogicIDOrPhyID(logicID) {
		return buildFailedHccsBWInfo(), fmt.Errorf("input invalid logicID when get hccs bandwidth info: %d", logicID)
	}
	cardID, deviceID, err := d.getCardIdAndDeviceId(logicID)
	if err != nil {
		return buildFailedHccsBWInfo(), fmt.Errorf("failed to get cardID and deviceID by logicID(%d) "+
			"when get hccs bandwidth info, error: %v", logicID, err)
	}
	cgoHccsBandwidthInfo, err := d.DcMgr.DcGetHccsBandwidthInfo(cardID, deviceID, common.HccsBWProfilingTime)
	if err != nil {
		return buildFailedHccsBWInfo(), fmt.Errorf("failed to get hccs bandwidth info by cardId(%d) deviceID(%d), error: %v",
			cardID, deviceID, err)
	}

	return &cgoHccsBandwidthInfo, nil
}

// buildFailedHccsInfo build failed hccs info
func buildFailedHccsInfo() *common.HccsStatisticInfo {
	errorResult := &common.HccsStatisticInfo{
		TxCnt:     make([]uint64, 8),
		RxCnt:     make([]uint64, 8),
		CrcErrCnt: make([]uint64, 8),
	}
	for i := 0; i < 8; i++ {
		errorResult.TxCnt[i] = common.FailedValue
		errorResult.RxCnt[i] = common.FailedValue
		errorResult.CrcErrCnt[i] = common.FailedValue
	}
	return errorResult
}

// buildFailedHccsBWInfo build failed hccs bandwidth info
func buildFailedHccsBWInfo() *common.HccsBandwidthInfo {
	errorResult := &common.HccsBandwidthInfo{
		ProfilingTime: uint32(common.HccsBWProfilingTime),
		TotalTxbw:     common.FailedValue,
		TotalRxbw:     common.FailedValue,
		TxBandwidth:   make([]float64, 8),
		RxBandwidth:   make([]float64, 8),
	}
	for i := 0; i < 8; i++ {
		errorResult.TxBandwidth[i] = common.FailedValue
		errorResult.RxBandwidth[i] = common.FailedValue
	}
	return errorResult
}

func (d *DeviceManager) getCardIdAndDeviceId(logicID int32) (int32, int32, error) {

	if !common.IsValidLogicIDOrPhyID(logicID) {
		return common.RetError, common.RetError, fmt.Errorf("input invalid logicID: %d", logicID)
	}

	result, ok := idCache.Load(logicID)
	if !ok {
		return d.doGetCardIDAndDeviceID(logicID)
	}
	idMapping, ok := result.(npuIdMapping)
	if !ok {
		idCache.Delete(logicID)
		return d.doGetCardIDAndDeviceID(logicID)
	}
	hwlog.RunLog.Debugf("get cardId and deviceId by logicID(%d) from cache, cardId:%v, deviceId:%v",
		logicID, idMapping.cardId, idMapping.deviceId)
	return idMapping.cardId, idMapping.deviceId, nil
}

func (d *DeviceManager) doGetCardIDAndDeviceID(logicID int32) (int32, int32, error) {
	cardId, deviceId, err := d.DcMgr.DcGetCardIDDeviceID(logicID)
	if err != nil {
		hwlog.RunLog.ErrorfWithLimit(common.DomainForLogicIdErr, logicID,
			"failed to get cardId and deviceId by logicID(%d), error: %v", logicID, err)
		return common.RetError, common.RetError, err
	}
	hwlog.ResetErrCnt(common.DomainForLogicIdErr, logicID)
	hwlog.RunLog.Debugf("get cardId and deviceId by logicID(%d) from dcmi, cardId:%v, deviceId:%v",
		logicID, cardId, deviceId)
	idCache.Store(logicID, npuIdMapping{logicId: logicID, cardId: cardId, deviceId: deviceId})
	return cardId, deviceId, nil
}

// GetChipBaseInfos get chip base info
func (d *DeviceManager) GetChipBaseInfos() ([]*common.ChipBaseInfo, error) {
	_, cardList, err := d.DcMgr.DcGetCardList()
	if err != nil {
		return nil, fmt.Errorf("get card list failed, error: %v", err)
	}
	var chips = []*common.ChipBaseInfo{}
	for _, cardID := range cardList {
		devNumInCard, err := d.DcMgr.DcGetDeviceNumInCard(cardID)
		if err != nil {
			return nil, fmt.Errorf("get device num by cardID: %d failed, error: %v",
				cardID, err)
		}
		for devID := int32(0); devID < devNumInCard; devID++ {
			logicID, err := d.DcMgr.DcGetDeviceLogicID(cardID, devID)
			if err != nil {
				return nil, fmt.Errorf("get device (cardID: %d, deviceID: %d) logic id "+
					"failed, error: %v", cardID, devID, err)
			}
			physicID, err := d.DcMgr.DcGetPhysicIDFromLogicID(logicID)
			if err != nil {
				return nil, fmt.Errorf("get device (cardID: %d, deviceID: %d) physic id "+"failed, error: %v",
					cardID, devID, err)
			}
			hwlog.RunLog.Infof("get chip base info, cardID: %d, deviceID: %d, logicID: %d, physicID: %d", cardID,
				devID, logicID, physicID)
			chips = append(chips, &common.ChipBaseInfo{
				PhysicID: physicID,
				LogicID:  logicID,
				CardID:   cardID,
				DeviceID: devID,
			})
		}
	}
	return chips, nil
}

// DcStartHccsPingMesh start hccs ping mesh
func (d *DeviceManager) DcStartHccsPingMesh(cardID int32, deviceID int32, portID int,
	operate common.HccspingMeshOperate) error {
	return d.DcMgr.DcStartHccsPingMesh(cardID, deviceID, portID, operate)
}

// DcStopHccsPingMesh stop hccs ping mesh
func (d *DeviceManager) DcStopHccsPingMesh(cardID int32, deviceID int32, portID int, taskID uint) error {
	return d.DcMgr.DcStopHccsPingMesh(cardID, deviceID, portID, taskID)
}

// DcGetHccsPingMeshInfo get hccs ping mesh info
func (d *DeviceManager) DcGetHccsPingMeshInfo(cardID int32, deviceID int32, portID int,
	taskID uint) (*common.HccspingMeshInfo, error) {
	return d.DcMgr.DcGetHccsPingMeshInfo(cardID, deviceID, portID, taskID)
}

// DcGetHccsPingMeshState get hccs ping mesh state
func (d *DeviceManager) DcGetHccsPingMeshState(cardID int32, deviceID int32, portID int, taskID uint) (int, error) {
	return d.DcMgr.DcGetHccsPingMeshState(cardID, deviceID, portID, taskID)
}

// DcGetSuperPodStatus get super pod status
func (d *DeviceManager) DcGetSuperPodStatus(cardID int32, deviceID int32, sdid uint32) (int, error) {
	var err error
	var status int
	for i := 0; i < maxRetries; i++ {
		if status, err = d.DcMgr.DcGetSuperPodStatus(cardID, deviceID, sdid); err != nil {
			hwlog.RunLog.Errorf("get super pod status failed, retry %d, cardID: %d, deviceID: %d, "+
				"sdid: %d, error: %v", i, cardID, deviceID, sdid, err)
			continue
		}
		break
	}
	return status, err
}

// DcSetSuperPodStatus set super pod status
func (d *DeviceManager) DcSetSuperPodStatus(cardID int32, deviceID int32, sdid, status uint32) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		if err = d.DcMgr.DcSetSuperPodStatus(cardID, deviceID, sdid, status); err != nil {
			hwlog.RunLog.Errorf("set super pod status failed, retry %d, cardID: %d, deviceID: %d, "+
				"sdid: %d, status: %d, error: %v", i, cardID, deviceID, sdid, status, err)
			continue
		}
		break
	}
	return err
}
