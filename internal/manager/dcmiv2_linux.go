//go:build linux && cgo

package manager

/*
#cgo LDFLAGS: -ldl
#cgo CFLAGS: -I${SRCDIR}/../../mind-cluster/component/ascend-common/devmanager/dcmi
#include <dlfcn.h>
#include <stdlib.h>
#include <dcmi_interface_api.h>

// The repository header predates DCMI v2 and only declares NDIE/VDIE.
// The 950DT host header adds DDIE as enum value 2; the function ABI still
// passes the enum as an int. Keep the compatibility value local so this
// package can build against the bundled older header.
#define DCMI_V2_DDIE_VALUE 2

static void *dcmi_v2_handle;
static int (*dcmiv2_init_func)(void);
static int (*dcmiv2_get_all_device_count_func)(int *);
static int (*dcmiv2_get_device_list_func)(int *, int *, int);
static int (*dcmiv2_get_device_chip_info_func)(int, struct dcmi_chip_info_v2 *);
static int (*dcmiv2_get_device_health_func)(int, unsigned int *);
static int (*dcmiv2_get_device_die_id_func)(int, enum dcmi_die_type, struct dcmi_die_id *);
static int (*dcmiv2_get_chip_phy_id_by_dev_id_func)(int, unsigned int *);
static int (*dcmiv2_get_dev_id_by_chip_phy_id_func)(int, unsigned int *);
static int (*dcmiv2_get_slot_id_and_chip_id_by_dev_id_func)(int, unsigned int *, unsigned int *);
static int (*dcmiv2_get_dev_id_by_slot_id_and_chip_id_func)(unsigned int, unsigned int, int *);
static int (*dcmiv2_get_device_hbm_info_func)(int, struct dcmi_hbm_info *);

#define LOAD_DCMI_V2(symbol, target) do { \
    target = dlsym(dcmi_v2_handle, #symbol); \
    if (target == NULL) { dlclose(dcmi_v2_handle); dcmi_v2_handle = NULL; return -99998; } \
} while (0)

static int dcmi_v2_load(const char *path) {
    if (dcmi_v2_handle != NULL) return 0;
    dcmi_v2_handle = dlopen(path, RTLD_LAZY | RTLD_GLOBAL);
    if (dcmi_v2_handle == NULL) return -99999;
    LOAD_DCMI_V2(dcmiv2_init, dcmiv2_init_func);
    LOAD_DCMI_V2(dcmiv2_get_all_device_count, dcmiv2_get_all_device_count_func);
    LOAD_DCMI_V2(dcmiv2_get_device_list, dcmiv2_get_device_list_func);
    LOAD_DCMI_V2(dcmiv2_get_device_chip_info, dcmiv2_get_device_chip_info_func);
    LOAD_DCMI_V2(dcmiv2_get_device_health, dcmiv2_get_device_health_func);
    LOAD_DCMI_V2(dcmiv2_get_device_die_id, dcmiv2_get_device_die_id_func);
    LOAD_DCMI_V2(dcmiv2_get_chip_phy_id_by_dev_id, dcmiv2_get_chip_phy_id_by_dev_id_func);
    LOAD_DCMI_V2(dcmiv2_get_dev_id_by_chip_phy_id, dcmiv2_get_dev_id_by_chip_phy_id_func);
    LOAD_DCMI_V2(dcmiv2_get_slot_id_and_chip_id_by_dev_id, dcmiv2_get_slot_id_and_chip_id_by_dev_id_func);
    LOAD_DCMI_V2(dcmiv2_get_dev_id_by_slot_id_and_chip_id, dcmiv2_get_dev_id_by_slot_id_and_chip_id_func);
    LOAD_DCMI_V2(dcmiv2_get_device_hbm_info, dcmiv2_get_device_hbm_info_func);
    return 0;
}

static int dcmi_v2_init_call(void) { return dcmiv2_init_func(); }
static int dcmi_v2_get_all_device_count_call(int *count) { return dcmiv2_get_all_device_count_func(count); }
static int dcmi_v2_get_device_list_call(int *list, int *count, int len) { return dcmiv2_get_device_list_func(list, count, len); }
static int dcmi_v2_get_device_chip_info_call(int id, struct dcmi_chip_info_v2 *info) { return dcmiv2_get_device_chip_info_func(id, info); }
static int dcmi_v2_get_device_health_call(int id, unsigned int *health) { return dcmiv2_get_device_health_func(id, health); }
static int dcmi_v2_get_device_die_id_call(int id, int type, struct dcmi_die_id *die) { return dcmiv2_get_device_die_id_func(id, (enum dcmi_die_type)type, die); }
static int dcmi_v2_get_chip_phy_id_call(int id, unsigned int *phy) { return dcmiv2_get_chip_phy_id_by_dev_id_func(id, phy); }
static int dcmi_v2_get_dev_id_by_phy_call(int phy, unsigned int *id) { return dcmiv2_get_dev_id_by_chip_phy_id_func(phy, id); }
static int dcmi_v2_get_slot_chip_call(int id, unsigned int *slot, unsigned int *chip) { return dcmiv2_get_slot_id_and_chip_id_by_dev_id_func(id, slot, chip); }
static int dcmi_v2_get_dev_id_by_slot_chip_call(unsigned int slot, unsigned int chip, int *id) { return dcmiv2_get_dev_id_by_slot_id_and_chip_id_func(slot, chip, id); }
static int dcmi_v2_get_hbm_info_call(int id, struct dcmi_hbm_info *info) { return dcmiv2_get_device_hbm_info_func(id, info); }
*/
import "C"

