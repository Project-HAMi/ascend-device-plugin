package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/Project-HAMi/HAMi/pkg/device"
	"github.com/Project-HAMi/ascend-device-plugin/internal/manager"
)

// newTestPluginServer creates a PluginServer with sensible defaults for testing.
func newTestPluginServer(allocAnno, toAllocAnno string) *PluginServer {
	return &PluginServer{
		mgr:               &FakeManager{},
		commonWord:        testCommonWord,
		allocAnno:         allocAnno,
		toAllocDeviceAnno: toAllocAnno,
	}
}

// ============================================================================
// fileSHA256 tests
// ============================================================================

func TestFileSHA256(t *testing.T) {
	t.Parallel()

	type fileSHA256Args struct {
		path string
	}

	tests := []struct {
		name    string
		args    fileSHA256Args
		want    string
		wantErr bool
	}{
		{
			name:    "NonExistentFile",
			args:    fileSHA256Args{path: "/nonexistent/path/file.txt"},
			wantErr: true,
		},
		{
			name: "EmptyFile",
			args: func() fileSHA256Args {
				dir := t.TempDir()
				f := filepath.Join(dir, "empty.txt")
				_ = os.WriteFile(f, []byte{}, 0644)
				return fileSHA256Args{path: f}
			}(),
			want: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name: "KnownContent",
			args: func() fileSHA256Args {
				dir := t.TempDir()
				f := filepath.Join(dir, "hello.txt")
				_ = os.WriteFile(f, []byte("hello world"), 0644)
				return fileSHA256Args{path: f}
			}(),
			want: "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := fileSHA256(tc.args.path)
			if (err != nil) != tc.wantErr {
				t.Fatalf("fileSHA256() error = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("fileSHA256() = %q, want %q", got, tc.want)
			}
		})
	}
}

// ============================================================================
// copyFile tests
// ============================================================================

func TestCopyFile(t *testing.T) {
	t.Parallel()

	type copyFileArgs struct {
		src string
		dst string
	}

	type copyFileWant struct {
		content    string
		checkPerms bool
	}

	tests := []struct {
		name    string
		args    copyFileArgs
		want    copyFileWant
		wantErr bool
	}{
		{
			name:    "NonExistentSource",
			args:    copyFileArgs{src: "/nonexistent/src.txt", dst: filepath.Join(t.TempDir(), "dst.txt")},
			wantErr: true,
		},
		{
			name: "ContentPreserved",
			args: func() copyFileArgs {
				dir := t.TempDir()
				src := filepath.Join(dir, "src.txt")
				dst := filepath.Join(dir, "dst.txt")
				_ = os.WriteFile(src, []byte("test content for copy"), 0755)
				return copyFileArgs{src: src, dst: dst}
			}(),
			want: copyFileWant{content: "test content for copy"},
		},
		{
			name: "PermissionsPreserved",
			args: func() copyFileArgs {
				dir := t.TempDir()
				src := filepath.Join(dir, "src.txt")
				dst := filepath.Join(dir, "dst.txt")
				_ = os.WriteFile(src, []byte("x"), 0755)
				return copyFileArgs{src: src, dst: dst}
			}(),
			want: copyFileWant{checkPerms: true},
		},
		{
			name: "OverwritesExisting",
			args: func() copyFileArgs {
				dir := t.TempDir()
				src := filepath.Join(dir, "src.txt")
				dst := filepath.Join(dir, "dst.txt")
				_ = os.WriteFile(src, []byte("new content"), 0644)
				_ = os.WriteFile(dst, []byte("old content"), 0644)
				return copyFileArgs{src: src, dst: dst}
			}(),
			want: copyFileWant{content: "new content"},
		},
		{
			name: "DestinationDirectoryNotExist",
			args: func() copyFileArgs {
				dir := t.TempDir()
				src := filepath.Join(dir, "src.txt")
				dst := filepath.Join(dir, "nonexistent", "dst.txt")
				_ = os.WriteFile(src, []byte("x"), 0644)
				return copyFileArgs{src: src, dst: dst}
			}(),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := copyFile(tc.args.src, tc.args.dst)
			if (err != nil) != tc.wantErr {
				t.Fatalf("copyFile() error = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if tc.want.content != "" {
				got, err := os.ReadFile(tc.args.dst)
				if err != nil {
					t.Fatalf("failed to read dst: %v", err)
				}
				if string(got) != tc.want.content {
					t.Fatalf("dst content = %q, want %q", got, tc.want.content)
				}
			}
			if tc.want.checkPerms {
				srcInfo, _ := os.Stat(tc.args.src)
				dstInfo, _ := os.Stat(tc.args.dst)
				if srcInfo.Mode() != dstInfo.Mode() {
					t.Fatalf("dst mode = %v, want %v", dstInfo.Mode(), srcInfo.Mode())
				}
			}
		})
	}
}

// ============================================================================
// buildContainerAllocateResponse tests
// ============================================================================

