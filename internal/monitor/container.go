package monitor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	Stats         PodStats
	DeviceMemory  [hbmDevices]uint64
	DeviceUUIDs   []string
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

// ListContainers enumerates container directories and reads shmem from each.
// Directory format: {podUID}_{containerName}.
func (l *ContainerLister) ListContainers() ([]ContainerEntry, error) {
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
			os.RemoveAll(filepath.Join(l.containersPath, name))
			continue
		}
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			klog.V(3).Infof("Stale container dir (pod %s): %s, removing", pod.Status.Phase, name)
			os.RemoveAll(filepath.Join(l.containersPath, name))
			continue
		}

		shmemPath := filepath.Join(l.containersPath, name, "vnpu_local_shmem")
		reader, err := OpenLocalShmem(shmemPath)
		if err != nil {
			klog.V(5).Infof("Skip shmem %s: %v", shmemPath, err)
			continue
		}

		var devUUIDs []string
		if anno, ok := pod.Annotations["huawei.com/Ascend310P"]; ok {
			var devs []struct{ UUID string }
			if json.Unmarshal([]byte(anno), &devs) == nil {
				for _, d := range devs {
					if d.UUID != "" {
						devUUIDs = append(devUUIDs, d.UUID)
					}
				}
			}
		}

		result = append(result, ContainerEntry{
			PodUID:        podUID,
			ContainerName: ctrName,
			Namespace:     pod.Namespace,
			PodName:       pod.Name,
			Stats:         reader.ReadPodStats(),
			DeviceMemory:  reader.ReadMemoryByDevice(),
			DeviceUUIDs:   devUUIDs,
		})
		reader.Close()
	}
	return result, nil
}
