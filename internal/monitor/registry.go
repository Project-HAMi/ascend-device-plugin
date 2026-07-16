package monitor

import (
	"fmt"
	"os"
	"sync/atomic"
	"syscall"
	"unsafe"
)

// LocalContainerShmem layout matching Rust #[repr(C)] struct in crates/limiter/src/shmem/mod.rs.
const (
	// Primary fields
	localShmMemLimitOffset      = 0
	localShmMemUsedOffset       = 8
	localShmComputePrioOffset   = 16
	localShmActiveWorkersOffset = 48

	// ProcessSlot: pid@0, hbm_used[8]@8, is_active@72, size 80.
	procSlotSize      = 80
	procSlotPID       = 0
	procSlotHBMOffset = 8 // [AtomicU64; NPU_DEVICE_MAX=8]
	procSlotActive    = 72

	// procs array starts after reports (32 * 32 = 1024) at offset 56
	localShmProcsOffset = 1080 // 56 + 32*32
	procsMax            = 64
	hbmDevices          = 8
)

type PodStats struct {
	MemoryUsed       uint64
	MemoryLimit      uint64
	HasActiveWorkers bool
}

type ShmemReader struct {
	data []byte
}

func OpenLocalShmem(path string) (*ShmemReader, error) {
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() {
		_ = f.Close()
	}()

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if fi.Size() < int64(localShmProcsOffset+procSlotSize) {
		return nil, fmt.Errorf("file too small: %d bytes", fi.Size())
	}

	data, err := syscall.Mmap(int(f.Fd()), 0, int(fi.Size()), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		return nil, fmt.Errorf("mmap: %w", err)
	}
	return &ShmemReader{data: data}, nil
}

func (r *ShmemReader) Close() error {
	return syscall.Munmap(r.data)
}

func (r *ShmemReader) ReadPodStats() PodStats {
	return PodStats{
		MemoryUsed:       atomic.LoadUint64((*uint64)(unsafe.Pointer(&r.data[localShmMemUsedOffset]))),
		MemoryLimit:      atomic.LoadUint64((*uint64)(unsafe.Pointer(&r.data[localShmMemLimitOffset]))),
		HasActiveWorkers: atomic.LoadUint32((*uint32)(unsafe.Pointer(&r.data[localShmActiveWorkersOffset]))) != 0,
	}
}

// ReadMemoryByDevice sums HBM usage per-device across all active process slots.
func (r *ShmemReader) ReadMemoryByDevice() [hbmDevices]uint64 {
	var devMem [hbmDevices]uint64
	for i := 0; i < procsMax; i++ {
		base := localShmProcsOffset + i*procSlotSize
		if base+procSlotSize > len(r.data) {
			break
		}
		isActive := atomic.LoadUint32((*uint32)(unsafe.Pointer(&r.data[base+procSlotActive])))
		if isActive == 0 {
			continue
		}
		for d := 0; d < hbmDevices; d++ {
			devMem[d] += atomic.LoadUint64((*uint64)(unsafe.Pointer(&r.data[base+procSlotHBMOffset+d*8])))
		}
	}
	return devMem
}
