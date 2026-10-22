package monitor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

type ContainerInfo struct {
	Namespace     string
	PodName       string
	ContainerName string
	PodUID        string
}

type ContainerEntry struct {
	PodUID        string
	ContainerName string
	Namespace     string
	PodName       string
	Devices       []ContainerDeviceStats
}

type ContainerDeviceStats struct {
	// Physical device ID for per-device domains; legacy files retain their
	// historical annotation/column ordinal as vdevice_index.
	Index       int
	UUID        string
	MemoryUsed  uint64
	MemoryLimit uint64
}

type ContainerLister struct {
	containersPath string
	mutex          sync.Mutex
	clientset      *kubernetes.Clientset
	nodeName       string

	informerFactory informers.SharedInformerFactory
	podLister       corelisters.PodLister
	podListerSynced cache.InformerSynced
	stopCh          chan struct{}
}

func NewContainerLister(containersPath string) (*ContainerLister, error) {
	config, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	if err != nil {
		klog.Errorf("Failed to build kubeconfig: %v", err)
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Errorf("Failed to build clientset: %v", err)
		return nil, err
	}

	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		return nil, fmt.Errorf("env NODE_NAME not set")
	}

	lister := &ContainerLister{
		containersPath: containersPath,
		clientset:      clientset,
		nodeName:       nodeName,
		stopCh:         make(chan struct{}),
	}

	informerFactory := informers.NewSharedInformerFactoryWithOptions(
		clientset,
		0,
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.FieldSelector = fmt.Sprintf("spec.nodeName=%s", nodeName)
		}),
	)

	podInformer := informerFactory.Core().V1().Pods()
	lister.podLister = podInformer.Lister()
	lister.podListerSynced = podInformer.Informer().HasSynced
	lister.informerFactory = informerFactory

	informerFactory.Start(lister.stopCh)
	if !cache.WaitForCacheSync(lister.stopCh, lister.podListerSynced) {
		return nil, fmt.Errorf("failed to sync pod informer cache")
	}

	klog.Info("ContainerLister informer synced")
	return lister, nil
}

func (l *ContainerLister) Stop() {
	close(l.stopCh)
}

// ascendDeviceResource returns the Ascend device resource the container requests
// (also the pod-annotation key holding its device UUIDs), or "" if there is none.
func ascendDeviceResource(pod *corev1.Pod, ctrName string) string {
	for _, c := range pod.Spec.Containers {
		if c.Name != ctrName {
			continue
		}
		for _, rl := range []corev1.ResourceList{c.Resources.Limits, c.Resources.Requests} {
			for res := range rl {
				k := string(res)
				if strings.HasPrefix(k, "huawei.com/Ascend") &&
					!strings.HasSuffix(k, "-core") && !strings.HasSuffix(k, "-memory") {
					return k
				}
			}
		}
	}
	return ""
}

