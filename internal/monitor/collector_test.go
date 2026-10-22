package monitor

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

type coreCollectorFixture struct {
	collector *vNPUCollector
	devices   []DeviceStat
}

func (f coreCollectorFixture) Describe(ch chan<- *prometheus.Desc) { f.collector.Describe(ch) }
func (f coreCollectorFixture) Collect(ch chan<- prometheus.Metric) {
	f.collector.collectPodMetrics(ch, f.devices)
	f.collector.collectHostMetrics(ch, f.devices)
}

func gatherCoreMetrics(t *testing.T, collector *vNPUCollector, devices []DeviceStat) map[string]*dto.MetricFamily {
	t.Helper()
	registry := prometheus.NewPedanticRegistry()
	registry.MustRegister(coreCollectorFixture{collector: collector, devices: devices})
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

func TestCoreMetricsAttributePhysicalCardsAndPreserveHardwareHBM(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "pod_model")
	writeCoreShmem(t, filepath.Join(dir, localShmemName+".device-7"), 7000, true,
		map[int]coreProcessFixture{0: {pid: 100, active: 1, memory: [hbmDevices]uint64{700, 90000}}})
	writeCoreShmem(t, filepath.Join(dir, localShmemName+".device-3"), 3000, true,
		map[int]coreProcessFixture{0: {pid: 100, active: 1, memory: [hbmDevices]uint64{300, 80000}}})
	lister, _ := coreMonitorLister(t, root, coreMonitorPod(t, "pod", []string{"model"}, []string{"die-3", "die-7"}))
	collector := &vNPUCollector{lister: lister}
	devices := []DeviceStat{
		{Index: 1, PhysicalID: 7, UUID: "die-7", DeviceType: "910B", MemoryUsed: 1700, MemoryTotal: 65536, AICorePct: 71},
		{Index: 5, PhysicalID: 3, UUID: "die-3", DeviceType: "910B", MemoryUsed: 1300, MemoryTotal: 65536, AICorePct: 31},
	}
	metrics := gatherCoreMetrics(t, collector, devices)
	for name, expected := range map[string]map[string]float64{
		"hami_vgpu_memory_used_bytes":             {"die-7": 700, "die-3": 300},
		"hami_vgpu_memory_limit_bytes":            {"die-7": 7000, "die-3": 3000},
		"hami_container_device_utilization_ratio": {"die-7": 71, "die-3": 31},
	} {
		family := metrics[name]
		if family == nil || len(family.Metric) != 2 {
			t.Fatalf("%s: got %v", name, family)
		}
		for _, metric := range family.Metric {
			labels := make(map[string]string)
			for _, label := range metric.GetLabel() {
				labels[label.GetName()] = label.GetValue()
			}
			uuid := labels["device_uuid"]
			wantIndex := map[string]string{"die-7": "7", "die-3": "3"}[uuid]
			wantLabels := map[string]string{"namespace": "test", "pod": "pod", "container": "model",
				"device_uuid": uuid, "vdevice_index": wantIndex}
			if !reflect.DeepEqual(labels, wantLabels) || wantIndex == "" || metric.GetGauge().GetValue() != expected[uuid] {
				t.Errorf("%s labels=%v value=%v; expected physical ID and %v", name, labels, metric.GetGauge().GetValue(), expected)
			}
		}
	}
	for _, metric := range metrics["hami_host_gpu_memory_used_bytes"].Metric {
		labels := make(map[string]string)
		for _, label := range metric.GetLabel() {
			labels[label.GetName()] = label.GetValue()
		}
		wantMiB := map[string]float64{"die-7": 1700, "die-3": 1300}[labels["device_uuid"]]
		wantIndex := map[string]string{"die-7": "1", "die-3": "5"}[labels["device_uuid"]]
		if metric.GetGauge().GetValue() != wantMiB*1024*1024 || labels["device_index"] != wantIndex {
			t.Errorf("hardware HBM/index replaced by container ledger: %v", metric)
		}
	}
	if err := os.Remove(filepath.Join(dir, localShmemName+".device-7")); err != nil {
		t.Fatal(err)
	}
	metrics = gatherCoreMetrics(t, collector, devices)
	if family := metrics["hami_vgpu_memory_used_bytes"]; family == nil || len(family.Metric) != 1 {
		t.Fatalf("deleted per-card domain retained a metric: %v", family)
	}
}

func TestCoreUnknownDeviceDoesNotExportGuessedMetrics(t *testing.T) {
	dir := t.TempDir()
	for _, suffix := range []string{"7", "3", "99", "07", "invalid"} {
		writeCoreShmem(t, filepath.Join(dir, localShmemName+".device-"+suffix), 9999, true,
			map[int]coreProcessFixture{0: {pid: 100, active: 1, memory: [hbmDevices]uint64{9999}}})
	}
	writeCoreShmem(t, filepath.Join(dir, localShmemName), 9999, true, nil)
	// Physical 7 cannot be resolved, physical 3 is not in the pod allowlist,
	// and noncanonical/unknown suffixes cannot fall back to stale legacy data.
	if got := readContainerDevices(dir, []string{"die-7"}, map[int]string{3: "die-3"}); len(got) != 0 {
		t.Fatalf("unresolved domains exported guessed metrics: %+v", got)
	}
}

func TestCoreHostMemoryDistinguishesUnavailableFromEmpty(t *testing.T) {
	lister, _ := coreMonitorLister(t, t.TempDir())
	metrics := gatherCoreMetrics(t, &vNPUCollector{lister: lister}, []DeviceStat{
		{Index: 0, UUID: "unavailable"},
		{Index: 1, UUID: "empty", MemoryTotal: 65536},
	})
	family := metrics["hami_host_gpu_memory_used_bytes"]
	if family == nil || len(family.Metric) != 1 || family.Metric[0].GetGauge().GetValue() != 0 {
		t.Fatalf("unavailable memory became zero or valid zero disappeared: %v", family)
	}
	for _, label := range family.Metric[0].GetLabel() {
		if label.GetName() == "device_uuid" && label.GetValue() != "empty" {
			t.Fatalf("exported unavailable memory: %v", family)
		}
	}
}