func TestBuildContainerAllocateResponse(t *testing.T) {
	const allocAnno = "huawei.com/Ascend910"

	type buildContainerAllocateResponseArgs struct {
		pod           *v1.Pod
		containerDevs device.ContainerDevices
		rtInfoLookup  map[string]RuntimeInfo
	}

	type buildContainerAllocateResponseWant struct {
		envs   map[string]string
		mounts []*v1beta1.Mount
	}

	tests := []struct {
		name    string
		setup   func() (*PluginServer, CleanupFunc)
		args    buildContainerAllocateResponseArgs
		want    buildContainerAllocateResponseWant
		wantErr string
	}{
		{
			name: "SingleDeviceNonHamiCore",
			setup: func() (*PluginServer, CleanupFunc) {
				return &PluginServer{
					mgr: &FakeManager{
						GetDeviceByUUIDFunc: func(uuid string) *manager.Device {
							return &manager.Device{UUID: "uuid1", PhyID: 3}
						},
					},
					allocAnno: allocAnno,
				}, func() {}
			},
			args: buildContainerAllocateResponseArgs{
				pod:           &v1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{}}},
				containerDevs: device.ContainerDevices{cd("uuid1", "Ascend910", 1024, 4)},
				rtInfoLookup: map[string]RuntimeInfo{
					"uuid1": {UUID: "uuid1", Temp: "vir01"},
				},
			},
			want: buildContainerAllocateResponseWant{
				envs: map[string]string{
					"ASCEND_VISIBLE_DEVICES": "3",
					"ASCEND_VNPU_SPECS":      "vir01",
				},
			},
		},
		{
			name: "MultipleDevices",
			setup: func() (*PluginServer, CleanupFunc) {
				return &PluginServer{
					mgr: &FakeManager{
						GetDeviceByUUIDFunc: func(uuid string) *manager.Device {
							switch uuid {
							case "uuid1":
								return &manager.Device{UUID: "uuid1", PhyID: 0}
							case "uuid2":
								return &manager.Device{UUID: "uuid2", PhyID: 1}
							default:
								return nil
							}
						},
					},
					allocAnno: allocAnno,
				}, func() {}
			},
			args: buildContainerAllocateResponseArgs{
				pod:           &v1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{}}},
				containerDevs: device.ContainerDevices{cd("uuid1", "Ascend910", 1024, 4), cd("uuid2", "Ascend910", 2048, 8)},
				rtInfoLookup: map[string]RuntimeInfo{
					"uuid1": {UUID: "uuid1", Temp: "vir01"},
					"uuid2": {UUID: "uuid2", Temp: "vir01"},
				},
			},
			want: buildContainerAllocateResponseWant{
				envs: map[string]string{
					"ASCEND_VISIBLE_DEVICES": "0,1",
					"ASCEND_VNPU_SPECS":      "vir01",
				},
			},
		},
		{
			name: "UnknownUUID",
			setup: func() (*PluginServer, CleanupFunc) {
				return &PluginServer{
					mgr:       &FakeManager{},
					allocAnno: allocAnno,
				}, func() {}
			},
			args: buildContainerAllocateResponseArgs{
				pod:           &v1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{}}},
				containerDevs: device.ContainerDevices{cd("unknown-uuid", "Ascend910", 1024, 4)},
			},
			wantErr: "unknown uuid",
		},
		{
			name: "EmptyContainerDevs",
			setup: func() (*PluginServer, CleanupFunc) {
				return &PluginServer{
					mgr:       &FakeManager{},
					allocAnno: allocAnno,
				}, func() {}
			},
			args: buildContainerAllocateResponseArgs{
				pod:           &v1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{}}},
				containerDevs: device.ContainerDevices{},
			},
			wantErr: "annotation",
		},
		{
			name: "HamiCoreMode_Mounts",
			setup: func() (*PluginServer, CleanupFunc) {
				return &PluginServer{
					mgr: &FakeManager{
						GetDeviceByUUIDFunc: func(uuid string) *manager.Device {
							return &manager.Device{UUID: "uuid1", PhyID: 3}
						},
					},
					allocAnno: allocAnno,
				}, func() {}
			},
			args: buildContainerAllocateResponseArgs{
				pod: &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							VNPUModeAnnotation: VNPUModeHamiCore,
						},
					},
				},
				containerDevs: device.ContainerDevices{cd("uuid1", "Ascend910", 1024, 4)},
				rtInfoLookup: func() map[string]RuntimeInfo {
					mem := int64(16384)
					core := int32(4)
					return map[string]RuntimeInfo{
						"uuid1": {UUID: "uuid1", Temp: "vir01", Memory: &mem, Core: &core},
					}
				}(),
			},
			want: buildContainerAllocateResponseWant{
				envs: map[string]string{
					"ASCEND_VISIBLE_DEVICES": "3",
					"NPU_MEM_QUOTA":          "16384",
					"NPU_PRIORITY":           "4",
					"NPU_GLOBAL_SHM_PATH":    "/hami-shared-region/3_global_registry",
				},
				mounts: []*v1beta1.Mount{
					{HostPath: "/usr/local/bin/npu-smi", ContainerPath: "/usr/local/bin/npu-smi", ReadOnly: true},
					{HostPath: "/etc/ascend_install.info", ContainerPath: "/etc/ascend_install.info", ReadOnly: true},
					{HostPath: "/usr/local/Ascend/driver/lib64/driver", ContainerPath: "/usr/local/Ascend/driver/lib64/driver", ReadOnly: true},
					{HostPath: "/usr/local/Ascend/driver/version.info", ContainerPath: "/usr/local/Ascend/driver/version.info", ReadOnly: true},
					{HostPath: "/usr/local/hami-vnpu-core", ContainerPath: "/hami-vnpu-core", ReadOnly: true},
					{HostPath: "/usr/local/hami-vnpu-core/ld.so.preload", ContainerPath: "/etc/ld.so.preload", ReadOnly: true},
					{HostPath: "/usr/local/hami-shared-region", ContainerPath: "/hami-shared-region", ReadOnly: false},
				},
			},
		},
		{
			name: "HamiCoreMode_NilMemoryCore",
			setup: func() (*PluginServer, CleanupFunc) {
				return &PluginServer{
					mgr: &FakeManager{
						GetDeviceByUUIDFunc: func(uuid string) *manager.Device {
							return &manager.Device{UUID: "uuid1", PhyID: 3}
						},
					},
					allocAnno: allocAnno,
				}, func() {}
			},
			args: buildContainerAllocateResponseArgs{
				pod: &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{VNPUModeAnnotation: VNPUModeHamiCore},
					},
				},
				containerDevs: device.ContainerDevices{cd("uuid1", "Ascend910", 1024, 4)},
				rtInfoLookup: map[string]RuntimeInfo{
					"uuid1": {UUID: "uuid1", Temp: "vir01", Memory: nil, Core: nil},
				},
			},
			want: buildContainerAllocateResponseWant{
				envs: map[string]string{
					"ASCEND_VISIBLE_DEVICES": "3",
					"NPU_GLOBAL_SHM_PATH":    "/hami-shared-region/3_global_registry",
				},
			},
		},
		{
			name: "HamiCoreMode_MemoryAndCoreSet",
			setup: func() (*PluginServer, CleanupFunc) {
				return &PluginServer{
					mgr: &FakeManager{
						GetDeviceByUUIDFunc: func(uuid string) *manager.Device {
							return &manager.Device{UUID: "uuid1", PhyID: 5}
						},
					},
					allocAnno: allocAnno,
				}, func() {}
			},
			args: buildContainerAllocateResponseArgs{
				pod: &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{VNPUModeAnnotation: VNPUModeHamiCore},
					},
				},
				containerDevs: device.ContainerDevices{cd("uuid1", "Ascend910", 1024, 4)},
				rtInfoLookup: func() map[string]RuntimeInfo {
					mem := int64(8192)
					core := int32(2)
					return map[string]RuntimeInfo{
						"uuid1": {UUID: "uuid1", Temp: "vir01", Memory: &mem, Core: &core},
					}
				}(),
			},
			want: buildContainerAllocateResponseWant{
				envs: map[string]string{
					"NPU_MEM_QUOTA": "8192",
					"NPU_PRIORITY":  "2",
				},
			},
		},
		{
			name: "NonHamiCore_EmptyTemp",
			setup: func() (*PluginServer, CleanupFunc) {
				return &PluginServer{
					mgr: &FakeManager{
						GetDeviceByUUIDFunc: func(uuid string) *manager.Device {
							return &manager.Device{UUID: "uuid1", PhyID: 5}
						},
					},
					allocAnno: allocAnno,
				}, func() {}
			},
			args: buildContainerAllocateResponseArgs{
				pod:           &v1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{}}},
				containerDevs: device.ContainerDevices{cd("uuid1", "Ascend910", 1024, 4)},
				rtInfoLookup: map[string]RuntimeInfo{
					"uuid1": {UUID: "uuid1", Temp: ""},
				},
			},
			want: buildContainerAllocateResponseWant{
				envs: map[string]string{
					"ASCEND_VISIBLE_DEVICES": "5",
				},
			},
		},
		{
			name: "NonHamiCore_NonEmptyTemp",
			setup: func() (*PluginServer, CleanupFunc) {
				return &PluginServer{
					mgr: &FakeManager{
						GetDeviceByUUIDFunc: func(uuid string) *manager.Device {
							return &manager.Device{UUID: "uuid1", PhyID: 5}
						},
					},
					allocAnno: allocAnno,
				}, func() {}
			},
			args: buildContainerAllocateResponseArgs{
				pod:           &v1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{}}},
				containerDevs: device.ContainerDevices{cd("uuid1", "Ascend910", 1024, 4)},
				rtInfoLookup: map[string]RuntimeInfo{
					"uuid1": {UUID: "uuid1", Temp: "vir02"},
				},
			},
			want: buildContainerAllocateResponseWant{
				envs: map[string]string{
					"ASCEND_VISIBLE_DEVICES": "5",
					"ASCEND_VNPU_SPECS":      "vir02",
				},
			},
		},
		{
			name: "UUIDNotInLookup",
			setup: func() (*PluginServer, CleanupFunc) {
				return &PluginServer{
					mgr: &FakeManager{
						GetDeviceByUUIDFunc: func(uuid string) *manager.Device {
							return &manager.Device{UUID: "uuid1", PhyID: 3}
						},
					},
					allocAnno: allocAnno,
				}, func() {}
			},
			args: buildContainerAllocateResponseArgs{
				pod:           &v1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{}}},
				containerDevs: device.ContainerDevices{cd("uuid1", "Ascend910", 1024, 4)},
				rtInfoLookup:  map[string]RuntimeInfo{},
			},
			want: buildContainerAllocateResponseWant{
				envs: map[string]string{
					"ASCEND_VISIBLE_DEVICES": "3",
				},
			},
		},
		{
			name: "FirstTempUsedWhenMultipleDevices",
			setup: func() (*PluginServer, CleanupFunc) {
				return &PluginServer{
					mgr: &FakeManager{
						GetDeviceByUUIDFunc: func(uuid string) *manager.Device {
							switch uuid {
							case "uuid1":
								return &manager.Device{UUID: "uuid1", PhyID: 0}
							case "uuid2":
								return &manager.Device{UUID: "uuid2", PhyID: 1}
							default:
								return nil
							}
						},
					},
					allocAnno: allocAnno,
				}, func() {}
			},
			args: buildContainerAllocateResponseArgs{
				pod:           &v1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{}}},
				containerDevs: device.ContainerDevices{cd("uuid1", "Ascend910", 1024, 4), cd("uuid2", "Ascend910", 2048, 8)},
				rtInfoLookup: map[string]RuntimeInfo{
					"uuid1": {UUID: "uuid1", Temp: "vir01"},
					"uuid2": {UUID: "uuid2", Temp: "vir02"},
				},
			},
			want: buildContainerAllocateResponseWant{
				envs: map[string]string{
					"ASCEND_VISIBLE_DEVICES": "0,1",
					"ASCEND_VNPU_SPECS":      "vir01",
				},
			},
		},
		{
			name: "HamiCoreMultiDevice",
			setup: func() (*PluginServer, CleanupFunc) {
				return &PluginServer{
					mgr: &FakeManager{
						GetDeviceByUUIDFunc: func(uuid string) *manager.Device {
							switch uuid {
							case "uuid1":
								return &manager.Device{UUID: "uuid1", PhyID: 0}
							case "uuid2":
								return &manager.Device{UUID: "uuid2", PhyID: 1}
							default:
								return nil
							}
						},
					},
					allocAnno: allocAnno,
				}, func() {}
			},
			args: buildContainerAllocateResponseArgs{
				pod: &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{VNPUModeAnnotation: VNPUModeHamiCore},
					},
				},
				containerDevs: device.ContainerDevices{cd("uuid1", "Ascend910", 1024, 4), cd("uuid2", "Ascend910", 2048, 8)},
				rtInfoLookup: func() map[string]RuntimeInfo {
					mem := int64(32768)
					core := int32(8)
					return map[string]RuntimeInfo{
						"uuid1": {UUID: "uuid1", Temp: "vir01", Memory: &mem, Core: &core},
						"uuid2": {UUID: "uuid2", Temp: "vir02", Memory: nil, Core: nil},
					}
				}(),
			},
			want: buildContainerAllocateResponseWant{
				envs: map[string]string{
					"ASCEND_VISIBLE_DEVICES": "0,1",
					"NPU_MEM_QUOTA":          "32768",
					"NPU_PRIORITY":           "8",
					"NPU_GLOBAL_SHM_PATH":    "/hami-shared-region/0_global_registry",
				},
			},
		},
		{
			name: "ErrorIncludesAllocAnno",
			setup: func() (*PluginServer, CleanupFunc) {
				return &PluginServer{
					mgr:       &FakeManager{},
					allocAnno: allocAnno,
				}, func() {}
			},
			args: buildContainerAllocateResponseArgs{
				pod:           &v1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{}}},
				containerDevs: device.ContainerDevices{},
			},
			wantErr: "huawei.com/Ascend910",
		},
		{
			name: "ResponseStructFields",
			setup: func() (*PluginServer, CleanupFunc) {
				return &PluginServer{
					mgr: &FakeManager{
						GetDeviceByUUIDFunc: func(uuid string) *manager.Device {
							return &manager.Device{UUID: "uuid1", PhyID: 7}
						},
					},
					allocAnno: allocAnno,
				}, func() {}
			},
			args: buildContainerAllocateResponseArgs{
				pod:           &v1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{}}},
				containerDevs: device.ContainerDevices{cd("uuid1", "Ascend910", 1024, 4)},
				rtInfoLookup: map[string]RuntimeInfo{
					"uuid1": {UUID: "uuid1", Temp: "vir01"},
				},
			},
			want: buildContainerAllocateResponseWant{
				envs: map[string]string{
					"ASCEND_VISIBLE_DEVICES": "7",
					"ASCEND_VNPU_SPECS":      "vir01",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ps, cleanup := tc.setup()
			t.Cleanup(cleanup)

			resp, err := ps.buildContainerAllocateResponse(tc.args.pod, tc.args.containerDevs, tc.args.rtInfoLookup)

			if tc.wantErr != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error should contain %q, got: %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Check envs
			for k, wantVal := range tc.want.envs {
				if resp.Envs[k] != wantVal {
					t.Fatalf("env[%q] = %q, want %q", k, resp.Envs[k], wantVal)
				}
			}

			// Check that unwanted envs are absent
			if _, ok := resp.Envs["ASCEND_VNPU_SPECS"]; ok && tc.want.envs["ASCEND_VNPU_SPECS"] == "" {
				// Only fail if the test doesn't expect ASCEND_VNPU_SPECS
				if _, wantSet := tc.want.envs["ASCEND_VNPU_SPECS"]; !wantSet {
					t.Fatal("ASCEND_VNPU_SPECS should not be set")
				}
			}

			// Check mounts
			if tc.want.mounts != nil {
				if len(resp.Mounts) != len(tc.want.mounts) {
					t.Fatalf("expected %d mounts, got %d", len(tc.want.mounts), len(resp.Mounts))
				}
				for i, wantMount := range tc.want.mounts {
					if resp.Mounts[i].HostPath != wantMount.HostPath {
						t.Errorf("mount[%d].HostPath = %q, want %q", i, resp.Mounts[i].HostPath, wantMount.HostPath)
					}
					if resp.Mounts[i].ContainerPath != wantMount.ContainerPath {
						t.Errorf("mount[%d].ContainerPath = %q, want %q", i, resp.Mounts[i].ContainerPath, wantMount.ContainerPath)
					}
					if resp.Mounts[i].ReadOnly != wantMount.ReadOnly {
						t.Errorf("mount[%d].ReadOnly = %v, want %v", i, resp.Mounts[i].ReadOnly, wantMount.ReadOnly)
					}
				}
			}

			// Non-hami-core mode: Mounts and Devices should be nil
			if tc.want.mounts == nil && tc.args.pod.Annotations[VNPUModeAnnotation] != VNPUModeHamiCore {
				if resp.Mounts != nil {
					t.Fatal("resp.Mounts should be nil in non-hami-core mode")
				}
				if resp.Devices != nil {
					t.Fatal("resp.Devices should be nil")
				}
			}
		})
	}
}

