package monitor

import (
	"ascend-common/devmanager/common"
	"errors"
	"maps"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func sampleAllocation(name, container string) enpuAllocation {
	return enpuAllocation{Namespace: "test", PodName: name, PodUID: name, ContainerName: "model", ContainerID: container, DeviceUUID: "die-15", Policy: "best-effort", PhysicalID: 15, VirtualID: 1, CoreQuota: 30, MemoryRequest: 256 * 1048576, MemoryLimit: 4096 * 1048576}
}

func gatherENPU(t *testing.T, collector *enpuCollector) map[string]*dto.MetricFamily {
	t.Helper()
	registry := prometheus.NewPedanticRegistry()
	registry.MustRegister(collector)
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string]*dto.MetricFamily)
	for _, family := range families {
		result[family.GetName()] = family
	}
	return result
}

func TestENPUHostMetricDeviceTypes(t *testing.T) {
	for _, tc := range []struct {
		deviceType, want string
	}{
		{"Ascend910C", "Ascend-910C"},
		{"Ascend910_9392", "Ascend-910_9392"},
		{"Ascend-910C", "Ascend-910C"},
		{"NVIDIA-A100", "NVIDIA-A100"},
		{"910C", "Ascend-910C"},
	} {
		t.Run(tc.deviceType, func(t *testing.T) {
			used, utilization := 1500.0, 80.0
			metrics := gatherENPU(t, &enpuCollector{
				allocations: func() ([]enpuAllocation, error) { return nil, nil },
				devices: func([]enpuAllocation) ([]enpuDeviceSample, error) {
					return []enpuDeviceSample{{LogicID: 3, PhysicalID: 15, UUID: "die-15", DeviceType: tc.deviceType, MemoryUsed: &used, Utilization: &utilization}}, nil
				},
			})
			wantLabels := map[string]string{"device_index": "3", "device_uuid": "die-15", "device_type": tc.want}
			for name, wantValue := range map[string]float64{
				"hami_host_gpu_memory_used_bytes": used,
				"hami_host_gpu_utilization_ratio": utilization,
			} {
				family := metrics[name]
				if family == nil || len(family.Metric) != 1 {
					t.Fatalf("%s: got %v, want one metric", name, family)
				}
				metric := family.Metric[0]
				if got := metric.GetGauge().GetValue(); got != wantValue {
					t.Errorf("%s value = %v, want %v", name, got, wantValue)
				}
				gotLabels := make(map[string]string)
				for _, label := range metric.GetLabel() {
					gotLabels[label.GetName()] = label.GetValue()
				}
				if !maps.Equal(gotLabels, wantLabels) {
					t.Errorf("%s labels = %v, want %v", name, gotLabels, wantLabels)
				}
			}
		})
	}
}

func TestENPUContainerMetrics(t *testing.T) {
	used, utilization := 1500.0, 80.0
	allocations := []enpuAllocation{sampleAllocation("one", "id1"), sampleAllocation("two", "id2")}
	allocations[1].VirtualID = 2
	collector := &enpuCollector{
		allocations: func() ([]enpuAllocation, error) { return allocations, nil },
		devices: func([]enpuAllocation) ([]enpuDeviceSample, error) {
			return []enpuDeviceSample{{LogicID: 3, PhysicalID: 15, UUID: "die-15", DeviceType: "910C", MemoryUsed: &used, Utilization: &utilization, MemoryByContainer: map[string]float64{"id1": 100, "id2": 700, "unrelated": 500}}}, nil
		},
	}
	metrics := gatherENPU(t, collector)
	for name, expected := range map[string][]float64{
		"hami_vgpu_memory_used_bytes":         {100, 700},
		"hami_vgpu_memory_limit_bytes":        {4096 * 1048576, 4096 * 1048576},
		"hami_enpu_memory_request_bytes":      {256 * 1048576, 256 * 1048576},
		"hami_enpu_aicore_quota_percent":      {30, 30},
		"hami_enpu_memory_collection_success": {1, 1},
		"hami_host_gpu_memory_used_bytes":     {1500},
		"hami_host_gpu_utilization_ratio":     {80},
	} {
		family := metrics[name]
		if family == nil || len(family.Metric) != len(expected) {
			t.Fatalf("%s: got %v", name, family)
		}
		for i, metric := range family.Metric {
			if metric.GetGauge().GetValue() != expected[i] {
				t.Errorf("%s[%d] = %v, want %v", name, i, metric.GetGauge().GetValue(), expected[i])
			}
		}
	}
	for _, name := range []string{"hami_container_device_utilization_ratio", "hami_vgpu_memory_context_bytes", "hami_vgpu_memory_buffer_bytes", "hami_vgpu_memory_module_bytes"} {
		if metrics[name] != nil {
			t.Errorf("unavailable ENPU statistic exported: %s", name)
		}
	}
	allocations = allocations[:1]
	if got := len(gatherENPU(t, collector)["hami_vgpu_memory_used_bytes"].Metric); got != 1 {
		t.Fatalf("deleted allocation retained: %d", got)
	}
}

