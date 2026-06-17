package monitor

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/klog/v2"
)

// StartMetricsServer starts a Prometheus metrics HTTP server.
// containersPath: path to the host directory containing per-container shmem dirs
// (e.g., /usr/local/hami-vnpu-core/containers).
func StartMetricsServer(bindAddr string, containersPath string) {
	collector, err := newVNPUCollector(containersPath)
	if err != nil {
		klog.Errorf("Failed to create vNPU collector: %v", err)
		return
	}

	reg := prometheus.NewRegistry()
	reg.MustRegister(collector)

	http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	go func() {
		klog.Infof("vNPU monitor metrics server starting on %s", bindAddr)
		if err := http.ListenAndServe(bindAddr, nil); err != nil {
			klog.Errorf("vNPU monitor metrics server error: %v", err)
		}
	}()
}