// ============================================================================
// popNextContainerDevices tests
// ============================================================================

func TestPopNextContainerDevices(t *testing.T) {
	ps := newTestPluginServer("huawei.com/Ascend910", "hami.io/Ascend910-devices-to-allocate")

	type popNextContainerDevicesWant struct {
		firstUUID    string
		mutatedFirst bool
		remaining    int
	}

	tests := []struct {
		name         string
		podSingleDev device.PodSingleDevice
		want         popNextContainerDevicesWant
		wantErr      string
	}{
		{
			name:         "EmptyPodSingleDevice",
			podSingleDev: device.PodSingleDevice{},
			wantErr:      "no pending device allocation found",
		},
		{
			name:         "AllContainersEmpty",
			podSingleDev: device.PodSingleDevice{{}, {}, {}},
			wantErr:      "no pending device allocation found",
		},
		{
			name: "FirstContainerHasDevices",
			podSingleDev: device.PodSingleDevice{
				{cd("uuid1", "Ascend910", 1024, 4)},
				{cd("uuid2", "Ascend910", 2048, 8)},
			},
			want: popNextContainerDevicesWant{
				firstUUID:    "uuid1",
				mutatedFirst: true,
				remaining:    1,
			},
		},
		{
			name: "SecondContainerHasDevices",
			podSingleDev: device.PodSingleDevice{
				{},
				{cd("uuid2", "Ascend910", 2048, 8)},
				{cd("uuid3", "Ascend910", 512, 2)},
			},
			want: popNextContainerDevicesWant{
				firstUUID: "uuid2",
			},
		},
		{
			name: "MutationErasesFirstNonEmpty",
			podSingleDev: device.PodSingleDevice{
				{cd("uuid1", "Ascend910", 1024, 4)},
				{cd("uuid2", "Ascend910", 2048, 8)},
			},
			want: popNextContainerDevicesWant{
				firstUUID:    "uuid1",
				mutatedFirst: true,
				remaining:    1,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ps.popNextContainerDevices(tc.podSingleDev)

			if tc.wantErr != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error should contain %q, got: %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.want.firstUUID != "" {
				if len(got) == 0 || got[0].UUID != tc.want.firstUUID {
					t.Fatalf("device UUID = %q, want %q", got[0].UUID, tc.want.firstUUID)
				}
			}

			if tc.want.mutatedFirst && len(tc.podSingleDev) > 0 {
				if len(tc.podSingleDev[0]) != 0 {
					t.Fatalf("first container should be erased after pop, got %d devices", len(tc.podSingleDev[0]))
				}
			}

			if tc.want.remaining > 0 && len(tc.podSingleDev) > 1 {
				if len(tc.podSingleDev[1]) != tc.want.remaining {
					t.Fatalf("second container should still have %d device(s), got %d", tc.want.remaining, len(tc.podSingleDev[1]))
				}
			}
		})
	}
}

