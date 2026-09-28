package monitor

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/klog/v2"
)

// StartMetricsServer starts the hami-core metrics endpoint.
func StartMetricsServer(bindAddr string, containersPath string) {
	StartMetricsServerForModes(bindAddr, containersPath, true, false)
}

// StartMetricsServerForModes serves metrics for the enabled Ascend backends.
func StartMetricsServerForModes(bindAddr, containersPath string, hamiCore, enpu bool) {
	reg := prometheus.NewRegistry()
	var collectors ascendCollectors
	var coreCollector *vNPUCollector
	if hamiCore {
		collector, err := newVNPUCollector(containersPath)
		if err != nil {
			klog.Errorf("Failed to create vNPU collector: %v", err)
		} else {
			coreCollector = collector
			collectors = append(collectors, collector)
		}
	}
	if enpu {
		collector, err := newENPUCollector()
		if err != nil {
			klog.Errorf("Failed to create ENPU collector: %v", err)
		} else {
			collectors = append(collectors, collector)
			if coreCollector != nil {
				coreCollector.skipHostMetrics = true
			}
		}
	}
	if len(collectors) == 0 {
		return
	}
	reg.MustRegister(collectors)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	go func() {
		klog.Infof("vNPU monitor metrics server starting on %s", bindAddr)
		if err := http.ListenAndServe(bindAddr, mux); err != nil {
			klog.Errorf("vNPU monitor metrics server error: %v", err)
		}
	}()
}

type ascendCollectors []prometheus.Collector

func (collectors ascendCollectors) Describe(ch chan<- *prometheus.Desc) {
	for _, collector := range collectors {
		collector.Describe(ch)
	}
}

func (collectors ascendCollectors) Collect(ch chan<- prometheus.Metric) {
	for _, collector := range collectors {
		collector.Collect(ch)
	}
}
