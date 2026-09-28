package monitor

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"ascend-common/devmanager/common"
	"ascend-common/devmanager/dcmi"
)

const enpuHostProc = "/host/proc"

func collectENPUDeviceStats(allocations []enpuAllocation) ([]enpuDeviceSample, error) {
	mgr, err := getDcManager()
	if err != nil {
		return nil, err
	}
	return readENPUDeviceStats(mgr, dcmi.FuncDcmiGetDeviceHbmInfo, enpuHostProc, allocations)
}

func readENPUDeviceStats(mgr dcmi.DcDriverInterface, hbm func(int32, int32) (*common.HbmInfo, error), procRoot string, allocations []enpuAllocation) ([]enpuDeviceSample, error) {
	_, ids, err := mgr.DcGetLogicIDList()
	if err != nil {
		return nil, err
	}
	needed := make(map[int]bool)
	for _, allocation := range allocations {
		needed[allocation.PhysicalID] = true
	}
	var starts map[int32]string
	var procErr error
	if len(needed) > 0 {
		starts, procErr = enpuProcessStarts(procRoot)
	}
	var result []enpuDeviceSample
	var errs []error
	for _, id := range ids {
		card, device, err := mgr.DcGetCardIDDeviceID(id)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		physical, err := mgr.DcGetPhysicIDFromLogicID(id)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		uuid, err := mgr.DcGetDieID(card, device, dcmi.VDIE)
		if err != nil || uuid == "" {
			errs = append(errs, fmt.Errorf("DIE %d identity: %v", physical, err))
			continue
		}
		product, productErr := mgr.DcGetProductType(card, device)
		product = strings.TrimSpace(product)
		if productErr != nil || product == "" {
			product = ""
			chip, chipErr := mgr.DcGetChipInfo(card, device)
			if chipErr == nil && chip != nil {
				product = strings.TrimSpace(chip.Name)
			}
			if product == "" {
				errs = append(errs, fmt.Errorf("DIE %d device type unavailable: product error %v, chip error %v", physical, productErr, chipErr))
			}
		}
		sample := enpuDeviceSample{LogicID: int(id), PhysicalID: int(physical), UUID: uuid, DeviceType: product}
		if info, err := hbm(card, device); err == nil && info != nil && info.MemorySize > 0 && info.Usage <= info.MemorySize {
			value := float64(info.Usage) * common.UnitMB
			sample.MemoryUsed = &value
		} else {
			errs = append(errs, fmt.Errorf("DIE %d HBM: %v", physical, err))
		}
		if utilization, err := mgr.DcGetDeviceUtilizationRate(card, device, common.AICore); err == nil && utilization >= 0 && utilization <= 100 {
			value := float64(utilization)
			sample.Utilization = &value
		} else {
			errs = append(errs, fmt.Errorf("DIE %d utilization: %v", physical, err))
		}
		if needed[int(physical)] {
			err := procErr
			if err == nil {
				var info *common.DevProcessInfo
				info, err = mgr.DcGetDevProcessInfo(card, device)
				if err == nil {
					sample.MemoryByContainer, err = enpuContainerMemory(info, procRoot, starts)
				}
			}
			sample.MemoryErr = err
			if err != nil {
				errs = append(errs, fmt.Errorf("DIE %d process memory: %w", physical, err))
			}
		}
		result = append(result, sample)
	}
	return result, errors.Join(errs...)
}

func enpuProcessStart(procRoot string, pid int32) (string, error) {
	data, err := os.ReadFile(filepath.Join(procRoot, strconv.Itoa(int(pid)), "stat"))
	if err != nil {
		return "", err
	}
	end := strings.LastIndexByte(string(data), ')')
	if end < 0 {
		return "", fmt.Errorf("invalid process stat for PID %d", pid)
	}
	fields := strings.Fields(string(data[end+1:]))
	if len(fields) <= 19 {
		return "", fmt.Errorf("short process stat for PID %d", pid)
	}
	return fields[19], nil
}

func enpuProcessStarts(procRoot string) (map[int32]string, error) {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil, err
	}
	starts := make(map[int32]string)
	for _, entry := range entries {
		pid, err := strconv.ParseInt(entry.Name(), 10, 32)
		if err != nil || pid <= 0 {
			continue
		}
		if start, err := enpuProcessStart(procRoot, int32(pid)); err == nil {
			starts[int32(pid)] = start
		}
	}
	return starts, nil
}

func enpuProcessMissing(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ESRCH)
}

// enpuProcessGone requires a missing PID directory and an accessible proc view.
func enpuProcessGone(procRoot string, pid int32) bool {
	_, err := os.Stat(filepath.Join(procRoot, strconv.Itoa(int(pid))))
	if !enpuProcessMissing(err) {
		return false
	}
	data, err := os.ReadFile(filepath.Join(procRoot, "self", "stat"))
	return err == nil && len(data) > 0
}

func enpuContainerMemory(info *common.DevProcessInfo, procRoot string, starts map[int32]string) (map[string]float64, error) {
	if info == nil || info.ProcNum < 0 || int(info.ProcNum) != len(info.DevProcArray) || info.ProcNum > common.MaxProcNum {
		return nil, fmt.Errorf("invalid DCMI process list")
	}
	result := make(map[string]float64)
	seen := make(map[int32]bool)
	for _, proc := range info.DevProcArray {
		if proc.Pid <= 0 || seen[proc.Pid] || proc.MemUsage < 0 || math.IsNaN(proc.MemUsage) || math.IsInf(proc.MemUsage, 0) || proc.MemUsage > math.MaxFloat64/common.UnitMB {
			return nil, fmt.Errorf("invalid DCMI process entry")
		}
		seen[proc.Pid] = true
		before, found := starts[proc.Pid]
		if !found {
			if enpuProcessGone(procRoot, proc.Pid) {
				continue
			}
			return nil, fmt.Errorf("PID %d appeared during collection", proc.Pid)
		}
		container, err := enpuProcessContainer(procRoot, proc.Pid)
		if err != nil {
			if enpuProcessMissing(err) && enpuProcessGone(procRoot, proc.Pid) {
				continue
			}
			return nil, err
		}
		after, err := enpuProcessStart(procRoot, proc.Pid)
		if err != nil {
			if enpuProcessMissing(err) && enpuProcessGone(procRoot, proc.Pid) {
				continue
			}
			return nil, fmt.Errorf("read stat for PID %d: %w", proc.Pid, err)
		}
		if after != before {
			return nil, fmt.Errorf("PID %d changed during collection", proc.Pid)
		}
		if container != "" {
			result[container] += proc.MemUsage * common.UnitMB
			if math.IsInf(result[container], 0) {
				return nil, fmt.Errorf("container memory overflow")
			}
		}
	}
	return result, nil
}