// ============================================================================
// buildRuntimeInfoLookup tests
// ============================================================================

func TestBuildRuntimeInfoLookup(t *testing.T) {
	const allocAnno = "huawei.com/Ascend910"

	type buildRuntimeInfoLookupWant struct {
		lookup map[string]RuntimeInfo
	}

	tests := []struct {
		name    string
		pod     *v1.Pod
		want    buildRuntimeInfoLookupWant
		wantErr string
	}{
		{
			name:    "AnnotationNotSet",
			pod:     &v1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{}}},
			wantErr: "not set",
		},
		{
			name: "InvalidJSON",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						allocAnno: "not-json",
					},
				},
			},
			wantErr: "invalid",
		},
		{
			name: "Normal",
			pod: func() *v1.Pod {
				mem := int64(16384)
				core := int32(4)
				rtInfo := []RuntimeInfo{
					{UUID: "uuid1", Temp: "vir01", Memory: &mem, Core: &core},
					{UUID: "uuid2", Temp: "vir02"},
				}
				data, _ := json.Marshal(rtInfo)
				return &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							allocAnno: string(data),
						},
					},
				}
			}(),
			want: buildRuntimeInfoLookupWant{
				lookup: map[string]RuntimeInfo{
					"uuid1": {UUID: "uuid1", Temp: "vir01"},
					"uuid2": {UUID: "uuid2", Temp: "vir02"},
				},
			},
		},
		{
			name: "EmptyUUIDSkipped",
			pod: func() *v1.Pod {
				rtInfo := []RuntimeInfo{
					{UUID: "", Temp: "vir01"},
					{UUID: "uuid2", Temp: "vir02"},
				}
				data, _ := json.Marshal(rtInfo)
				return &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							allocAnno: string(data),
						},
					},
				}
			}(),
			want: buildRuntimeInfoLookupWant{
				lookup: map[string]RuntimeInfo{
					"uuid2": {UUID: "uuid2", Temp: "vir02"},
				},
			},
		},
		{
			name: "MultipleEntries",
			pod: func() *v1.Pod {
				rtInfo := []RuntimeInfo{
					{UUID: "uuid1", Temp: "vir01"},
					{UUID: "uuid2", Temp: "vir02"},
					{UUID: "uuid3", Temp: "vir03"},
				}
				data, _ := json.Marshal(rtInfo)
				return &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							allocAnno: string(data),
						},
					},
				}
			}(),
			want: buildRuntimeInfoLookupWant{
				lookup: map[string]RuntimeInfo{
					"uuid1": {UUID: "uuid1", Temp: "vir01"},
					"uuid2": {UUID: "uuid2", Temp: "vir02"},
					"uuid3": {UUID: "uuid3", Temp: "vir03"},
				},
			},
		},
	}

	ps := &PluginServer{allocAnno: allocAnno}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ps.buildRuntimeInfoLookup(tc.pod)

			if tc.wantErr != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error should contain %q, got: %v", tc.wantErr, err)
				}
				// For InvalidJSON, verify %w wrapping
				if tc.name == "InvalidJSON" {
					var jsonErr *json.SyntaxError
					if !errors.As(err, &jsonErr) {
						t.Fatalf("expected wrapped json.SyntaxError, got: %v", err)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tc.want.lookup) {
				t.Fatalf("expected %d entries, got %d", len(tc.want.lookup), len(got))
			}
			for uuid, wantInfo := range tc.want.lookup {
				gotInfo, ok := got[uuid]
				if !ok {
					t.Fatalf("expected UUID %q in lookup", uuid)
				}
				if gotInfo.Temp != wantInfo.Temp {
					t.Fatalf("lookup[%q].Temp = %q, want %q", uuid, gotInfo.Temp, wantInfo.Temp)
				}
			}
		})
	}
}