import (
	"fmt"
	"unsafe"

	"ascend-common/common-utils/utils"
	"ascend-common/devmanager/common"
)

type dcmiV2Client struct{}

func newDcmiV2Client() (*dcmiV2Client, error) {
	path, err := utils.GetDriverLibPath("libdcmi.so")
	if err != nil {
		return nil, fmt.Errorf("locate libdcmi.so: %w", err)
	}
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	if code := int32(C.dcmi_v2_load(cPath)); code != 0 {
		return nil, fmt.Errorf("load DCMI v2 symbols from %s failed with code %d", path, code)
	}
	return &dcmiV2Client{}, nil
}

func (c *dcmiV2Client) init() error {
	if code := int32(C.dcmi_v2_init_call()); code != 0 {
		return fmt.Errorf("dcmiv2_init returned %d", code)
	}
	return nil
}

func (c *dcmiV2Client) allDeviceCount() (int32, error) {
	var count C.int
	if code := int32(C.dcmi_v2_get_all_device_count_call(&count)); code != 0 {
		return 0, fmt.Errorf("dcmiv2_get_all_device_count returned %d", code)
	}
	return int32(count), nil
}

func (c *dcmiV2Client) deviceList() ([]int32, error) {
	count, err := c.allDeviceCount()
	if err != nil {
		return nil, err
	}
	maxDevices := common.HiAIMaxCardNum * common.HiAIMaxDeviceNum
	if count <= 0 || int(count) > maxDevices {
		return nil, fmt.Errorf("dcmiv2 device count %d is outside (0,%d]", count, maxDevices)
	}
	ids := make([]C.int, int(count))
	var reported C.int
	if code := int32(C.dcmi_v2_get_device_list_call(&ids[0], &reported, C.int(len(ids)))); code != 0 {
		return nil, fmt.Errorf("dcmiv2_get_device_list returned %d", code)
	}
	if int32(reported) != count {
		return nil, fmt.Errorf("dcmiv2 device list count %d does not match reported count %d", reported, count)
	}
	result := make([]int32, len(ids))
	for i, id := range ids {
		result[i] = int32(id)
	}
	return result, nil
}

func cString(value *C.uchar) string {
	return C.GoString((*C.char)(unsafe.Pointer(value)))
}

func (c *dcmiV2Client) chipInfo(id int32) (common.ChipInfo, error) {
	var info C.struct_dcmi_chip_info_v2
	if code := int32(C.dcmi_v2_get_device_chip_info_call(C.int(id), &info)); code != 0 {
		return common.ChipInfo{}, fmt.Errorf("dcmiv2_get_device_chip_info(%d) returned %d", id, code)
	}
	return common.ChipInfo{
		Type: cString(&info.chip_type[0]), Name: cString(&info.chip_name[0]),
		Version: cString(&info.chip_ver[0]), NpuName: cString(&info.npu_name[0]),
		AICoreCnt: int(info.aicore_cnt),
	}, nil
}

