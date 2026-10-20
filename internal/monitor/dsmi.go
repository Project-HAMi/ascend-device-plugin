package monitor

import (
	"ascend-common/common-utils/hwlog"
	"ascend-common/devmanager"
	"ascend-common/devmanager/common"
	"ascend-common/devmanager/dcmi"
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/Project-HAMi/ascend-device-plugin/internal"
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
	hostMgr     devmanager.DeviceInterface
	hostMgrOnce sync.Once
	hostMgrErr  error
	dcMgr       *dcmi.DcManager
	dcMgrOnce   sync.Once
	dcMgrErr    error
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

func getHostDeviceManager() (devmanager.DeviceInterface, error) {
	hostMgrOnce.Do(func() {
		hostMgr, hostMgrErr = devmanager.AutoInit("", 30)
		if hostMgrErr != nil {
			hostMgrErr = fmt.Errorf("auto-init device manager: %w", hostMgrErr)
		}
	})
	return hostMgr, hostMgrErr
}

// ConfigureHostDeviceManager reuses the plugin's already initialized manager,
// avoiding a second DCMI initialization in the metrics goroutine.
func ConfigureHostDeviceManager(mgr devmanager.DeviceInterface) {
	hostMgrOnce.Do(func() {
		hostMgr = mgr
	})
}

func collectHostDeviceStats() ([]DeviceStat, error) {
	mgr, err := getHostDeviceManager()
	if err != nil {
		return nil, err
	}

	_, ids, err := mgr.GetDeviceList()
	if err != nil {
		return nil, fmt.Errorf("get logic ids: %w", err)
	}
	dieType := dcmi.VDIE
	if chip, chipErr := mgr.GetValidChipInfo(); chipErr == nil && internal.ChipFamily(chip.Name) != "" {
		dieType = dcmi.DDIE
	}

	var devices []DeviceStat
	for _, logicID := range ids {
		uuid, _ := mgr.GetDieID(logicID, dieType)

		product, productErr := mgr.GetProductType(logicID)
		product = strings.TrimSpace(product)
		if productErr != nil || product == "" {
			if chip, chipErr := mgr.GetChipInfo(logicID); chipErr == nil && chip != nil {
				product = strings.TrimSpace(chip.Name)
			}
		}

		memTotal := uint64(0)
		memUsed := uint64(0)
		if hbm, hbmErr := mgr.GetDeviceHbmInfo(logicID); hbmErr == nil && hbm != nil && hbm.Usage <= hbm.MemorySize {
			memTotal = hbm.MemorySize
			memUsed = hbm.Usage
		}

		aicorePct := uint32(0)
		if rate, rateErr := mgr.GetDeviceUtilizationRate(logicID, common.AICore); rateErr == nil {
			aicorePct = rate
		}

		devices = append(devices, DeviceStat{
			Index:       int(logicID),
			UUID:        uuid,
			DeviceType:  product,
			MemoryTotal: memTotal,
			MemoryUsed:  memUsed,
			AICorePct:   aicorePct,
		})
	}
	return devices, nil
}