// ============================================================================
// decodeDeviceAnnotations tests
// ============================================================================

func TestDecodeDeviceAnnotations(t *testing.T) {
	type decodeDeviceAnnotationsWant struct {
		nonEmptyContainers int
	}

	tests := []struct {
		name    string
		pod     *v1.Pod
		want    decodeDeviceAnnotationsWant
		wantErr string
		setup   func() CleanupFunc
	}{
		{
			name:    "AnnotationNotPresent",
			pod:     &v1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{}}},
			wantErr: "not found in pod annotations",
			setup:   func() CleanupFunc { return setupInRequestDevices(testCommonWord) },
		},
		{
			name: "ValidAnnotation",
			pod: func() *v1.Pod {
				input := device.EncodePodSingleDevice(device.PodSingleDevice{
					{cd("uuid1", "Ascend910", 1024, 4)},
				})
				return &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"hami.io/Ascend910-devices-to-allocate": input,
						},
					},
				}
			}(),
			want:  decodeDeviceAnnotationsWant{nonEmptyContainers: 1},
			setup: func() CleanupFunc { return setupInRequestDevices(testCommonWord) },
		},
		{
			name: "MultiContainer",
			pod: func() *v1.Pod {
				input := device.EncodePodSingleDevice(device.PodSingleDevice{
					{cd("uuid1", "Ascend910", 1024, 4)},
					{cd("uuid2", "Ascend910", 2048, 8)},
				})
				return &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"hami.io/Ascend910-devices-to-allocate": input,
						},
					},
				}
			}(),
			want:  decodeDeviceAnnotationsWant{nonEmptyContainers: 2},
			setup: func() CleanupFunc { return setupInRequestDevices(testCommonWord) },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				t.Cleanup(tc.setup())
			}

			ps := newTestPluginServer("huawei.com/Ascend910", "hami.io/Ascend910-devices-to-allocate")
			got, err := ps.decodeDeviceAnnotations(tc.pod)

			if tc.wantErr != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error should contain %q, got: %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			nonEmpty := 0
			for _, ctrDevs := range got {
				if len(ctrDevs) > 0 {
					nonEmpty++
				}
			}
			if nonEmpty != tc.want.nonEmptyContainers {
				t.Fatalf("expected %d non-empty containers, got %d", tc.want.nonEmptyContainers, nonEmpty)
			}
		})
	}
}

