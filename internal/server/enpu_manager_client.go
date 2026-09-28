package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	"github.com/Project-HAMi/ascend-device-plugin/internal/manager"
)

// enpuManagerAllocation holds the manager-assigned identity and quotas.
type enpuManagerAllocation struct {
	PhyID         int32  `json:"phy_id"`
	VnpuID        int    `json:"vnpu_id"`
	PodUID        string `json:"pod_uid"`
	ContainerName string `json:"container_name"`
	ShmID         string `json:"shm_id"`
	AICoreQuota   int32  `json:"aicore_quota"`
	HBMRequest    int64  `json:"hbm_quota"`
	HBMLimit      int64  `json:"hbm_limit"`
	SchedPolicy   int    `json:"sched_policy"`
	Result        int    `json:"result"`
	ErrorMsg      string `json:"error_msg"`
}

func enpuManagerEndpoint() string {
	return strings.TrimRight(strings.TrimSpace(os.Getenv("ENPU_MANAGER_URL")), "/")
}

func enpuManagerConfigRoot() string {
	if root := strings.TrimSpace(os.Getenv("ENPU_MANAGER_CONFIG_ROOT")); root != "" {
		return root
	}
	return enpuHostPath("ENPU_CONFIG_ROOT", defaultENPUConfigRoot)
}

func podIdentity(pod *v1.Pod) string {
	if pod == nil {
		return "pod"
	}
	if pod.UID != "" {
		return string(pod.UID)
	}
	return pod.Namespace + "-" + pod.Name
}

func managerConfigPath(allocation *enpuManagerAllocation) string {
	root := filepath.Clean(enpuManagerConfigRoot())
	if filepath.Base(root) != "vcann-rt" {
		root = filepath.Join(root, "vcann-rt")
	}
	return filepath.Join(root,
		sanitizeENPUName(allocation.PodUID),
		sanitizeENPUName(allocation.ContainerName), "npu_info.config")
}

// allocateENPUManager registers the HAMi-selected DIE with enpu-manager.
func allocateENPUManager(pod *v1.Pod, ctrName string, dev *manager.Device, request, limit int64, core int32, policy int) (*enpuManagerAllocation, error) {
	endpoint := enpuManagerEndpoint()
	if endpoint == "" {
		return nil, nil
	}
	if dev == nil {
		return nil, fmt.Errorf("ENPU manager allocation requires a device")
	}
	uid := podIdentity(pod)
	if ctrName == "" {
		ctrName = "container"
	}
	payload := map[string]any{
		"pod_uid":        uid,
		"container_name": ctrName,
		"aicore_quota":   core,
		"hbm_request":    request,
		"hbm_limit":      limit,
		"sched_policy":   policy,
		"swap_priority":  1,
		"phy_id":         dev.PhyID,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal ENPU manager allocation: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"/api/v1/allocate", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create ENPU manager request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call ENPU manager at %s: %w", endpoint, err)
	}
	defer func() { _ = resp.Body.Close() }()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read ENPU manager response: %w", err)
	}
	var allocation enpuManagerAllocation
	if err := json.Unmarshal(responseBody, &allocation); err != nil {
		return nil, fmt.Errorf("decode ENPU manager response (HTTP %d): %w", resp.StatusCode, err)
	}
	alreadyExists := resp.StatusCode == http.StatusBadRequest && allocation.Result == -4
	if !alreadyExists && (resp.StatusCode/100 != 2 || allocation.Result != 0) {
		return nil, fmt.Errorf("ENPU manager allocation failed (HTTP %d, result=%d): %s", resp.StatusCode, allocation.Result, allocation.ErrorMsg)
	}
	if allocation.PodUID != uid || allocation.ContainerName != ctrName {
		return nil, fmt.Errorf("ENPU manager returned identity %q/%q, requested %q/%q", allocation.PodUID, allocation.ContainerName, uid, ctrName)
	}
	if allocation.PhyID != dev.PhyID {
		return nil, fmt.Errorf("ENPU manager returned phy_id=%d, HAMi selected phy_id=%d", allocation.PhyID, dev.PhyID)
	}
	if allocation.AICoreQuota != core || allocation.HBMRequest != request || allocation.HBMLimit != limit || allocation.SchedPolicy != policy {
		return nil, fmt.Errorf("ENPU manager allocation does not match HAMi request: returned core=%d hbm_request=%d hbm_limit=%d policy=%d, requested core=%d hbm_request=%d hbm_limit=%d policy=%d",
			allocation.AICoreQuota, allocation.HBMRequest, allocation.HBMLimit, allocation.SchedPolicy, core, request, limit, policy)
	}
	if allocation.VnpuID < 0 || allocation.VnpuID >= 128 || allocation.ShmID == "" {
		return nil, fmt.Errorf("ENPU manager returned invalid runtime identity vnpu_id=%d shm_id=%q", allocation.VnpuID, allocation.ShmID)
	}
	configPath := managerConfigPath(&allocation)
	if _, err := os.Stat(configPath); err != nil {
		return nil, fmt.Errorf("ENPU manager allocation succeeded but config %s is unavailable: %w", configPath, err)
	}
	klog.Infof("registered ENPU allocation with manager: pod=%s container=%s phy=%d vnpu=%d shm=%s config=%s", allocation.PodUID, allocation.ContainerName, allocation.PhyID, allocation.VnpuID, allocation.ShmID, configPath)
	return &allocation, nil
}