func (c *dcmiV2Client) deviceHealth(id int32) (uint32, error) {
	var health C.uint
	if code := int32(C.dcmi_v2_get_device_health_call(C.int(id), &health)); code != 0 {
		return 0, fmt.Errorf("dcmiv2_get_device_health(%d) returned %d", id, code)
	}
	return uint32(health), nil
}

func (c *dcmiV2Client) deviceDDIE(id int32) ([ascend950DTDieIDCount]uint32, error) {
	var info C.struct_dcmi_die_id
	if code := int32(C.dcmi_v2_get_device_die_id_call(C.int(id), C.int(C.DCMI_V2_DDIE_VALUE), &info)); code != 0 {
		return [ascend950DTDieIDCount]uint32{}, fmt.Errorf("dcmiv2_get_device_die_id(%d, DDIE) returned %d", id, code)
	}
	var values [ascend950DTDieIDCount]uint32
	for i := range values {
		values[i] = uint32(info.soc_die[i])
	}
	return values, nil
}

func (c *dcmiV2Client) chipPhyID(id int32) (int32, error) {
	var phy C.uint
	if code := int32(C.dcmi_v2_get_chip_phy_id_call(C.int(id), &phy)); code != 0 {
		return -1, fmt.Errorf("dcmiv2_get_chip_phy_id_by_dev_id(%d) returned %d", id, code)
	}
	if uint64(phy) > uint64(^uint32(0)>>1) {
		return -1, fmt.Errorf("physical ID %d is out of range", uint32(phy))
	}
	return int32(phy), nil
}

func (c *dcmiV2Client) deviceIDByChipPhyID(phy int32) (int32, error) {
	if phy < 0 {
		return -1, fmt.Errorf("invalid physical ID %d", phy)
	}
	var id C.uint
	if code := int32(C.dcmi_v2_get_dev_id_by_phy_call(C.int(phy), &id)); code != 0 {
		return -1, fmt.Errorf("dcmiv2_get_dev_id_by_chip_phy_id(%d) returned %d", phy, code)
	}
	if uint64(id) > uint64(^uint32(0)>>1) {
		return -1, fmt.Errorf("logic ID %d is out of range", uint32(id))
	}
	return int32(id), nil
}

func (c *dcmiV2Client) slotChipID(id int32) (int32, int32, error) {
	var slot, chip C.uint
	if code := int32(C.dcmi_v2_get_slot_chip_call(C.int(id), &slot, &chip)); code != 0 {
		return -1, -1, fmt.Errorf("dcmiv2_get_slot_id_and_chip_id_by_dev_id(%d) returned %d", id, code)
	}
	maxInt32 := uint64(^uint32(0) >> 1)
	if uint64(slot) > maxInt32 || uint64(chip) > maxInt32 {
		return -1, -1, fmt.Errorf("slot/chip ID out of range: slot=%d chip=%d", uint32(slot), uint32(chip))
	}
	return int32(slot), int32(chip), nil
}

func (c *dcmiV2Client) deviceIDBySlotChipID(slot, chip int32) (int32, error) {
	if slot < 0 || chip < 0 {
		return -1, fmt.Errorf("invalid slot/chip ID: slot=%d chip=%d", slot, chip)
	}
	var id C.int
	if code := int32(C.dcmi_v2_get_dev_id_by_slot_chip_call(C.uint(slot), C.uint(chip), &id)); code != 0 {
		return -1, fmt.Errorf("dcmiv2_get_dev_id_by_slot_id_and_chip_id(%d,%d) returned %d", slot, chip, code)
	}
	return int32(id), nil
}

func (c *dcmiV2Client) hbmInfo(id int32) (*common.HbmInfo, error) {
	var info C.struct_dcmi_hbm_info
	if code := int32(C.dcmi_v2_get_hbm_info_call(C.int(id), &info)); code != 0 {
		return nil, fmt.Errorf("dcmiv2_get_device_hbm_info(%d) returned %d", id, code)
	}
	return &common.HbmInfo{
		MemorySize: uint64(info.memory_size), Usage: uint64(info.memory_usage),
		Frequency: uint32(info.freq), Temp: int32(info.temp),
		BandWidthUtilRate: uint32(info.bandwith_util_rate),
	}, nil
}