// ============================================================================
// patchErasedAnnotation tests
// ============================================================================

func TestPatchErasedAnnotation(t *testing.T) {
	type patchErasedAnnotationWant struct {
		annotationChanged bool
		nonEmptyAfter     int
	}

	tests := []struct {
		name    string
		pod     *v1.Pod
		want    patchErasedAnnotationWant
		wantErr bool
	}{
		{
			name: "PatchesPodAnnotation",
			pod: func() *v1.Pod {
				toAllocAnno := "hami.io/Ascend910-devices-to-allocate"
				input := device.EncodePodSingleDevice(device.PodSingleDevice{
					{cd("uuid1", "Ascend910", 1024, 4)},
					{cd("uuid2", "Ascend910", 2048, 8)},
				})
				return &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test-pod",
						Namespace:   "default",
						Annotations: map[string]string{toAllocAnno: input},
					},
				}
			}(),
			want: patchErasedAnnotationWant{
				annotationChanged: true,
				nonEmptyAfter:     1,
			},
		},
		{
			name: "UpdatesInMemoryAnnotations",
			pod: func() *v1.Pod {
				toAllocAnno := "hami.io/Ascend910-devices-to-allocate"
				input := device.EncodePodSingleDevice(device.PodSingleDevice{
					{cd("uuid1", "Ascend910", 1024, 4)},
				})
				return &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test-pod",
						Namespace:   "default",
						Annotations: map[string]string{toAllocAnno: input},
					},
				}
			}(),
			want: patchErasedAnnotationWant{
				annotationChanged: true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(setupInRequestDevices(testCommonWord))
			t.Cleanup(setupFakeClient([]*v1.Pod{tc.pod}, nil))

			ps := &PluginServer{
				commonWord:        testCommonWord,
				toAllocDeviceAnno: "hami.io/Ascend910-devices-to-allocate",
			}
			podSingleDev, _ := ps.decodeDeviceAnnotations(tc.pod)
			ps.popNextContainerDevices(podSingleDev)

			origValue := tc.pod.Annotations[ps.toAllocDeviceAnno]

			err := ps.patchErasedAnnotation(tc.pod, podSingleDev)

			if (err != nil) != tc.wantErr {
				t.Fatalf("patchErasedAnnotation() error = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.want.annotationChanged {
				if tc.pod.Annotations[ps.toAllocDeviceAnno] == origValue {
					t.Fatal("pod.Annotations should have been updated in place")
				}
			}

			if tc.want.nonEmptyAfter > 0 {
				got, err := ps.decodeDeviceAnnotations(tc.pod)
				if err != nil {
					t.Fatalf("failed to decode patched annotation: %v", err)
				}
				nonEmpty := 0
				for _, ctrDevs := range got {
					if len(ctrDevs) > 0 {
						nonEmpty++
					}
				}
				if nonEmpty != tc.want.nonEmptyAfter {
					t.Fatalf("expected %d non-empty container(s) after erase, got %d", tc.want.nonEmptyAfter, nonEmpty)
				}
			}
		})
	}
}

