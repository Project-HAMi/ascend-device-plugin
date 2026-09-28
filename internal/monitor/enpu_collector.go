package monitor

import (
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
)

var enpuContainerLabels = []string{"namespace", "pod", "container", "vdevice_index", "device_uuid"}

var (
	enpuMemoryRequestDesc = prometheus.NewDesc("hami_enpu_memory_request_bytes", "ENPU scheduled memory reservation in bytes", enpuContainerLabels, nil)
	enpuCoreQuotaDesc     = prometheus.NewDesc("hami_enpu_aicore_quota_percent", "ENPU configured compute share; enforcement depends on scheduling policy", enpuContainerLabels, nil)
	enpuAllocationDesc    = prometheus.NewDesc("hami_enpu_allocation_info", "ENPU allocation identity and scheduling policy", append(append([]string{}, enpuContainerLabels...), "physical_device_id", "virtual_device_id", "policy"), nil)
	enpuMemorySuccessDesc = prometheus.NewDesc("hami_enpu_memory_collection_success", "Whether container memory was successfully attributed in this scrape", enpuContainerLabels, nil)
	enpuConfigSuccessDesc = prometheus.NewDesc("hami_enpu_config_collection_success", "Whether all running ENPU container allocations were read successfully", nil, nil)
	enpuDeviceSuccessDesc = prometheus.NewDesc("hami_enpu_device_collection_success", "Whether all ENPU device statistics were read successfully", nil, nil)
)

type enpuDeviceSample struct {
	LogicID, PhysicalID     int
	UUID, DeviceType        string
	MemoryUsed, Utilization *float64
	MemoryByContainer       map[string]float64
	MemoryErr               error
}

type enpuCollector struct {
	mu          sync.Mutex
	allocations func() ([]enpuAllocation, error)
	devices     func([]enpuAllocation) ([]enpuDeviceSample, error)
}

func newENPUCollector() (*enpuCollector, error) {
	lister, err := NewContainerLister("")
	if err != nil {
		return nil, err
	}
	root := os.Getenv("ENPU_CONFIG_ROOT")
	if root == "" {
		root = "/var/lib/hami-enpu"
	}
	managerRoot := ""
	if os.Getenv("ENPU_MANAGER_URL") != "" {
		managerRoot = os.Getenv("ENPU_MANAGER_CONFIG_ROOT")
		if managerRoot == "" {
			managerRoot = root
		}
	}
	return &enpuCollector{
		allocations: func() ([]enpuAllocation, error) {
			pods, err := lister.podLister.List(labels.Everything())
			if err != nil {
				return nil, err
			}
			return listENPUAllocations(pods, root, managerRoot)
		},
		devices: collectENPUDeviceStats,
	}, nil
}

func (c *enpuCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range []*prometheus.Desc{hostGPUdesc, hostGPUUtilizationdesc, ctrvGPUdesc, ctrvGPUlimitdesc, enpuMemoryRequestDesc, enpuCoreQuotaDesc, enpuAllocationDesc, enpuMemorySuccessDesc, enpuConfigSuccessDesc, enpuDeviceSuccessDesc} {
		ch <- desc
	}
}

func (c *enpuCollector) Collect(ch chan<- prometheus.Metric) {
	c.mu.Lock()
	defer c.mu.Unlock()
	allocations, configErr := c.allocations()
	if configErr != nil {
		klog.Warningf("ENPU allocation metrics: %v", configErr)
	}
	ch <- prometheus.MustNewConstMetric(enpuConfigSuccessDesc, prometheus.GaugeValue, enpuSuccess(configErr))
	devices, deviceErr := c.devices(allocations)
	if deviceErr != nil {
		klog.Warningf("ENPU device metrics: %v", deviceErr)
	}
	ch <- prometheus.MustNewConstMetric(enpuDeviceSuccessDesc, prometheus.GaugeValue, enpuSuccess(deviceErr))
	byPhysical := make(map[int]enpuDeviceSample, len(devices))
	for _, device := range devices {
		byPhysical[device.PhysicalID] = device
		values := []string{strconv.Itoa(device.LogicID), device.UUID, formatENPUDeviceType(device.DeviceType)}
		if device.MemoryUsed != nil {
			ch <- prometheus.MustNewConstMetric(hostGPUdesc, prometheus.GaugeValue, *device.MemoryUsed, values...)
		}
		if device.Utilization != nil {
			ch <- prometheus.MustNewConstMetric(hostGPUUtilizationdesc, prometheus.GaugeValue, *device.Utilization, values...)
		}
	}
	for _, allocation := range allocations {
		values := []string{allocation.Namespace, allocation.PodName, allocation.ContainerName, "0", allocation.DeviceUUID}
		device, found := byPhysical[allocation.PhysicalID]
		if !found || device.UUID != allocation.DeviceUUID {
			ch <- prometheus.MustNewConstMetric(enpuMemorySuccessDesc, prometheus.GaugeValue, 0, values...)
			continue
		}
		ch <- prometheus.MustNewConstMetric(ctrvGPUlimitdesc, prometheus.GaugeValue, allocation.MemoryLimit, values...)
		ch <- prometheus.MustNewConstMetric(enpuMemoryRequestDesc, prometheus.GaugeValue, allocation.MemoryRequest, values...)
		ch <- prometheus.MustNewConstMetric(enpuCoreQuotaDesc, prometheus.GaugeValue, allocation.CoreQuota, values...)
		info := append(append([]string{}, values...), strconv.Itoa(allocation.PhysicalID), strconv.Itoa(allocation.VirtualID), allocation.Policy)
		ch <- prometheus.MustNewConstMetric(enpuAllocationDesc, prometheus.GaugeValue, 1, info...)
		ch <- prometheus.MustNewConstMetric(enpuMemorySuccessDesc, prometheus.GaugeValue, enpuSuccess(device.MemoryErr), values...)
		if device.MemoryErr == nil {
			ch <- prometheus.MustNewConstMetric(ctrvGPUdesc, prometheus.GaugeValue, device.MemoryByContainer[allocation.ContainerID], values...)
		}
	}
}

// formatENPUDeviceType normalizes DCMI product and chip names for ENPU host metric labels.
func formatENPUDeviceType(deviceType string) string {
	if strings.HasPrefix(deviceType, "Ascend") && !strings.HasPrefix(deviceType, "Ascend-") {
		deviceType = strings.TrimPrefix(deviceType, "Ascend")
	}
	return formatDeviceType(deviceType)
}

func enpuSuccess(err error) float64 {
	if err != nil {
		return 0
	}
	return 1
}