func TestENPUMemoryUnavailableIsNotZero(t *testing.T) {
	for _, test := range []struct {
		name    string
		devices []enpuDeviceSample
	}{
		{"query error", []enpuDeviceSample{{PhysicalID: 15, UUID: "die-15", MemoryErr: errors.New("DCMI error")}}},
		{"wrong physical", []enpuDeviceSample{{PhysicalID: 14, UUID: "die-15"}}},
		{"wrong UUID", []enpuDeviceSample{{PhysicalID: 15, UUID: "other"}}},
		{"missing device", nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			metrics := gatherENPU(t, &enpuCollector{
				allocations: func() ([]enpuAllocation, error) { return []enpuAllocation{sampleAllocation("one", "id1")}, nil },
				devices:     func([]enpuAllocation) ([]enpuDeviceSample, error) { return test.devices, errors.New("failed") },
			})
			if metrics["hami_vgpu_memory_used_bytes"] != nil {
				t.Fatal("failed attribution exported fabricated memory")
			}
			if got := metrics["hami_enpu_memory_collection_success"].Metric[0].GetGauge().GetValue(); got != 0 {
				t.Fatalf("collection success = %v", got)
			}
		})
	}
}

func TestENPUAndCoreDescriptorCompatibility(t *testing.T) {
	registry := prometheus.NewPedanticRegistry()
	if err := registry.Register(ascendCollectors{&vNPUCollector{}, &enpuCollector{}}); err != nil {
		t.Fatalf("ENPU conflicts with existing hami-core metric descriptors: %v", err)
	}
}

func writeProcessFixture(t *testing.T, root string, pid int32, container, start string) {
	t.Helper()
	dir := filepath.Join(root, strconv.Itoa(int(pid)))
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	stat := strconv.Itoa(int(pid)) + " (process name (with parentheses)) S " + strings.Repeat("0 ", 18) + start + " 0\n"
	if err := os.WriteFile(filepath.Join(dir, "stat"), []byte(stat), 0644); err != nil {
		t.Fatal(err)
	}
	cgroup := "0::/system.slice/example.service\n"
	if container != "" {
		cgroup = "0::/system.slice/docker-" + container + ".scope\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "cgroup"), []byte(cgroup), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestENPUProcessMemoryAggregation(t *testing.T) {
	root := t.TempDir()
	first, second := strings.Repeat("a", 64), strings.Repeat("b", 64)
	for pid, container := range map[int32]string{10: first, 11: first, 12: second, 13: ""} {
		writeProcessFixture(t, root, pid, container, "12345")
	}
	starts, err := enpuProcessStarts(root)
	if err != nil {
		t.Fatal(err)
	}
	info := &common.DevProcessInfo{ProcNum: 4, DevProcArray: []common.DevProcInfo{{Pid: 10, MemUsage: 1.5}, {Pid: 11, MemUsage: 2.25}, {Pid: 12, MemUsage: 5}, {Pid: 13, MemUsage: 9}}}
	used, err := enpuContainerMemory(info, root, starts)
	if err != nil {
		t.Fatal(err)
	}
	if len(used) != 2 || used[first] != 3.75*1048576 || used[second] != 5*1048576 {
		t.Fatalf("incorrect PID attribution or MiB conversion: %v", used)
	}
	writeProcessFixture(t, root, 10, second, "12346")
	if _, err := enpuContainerMemory(info, root, starts); err == nil {
		t.Fatal("PID reuse accepted")
	}
}

func TestENPUInvalidProcessSamples(t *testing.T) {
	root := t.TempDir()
	writeProcessFixture(t, root, 10, strings.Repeat("a", 64), "12345")
	starts, err := enpuProcessStarts(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, info := range []*common.DevProcessInfo{
		nil,
		{ProcNum: 1},
		{ProcNum: 1, DevProcArray: []common.DevProcInfo{{Pid: 11, MemUsage: 1}}},
		{ProcNum: 1, DevProcArray: []common.DevProcInfo{{Pid: 10, MemUsage: math.NaN()}}},
		{ProcNum: 1, DevProcArray: []common.DevProcInfo{{Pid: 10, MemUsage: -1}}},
		{ProcNum: 2, DevProcArray: []common.DevProcInfo{{Pid: 10, MemUsage: 1}, {Pid: 10, MemUsage: 1}}},
	} {
		if _, err := enpuContainerMemory(info, root, starts); err == nil {
			t.Errorf("invalid sample accepted: %+v", info)
		}
	}
	used, err := enpuContainerMemory(&common.DevProcessInfo{}, root, starts)
	if err != nil || len(used) != 0 {
		t.Fatalf("empty successful query = %v, %v", used, err)
	}
	if err := os.Remove(filepath.Join(root, "10", "cgroup")); err != nil {
		t.Fatal(err)
	}
	if _, err := enpuContainerMemory(&common.DevProcessInfo{ProcNum: 1, DevProcArray: []common.DevProcInfo{{Pid: 10, MemUsage: 1}}}, root, starts); err == nil {
		t.Fatal("missing cgroup exported as zero")
	}
}
