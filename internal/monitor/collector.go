package monitor

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/klog/v2"
)

var (
	hostGPUdesc = prometheus.NewDesc(
		"hami_host_gpu_memory_used_bytes",
		"GPU device memory usage in bytes",
		[]string{"device_index", "device_uuid", "device_type"}, nil,
	)

	hostGPUUtilizationdesc = prometheus.NewDesc(
		"hami_host_gpu_utilization_ratio",
		"GPU core utilization ratio (0-100)",
		[]string{"device_index", "device_uuid", "device_type"}, nil,
	)

	ctrvGPUdesc = prometheus.NewDesc(
		"hami_vgpu_memory_used_bytes",
		"vGPU device memory usage in bytes",
		[]string{"namespace", "pod", "container", "vdevice_index", "device_uuid"}, nil,
	)

	ctrvGPUlimitdesc = prometheus.NewDesc(
		"hami_vgpu_memory_limit_bytes",
		"vGPU device memory limit in bytes",
		[]string{"namespace", "pod", "container", "vdevice_index", "device_uuid"}, nil,
	)

	ctrDeviceUtilizationdesc = prometheus.NewDesc(
		"hami_container_device_utilization_ratio",
		"Container device SM utilization ratio",
		[]string{"namespace", "pod", "container", "vdevice_index", "device_uuid"}, nil,
	)

	ctrDeviceMemoryContextDesc = prometheus.NewDesc(
		"hami_vgpu_memory_context_bytes",
		"Container device memory context size in bytes",
		[]string{"namespace", "pod", "container", "vdevice_index", "device_uuid"}, nil,
	)

	ctrDeviceMemoryModuleDesc = prometheus.NewDesc(
		"hami_vgpu_memory_module_bytes",
		"Container device memory module size in bytes",
		[]string{"namespace", "pod", "container", "vdevice_index", "device_uuid"}, nil,
	)

	ctrDeviceMemoryBufferDesc = prometheus.NewDesc(
		"hami_vgpu_memory_buffer_bytes",
		"Container device memory buffer size in bytes",
		[]string{"namespace", "pod", "container", "vdevice_index", "device_uuid"}, nil,
	)
)

type vNPUCollector struct {
	containersPath  string
	lister          *ContainerLister
	skipHostMetrics bool
}

func newVNPUCollector(containersPath string) (*vNPUCollector, error) {
	lister, err := NewContainerLister(containersPath)
	if err != nil {
		return nil, fmt.Errorf("new container lister: %w", err)
	}
	return &vNPUCollector{
		containersPath: containersPath,
		lister:         lister,
	}, nil
}

func (c *vNPUCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- hostGPUdesc
	ch <- hostGPUUtilizationdesc
	ch <- ctrvGPUdesc
	ch <- ctrvGPUlimitdesc
	ch <- ctrDeviceUtilizationdesc
	ch <- ctrDeviceMemoryContextDesc
	ch <- ctrDeviceMemoryModuleDesc
	ch <- ctrDeviceMemoryBufferDesc
}

func (c *vNPUCollector) Collect(ch chan<- prometheus.Metric) {
	klog.V(4).Info("Collecting vNPU metrics")

	hostDevices, err := collectHostDeviceStats()
	if err != nil {
		klog.Errorf("Host device stats: %v", err)
	}

	c.collectPodMetrics(ch, hostDevices)
	if !c.skipHostMetrics {
		c.collectHostMetrics(ch, hostDevices)
	}
}

func formatDeviceType(deviceType string) string {
	if strings.HasPrefix(deviceType, "Ascend-") || strings.HasPrefix(deviceType, "NVIDIA-") {
		return deviceType
	}
	return "Ascend-" + deviceType
}

func (c *vNPUCollector) collectHostMetrics(ch chan<- prometheus.Metric, devices []DeviceStat) {
	for _, d := range devices {
		hostUsed := float64(d.MemoryUsed) * 1024 * 1024
		labels := []string{fmt.Sprint(d.Index), d.UUID, formatDeviceType(d.DeviceType)}
		if d.MemoryTotal > 0 {
			ch <- prometheus.MustNewConstMetric(hostGPUdesc, prometheus.GaugeValue, hostUsed, labels...)
		}
		ch <- prometheus.MustNewConstMetric(hostGPUUtilizationdesc, prometheus.GaugeValue, float64(d.AICorePct), labels...)
	}
}

func (c *vNPUCollector) collectPodMetrics(ch chan<- prometheus.Metric, devices []DeviceStat) {
	entries, err := c.lister.ListContainers(devices)
	if err != nil {
		klog.Errorf("List containers: %v", err)
		return
	}

	utilByUUID := make(map[string]float64)
	for _, d := range devices {
		utilByUUID[d.UUID] = float64(d.AICorePct)
	}

	for _, e := range entries {
		memoryContextSize := uint64(0)
		memoryModuleSize := uint64(0)
		memoryBufferSize := uint64(0)

		for _, device := range e.Devices {
			baseLabels := []string{
				e.Namespace,
				e.PodName,
				e.ContainerName,
				strconv.Itoa(device.Index),
				device.UUID,
			}

			ch <- prometheus.MustNewConstMetric(ctrvGPUdesc, prometheus.GaugeValue, float64(device.MemoryUsed), baseLabels...)
			ch <- prometheus.MustNewConstMetric(ctrvGPUlimitdesc, prometheus.GaugeValue, float64(device.MemoryLimit), baseLabels...)
			ch <- prometheus.MustNewConstMetric(ctrDeviceUtilizationdesc, prometheus.GaugeValue, utilByUUID[device.UUID], baseLabels...)
			ch <- prometheus.MustNewConstMetric(ctrDeviceMemoryContextDesc, prometheus.GaugeValue, float64(memoryContextSize), baseLabels...)
			ch <- prometheus.MustNewConstMetric(ctrDeviceMemoryModuleDesc, prometheus.GaugeValue, float64(memoryModuleSize), baseLabels...)
			ch <- prometheus.MustNewConstMetric(ctrDeviceMemoryBufferDesc, prometheus.GaugeValue, float64(memoryBufferSize), baseLabels...)
		}
	}
}