// ListContainers enumerates container directories and reads shmem from each.
// Directory format: {podUID}_{containerName}.
func (l *ContainerLister) ListContainers(devices []DeviceStat) ([]ContainerEntry, error) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	pods, err := l.podLister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}

	podByUID := make(map[string]*corev1.Pod, len(pods))
	for _, pod := range pods {
		podByUID[string(pod.UID)] = pod
	}

	entries, err := os.ReadDir(l.containersPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read containers dir: %w", err)
	}

	var result []ContainerEntry
	uuidByPhysical := make(map[int]string, len(devices))
	for _, device := range devices {
		if device.PhysicalID >= 0 && device.UUID != "" {
			uuidByPhysical[device.PhysicalID] = device.UUID
		}
	}
	for _, dirent := range entries {
		if !dirent.IsDir() {
			continue
		}
		name := dirent.Name()
		parts := strings.SplitN(name, "_", 2)
		if len(parts) != 2 {
			continue
		}
		podUID := parts[0]
		ctrName := parts[1]

		pod := podByUID[podUID]
		if pod == nil {
			klog.V(3).Infof("Stale container dir (pod gone): %s, removing", name)
			_ = os.RemoveAll(filepath.Join(l.containersPath, name))
			continue
		}
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			klog.V(3).Infof("Stale container dir (pod %s): %s, removing", pod.Status.Phase, name)
			_ = os.RemoveAll(filepath.Join(l.containersPath, name))
			continue
		}

		var devUUIDs []string
		if key := ascendDeviceResource(pod, ctrName); key != "" {
			if anno, ok := pod.Annotations[key]; ok {
				var devs []struct{ UUID string }
				if json.Unmarshal([]byte(anno), &devs) == nil {
					for _, d := range devs {
						if d.UUID != "" {
							devUUIDs = append(devUUIDs, d.UUID)
						}
					}
				}
			}
		}

		deviceStats := readContainerDevices(filepath.Join(l.containersPath, name), devUUIDs, uuidByPhysical)
		if len(deviceStats) == 0 {
			continue
		}
		result = append(result, ContainerEntry{
			PodUID:        podUID,
			ContainerName: ctrName,
			Namespace:     pod.Namespace,
			PodName:       pod.Name,
			Devices:       deviceStats,
		})
	}
	return result, nil
}

const localShmemName = "vnpu_local_shmem"

// New cores use a separate accounting domain for each physical NPU. The slot
// array retains its ABI, but only column zero belongs to that domain. The pod's
// runtime annotation is a UUID allowlist, not a per-container device ordering.
func readContainerDevices(dir string, devUUIDs []string, uuidByPhysical map[int]string) []ContainerDeviceStats {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	allowed := make(map[string]bool, len(devUUIDs))
	for _, uuid := range devUUIDs {
		allowed[uuid] = true
	}
	var result []ContainerDeviceStats
	hasDeviceDomains := false
	for _, file := range files {
		suffix, found := strings.CutPrefix(file.Name(), localShmemName+".device-")
		if !found {
			continue
		}
		// Never mix stale unsuffixed data with any new-format domain, including
		// one that is still being initialized or cannot currently be resolved.
		hasDeviceDomains = true
		physical, err := strconv.ParseUint(suffix, 10, 31)
		if err != nil || strconv.FormatUint(physical, 10) != suffix || !file.Type().IsRegular() {
			continue
		}
		uuid := uuidByPhysical[int(physical)]
		if uuid == "" || !allowed[uuid] {
			continue
		}
		if stats, ok := readContainerShmem(filepath.Join(dir, file.Name())); ok {
			result = append(result, ContainerDeviceStats{
				Index: int(physical), UUID: uuid, MemoryUsed: stats.memory[0], MemoryLimit: stats.limit,
			})
		}
	}
	if hasDeviceDomains {
		return result
	}
	// Preserve the legacy annotation/column ordering only for unsuffixed cores.
	// Those columns do not identify physical devices.
	if stats, ok := readContainerShmem(filepath.Join(dir, localShmemName)); ok {
		seen := make(map[string]bool)
		for i, uuid := range devUUIDs {
			if i >= len(stats.memory) {
				break
			}
			if uuid == "" || seen[uuid] {
				continue
			}
			seen[uuid] = true
			result = append(result, ContainerDeviceStats{
				Index: i, UUID: uuid, MemoryUsed: stats.memory[i], MemoryLimit: stats.limit,
			})
		}
	}
	return result
}

type containerShmemStats struct {
	memory [hbmDevices]uint64
	limit  uint64
}

func readContainerShmem(path string) (containerShmemStats, bool) {
	reader, err := OpenLocalShmem(path)
	if err != nil {
		klog.V(5).Infof("Skip shmem %s: %v", path, err)
		return containerShmemStats{}, false
	}
	defer func() { _ = reader.Close() }()
	if !reader.Initialized() {
		return containerShmemStats{}, false
	}
	return containerShmemStats{memory: reader.ReadMemoryByDevice(), limit: reader.ReadPodStats().MemoryLimit}, true
}