// ============================================================================
// Integration: popNextContainerDevices after decode
// ============================================================================

func TestPopNextContainerDevices_AfterDecode(t *testing.T) {
	cleanup := setupInRequestDevices(testCommonWord)
	defer cleanup()

	ps := newTestPluginServer("huawei.com/Ascend910", "hami.io/Ascend910-devices-to-allocate")
	input := device.EncodePodSingleDevice(device.PodSingleDevice{
		{},
		{cd("uuid1", "Ascend910", 1024, 4)},
		{cd("uuid2", "Ascend910", 2048, 8)},
	})
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"hami.io/Ascend910-devices-to-allocate": input,
			},
		},
	}

	podSingleDev, err := ps.decodeDeviceAnnotations(pod)
	if err != nil {
		t.Fatalf("unexpected error decoding: %v", err)
	}

	got, err := ps.popNextContainerDevices(podSingleDev)
	if err != nil {
		t.Fatalf("unexpected error popping: %v", err)
	}
	if got[0].UUID != "uuid1" {
		t.Fatalf("device UUID = %q, want uuid1", got[0].UUID)
	}

	// Pop again should return uuid2
	got2, err := ps.popNextContainerDevices(podSingleDev)
	if err != nil {
		t.Fatalf("unexpected error on second pop: %v", err)
	}
	if got2[0].UUID != "uuid2" {
		t.Fatalf("device UUID = %q, want uuid2", got2[0].UUID)
	}

	// Third pop should fail
	_, err = ps.popNextContainerDevices(podSingleDev)
	if err == nil {
		t.Fatal("expected error on third pop, got nil")
	}
}

