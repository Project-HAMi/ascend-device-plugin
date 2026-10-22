package monitor

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

type coreProcessFixture struct {
	pid    int32
	active uint32
	memory [hbmDevices]uint64
}

func writeCoreShmem(t *testing.T, path string, limit uint64, initialized bool, slots map[int]coreProcessFixture) {
	t.Helper()
	data := make([]byte, localShmInitializedOffset+4)
	binary.LittleEndian.PutUint64(data[localShmMemLimitOffset:], limit)
	// Deliberately different from the process ledger: this is not a per-card
	// metric in a legacy multi-device file.
	binary.LittleEndian.PutUint64(data[localShmMemUsedOffset:], 999999)
	if initialized {
		binary.LittleEndian.PutUint32(data[localShmInitializedOffset:], 1)
	}
	for index, slot := range slots {
		base := localShmProcsOffset + index*procSlotSize
		binary.LittleEndian.PutUint32(data[base+procSlotPID:], uint32(slot.pid))
		binary.LittleEndian.PutUint32(data[base+procSlotActive:], slot.active)
		for device, used := range slot.memory {
			binary.LittleEndian.PutUint64(data[base+procSlotHBMOffset+device*8:], used)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestCoreShmemProcessLedger(t *testing.T) {
	path := filepath.Join(t.TempDir(), "local")
	first := [hbmDevices]uint64{10, 20, 30, 40, 50, 60, 70, 80}
	last := [hbmDevices]uint64{1, 2, 3, 4, 5, 6, 7, 8}
	writeCoreShmem(t, path, 6000, true, map[int]coreProcessFixture{
		0:  {pid: 101, active: 1, memory: first},
		1:  {pid: 0, active: 1, memory: first}, // registration has not published PID
		2:  {pid: 102, active: 0, memory: first},
		3:  {pid: 103, active: 2, memory: first}, // reclaiming slot
		63: {pid: 104, active: 1, memory: last},
	})
	reader, err := OpenLocalShmem(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()
	if !reader.Initialized() || reader.ReadPodStats().HasActiveWorkers {
		t.Fatal("initialized spatial domain must be readable with zero timing workers")
	}
	want := [hbmDevices]uint64{11, 22, 33, 44, 55, 66, 77, 88}
	if got := reader.ReadMemoryByDevice(); got != want {
		t.Fatalf("process ledger = %v, want %v", got, want)
	}
}

func TestCoreShmemRejectsTruncatedHeader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "local")
	if err := os.WriteFile(path, make([]byte, localShmInitializedOffset+3), 0644); err != nil {
		t.Fatal(err)
	}
	if reader, err := OpenLocalShmem(path); err == nil {
		_ = reader.Close()
		t.Fatal("truncated initialization marker accepted")
	}
}
