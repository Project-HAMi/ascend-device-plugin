package monitor

import (
	"ascend-common/common-utils/hwlog"
	"ascend-common/devmanager/common"
	"ascend-common/devmanager/dcmi"
	"context"
	"fmt"
	"strings"
	"sync"
)

type DeviceStat struct {
	Index       int
	PhysicalID  int
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

		physicalID, err := mgr.DcGetPhysicIDFromLogicID(logicID)
		if err != nil {
			continue
		}
		uuid, err := mgr.DcGetDieID(cardID, deviceID, dcmi.VDIE)
		if err != nil || uuid == "" {
			continue
		}

		pt, _ := mgr.DcGetProductType(cardID, deviceID)
		if strings.TrimSpace(pt) == "" {
			if chip, err := mgr.DcGetChipInfo(cardID, deviceID); err == nil && chip != nil {
				pt = chip.Name
			}
		}

		memTotal := uint64(0)
		memUsed := uint64(0)
		// Ascend 910 reports HBM through FuncDcmiGetDeviceHbmInfo, not the DDR interface.
		if hbm, err := dcmi.FuncDcmiGetDeviceHbmInfo(cardID, deviceID); err == nil && hbm != nil && hbm.MemorySize > 0 && hbm.Usage <= hbm.MemorySize {
			memTotal = hbm.MemorySize
			memUsed = hbm.Usage
		} else if memInfo, err := mgr.DcGetMemoryInfo(cardID, deviceID); err == nil && memInfo != nil && memInfo.MemorySize > 0 && memInfo.MemoryAvailable <= memInfo.MemorySize {
			memTotal = memInfo.MemorySize
			memUsed = memInfo.MemorySize - memInfo.MemoryAvailable
		}

		aicorePct := uint32(0)
		if rate, err := mgr.DcGetDeviceUtilizationRate(cardID, deviceID, common.AICore); err == nil {
			aicorePct = uint32(rate)
		}

		devices = append(devices, DeviceStat{
			Index:       int(logicID),
			PhysicalID:  int(physicalID),
			UUID:        uuid,
			DeviceType:  pt,
			MemoryTotal: memTotal,
			MemoryUsed:  memUsed,
			AICorePct:   aicorePct,
		})
	}
	return devices, nil
}
