package monitor

import (
	"ascend-common/common-utils/hwlog"
	"ascend-common/devmanager/common"
	"ascend-common/devmanager/dcmi"
	"context"
	"fmt"
	"sync"
)

type DeviceStat struct {
	Index       int
	UUID        string
	DeviceType  string
	MemoryUsed  uint64
	MemoryTotal uint64
	AICorePct   uint32
}

var (
	dcMgr     *dcmi.DcManager
	dcMgrOnce sync.Once
	dcMgrErr  error
)

func getDcManager() (*dcmi.DcManager, error) {
	dcMgrOnce.Do(func() {
		if err := hwlog.InitRunLogger(&hwlog.LogConfig{
			OnlyToStdout: true,
			LogLevel:     1, // warning
		}, context.Background()); err != nil {
			dcMgrErr = fmt.Errorf("hwlog init: %w", err)
			return
		}

		mgr := &dcmi.DcManager{}
		if err := mgr.DcInit(); err != nil {
			dcMgrErr = fmt.Errorf("dcmi init: %w", err)
			return
		}
		dcMgr = mgr
	})
	return dcMgr, dcMgrErr
}

func collectHostDeviceStats() ([]DeviceStat, error) {
	mgr, err := getDcManager()
	if err != nil {
		return nil, err
	}

	_, ids, err := mgr.DcGetLogicIDList()
	if err != nil {
		return nil, fmt.Errorf("get logic ids: %w", err)
	}

	var devices []DeviceStat
	for _, logicID := range ids {
		cardID, deviceID, err := mgr.DcGetCardIDDeviceID(logicID)
		if err != nil {
			continue
		}

		uuid, _ := mgr.DcGetDieID(cardID, deviceID, dcmi.VDIE)

		pt, _ := mgr.DcGetProductType(cardID, deviceID)

		memTotal := uint64(0)
		memUsed := uint64(0)
		if memInfo, err := mgr.DcGetMemoryInfo(cardID, deviceID); err == nil {
			memTotal = memInfo.MemorySize
			memUsed = memInfo.MemorySize - memInfo.MemoryAvailable
		}

		aicorePct := uint32(0)
		if rate, err := mgr.DcGetDeviceUtilizationRate(cardID, deviceID, common.AICore); err == nil {
			aicorePct = uint32(rate)
		}

		devices = append(devices, DeviceStat{
			Index:       int(logicID),
			UUID:        uuid,
			DeviceType:  pt,
			MemoryTotal: memTotal,
			MemoryUsed:  memUsed,
			AICorePct:   aicorePct,
		})
	}
	return devices, nil
}