// ============================================================================
// Benchmarks
// ============================================================================

func BenchmarkBuildRuntimeInfoLookup(b *testing.B) {
	ps := newTestPluginServer("huawei.com/Ascend910", "hami.io/Ascend910-devices-to-allocate")

	sizes := []int{1, 8, 64}
	for _, n := range sizes {
		mem := int64(32768)
		core := int32(10)
		rtInfo := make([]RuntimeInfo, n)
		for i := range rtInfo {
			rtInfo[i] = RuntimeInfo{UUID: fmt.Sprintf("uuid-%d", i), Temp: fmt.Sprintf("vir%02d", i), Memory: &mem, Core: &core}
		}
		data, _ := json.Marshal(rtInfo)
		pod := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"huawei.com/Ascend910": string(data)},
			},
		}

		b.Run(fmt.Sprintf("entries=%d", n), func(b *testing.B) {
			for b.Loop() {
				_, err := ps.buildRuntimeInfoLookup(pod)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkBuildContainerAllocateResponse(b *testing.B) {
	b.Run("SingleDevice", func(b *testing.B) {
		ps := &PluginServer{
			mgr: &FakeManager{
				GetDeviceByUUIDFunc: func(uuid string) *manager.Device {
					return &manager.Device{UUID: uuid, PhyID: 3}
				},
			},
			allocAnno: "huawei.com/Ascend910",
		}
		rtInfoLookup := map[string]RuntimeInfo{
			"uuid1": {UUID: "uuid1", Temp: "vir01"},
		}
		pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{}}}
		containerDevs := device.ContainerDevices{cd("uuid1", "Ascend910", 1024, 4)}

		for b.Loop() {
			_, err := ps.buildContainerAllocateResponse(pod, containerDevs, rtInfoLookup)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("MultiDevice_8", func(b *testing.B) {
		ps := &PluginServer{
			mgr: &FakeManager{
				GetDeviceByUUIDFunc: func(uuid string) *manager.Device {
					return &manager.Device{UUID: uuid, PhyID: 0}
				},
			},
			allocAnno: "huawei.com/Ascend910",
		}
		const n = 8
		rtInfoLookup := make(map[string]RuntimeInfo, n)
		containerDevs := make(device.ContainerDevices, n)
		for i := range n {
			uuid := fmt.Sprintf("uuid%d", i)
			rtInfoLookup[uuid] = RuntimeInfo{UUID: uuid, Temp: "vir01"}
			containerDevs[i] = cd(uuid, "Ascend910", 1024, 4)
		}
		pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{}}}

		for b.Loop() {
			_, err := ps.buildContainerAllocateResponse(pod, containerDevs, rtInfoLookup)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("HamiCore_8Devices", func(b *testing.B) {
		ps := &PluginServer{
			mgr: &FakeManager{
				GetDeviceByUUIDFunc: func(uuid string) *manager.Device {
					return &manager.Device{UUID: uuid, PhyID: 0}
				},
			},
			allocAnno: "huawei.com/Ascend910",
		}
		const n = 8
		mem := int64(32768)
		core := int32(10)
		rtInfoLookup := make(map[string]RuntimeInfo, n)
		containerDevs := make(device.ContainerDevices, n)
		for i := range n {
			uuid := fmt.Sprintf("uuid%d", i)
			rtInfoLookup[uuid] = RuntimeInfo{UUID: uuid, Temp: "vir01", Memory: &mem, Core: &core}
			containerDevs[i] = cd(uuid, "Ascend910", 1024, 4)
		}
		pod := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{VNPUModeAnnotation: VNPUModeHamiCore},
			},
		}

		for b.Loop() {
			_, err := ps.buildContainerAllocateResponse(pod, containerDevs, rtInfoLookup)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
