/*
 * Copyright 2026 The HAMi Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/Project-HAMi/ascend-device-plugin/internal/manager"
)

func managerTestAllocation() enpuManagerAllocation {
	return enpuManagerAllocation{
		PhyID: 15, VnpuID: 3, PodUID: "test-pod-uid", ContainerName: "vllm",
		ShmID: "test-die-shm", AICoreQuota: 20, HBMRequest: 16384, HBMLimit: 60000, SchedPolicy: 2,
	}
}

func managerTestPod() *v1.Pod {
	return &v1.Pod{ObjectMeta: metav1.ObjectMeta{UID: types.UID("test-pod-uid"), Name: "test-pod", Namespace: "test"}}
}

func managerTestConfig(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("ENPU_MANAGER_CONFIG_ROOT", root)
	path := filepath.Join(root, "vcann-rt", "test-pod-uid", "vllm", "npu_info.config")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("physical-npu-id=15\nvirtual-npu-id=3\naicore-quota=20\nmemory-request=16384\nmemory-limit=60000\nshm-id=test-die-shm\nscheduling-policy=2\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAllocateENPUManagerRetry(t *testing.T) {
	for _, damagedFirstResponse := range []bool{false, true} {
		name := "completed first allocation"
		if damagedFirstResponse {
			name = "manager committed but first response was truncated"
		}
		t.Run(name, func(t *testing.T) {
			configPath := managerTestConfig(t)
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/api/v1/allocate" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
					return
				}
				var req struct {
					PhyID         int32  `json:"phy_id"`
					PodUID        string `json:"pod_uid"`
					ContainerName string `json:"container_name"`
					Core          int32  `json:"aicore_quota"`
					Request       int64  `json:"hbm_request"`
					Limit         int64  `json:"hbm_limit"`
					Policy        int    `json:"sched_policy"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Error(err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if req.PhyID != 15 || req.PodUID != "test-pod-uid" || req.ContainerName != "vllm" || req.Core != 20 || req.Request != 16384 || req.Limit != 60000 || req.Policy != 2 {
					t.Errorf("allocation request changed: %+v", req)
				}
				call := calls.Add(1)
				if call == 1 && damagedFirstResponse {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("{"))
					return
				}
				allocation := managerTestAllocation()
				if call > 1 {
					allocation.Result = -4
					allocation.ErrorMsg = "vNPU already exists"
					w.WriteHeader(http.StatusBadRequest)
				}
				_ = json.NewEncoder(w).Encode(allocation)
			}))
			defer srv.Close()
			t.Setenv("ENPU_MANAGER_URL", srv.URL)
			dev := &manager.Device{PhyID: 15}
			first, err := allocateENPUManager(managerTestPod(), "vllm", dev, 16384, 60000, 20, 2)
			if damagedFirstResponse {
				if err == nil {
					t.Fatal("expected failure for truncated first response")
				}
			} else if err != nil || first.VnpuID != 3 {
				t.Fatalf("first allocation: %+v, %v", first, err)
			}
			retry, err := allocateENPUManager(managerTestPod(), "vllm", dev, 16384, 60000, 20, 2)
			if err != nil {
				t.Fatalf("retry must reuse official ALREADY_EXISTS response: %v", err)
			}
			if retry.VnpuID != 3 || retry.ShmID != "test-die-shm" || retry.PhyID != 15 || managerConfigPath(retry) != configPath || calls.Load() != 2 {
				t.Fatalf("retry changed allocation or configuration: %+v, calls=%d", retry, calls.Load())
			}
		})
	}
}

func TestAllocateENPUManagerRejectsMismatchedExistingAllocation(t *testing.T) {
	for _, tc := range []struct {
		name      string
		change    func(*enpuManagerAllocation)
		wantError string
	}{
		{"physical die", func(a *enpuManagerAllocation) { a.PhyID = 14 }, "HAMi selected"},
		{"pod identity", func(a *enpuManagerAllocation) { a.PodUID = "different-pod" }, "returned identity"},
		{"missing pod identity", func(a *enpuManagerAllocation) { a.PodUID = "" }, "returned identity"},
		{"container identity", func(a *enpuManagerAllocation) { a.ContainerName = "other" }, "returned identity"},
		{"core", func(a *enpuManagerAllocation) { a.AICoreQuota = 30 }, "does not match"},
		{"request", func(a *enpuManagerAllocation) { a.HBMRequest = 8192 }, "does not match"},
		{"limit", func(a *enpuManagerAllocation) { a.HBMLimit = 32768 }, "does not match"},
		{"policy", func(a *enpuManagerAllocation) { a.SchedPolicy = 3 }, "does not match"},
		{"missing shared memory", func(a *enpuManagerAllocation) { a.ShmID = "" }, "invalid runtime identity"},
		{"negative virtual id", func(a *enpuManagerAllocation) { a.VnpuID = -1 }, "invalid runtime identity"},
		{"virtual id out of range", func(a *enpuManagerAllocation) { a.VnpuID = 128 }, "invalid runtime identity"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			managerTestConfig(t)
			allocation := managerTestAllocation()
			allocation.Result = -4
			tc.change(&allocation)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(allocation)
			}))
			defer srv.Close()
			t.Setenv("ENPU_MANAGER_URL", srv.URL)
			_, err := allocateENPUManager(managerTestPod(), "vllm", &manager.Device{PhyID: 15}, 16384, 60000, 20, 2)
			if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("error=%v; want containing %q", err, tc.wantError)
			}
		})
	}
}

func TestAllocateENPUManagerDoesNotAcceptOtherFailures(t *testing.T) {
	for _, tc := range []struct {
		name         string
		status       int
		result       int
		removeConfig bool
		wantError    string
	}{
		{"resource failure", http.StatusBadRequest, -5, false, "allocation failed"},
		{"server failure despite already-exists body", http.StatusInternalServerError, -4, false, "allocation failed"},
		{"missing config", http.StatusBadRequest, -4, true, "config"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			configPath := managerTestConfig(t)
			if tc.removeConfig {
				if err := os.Remove(configPath); err != nil {
					t.Fatal(err)
				}
			}
			allocation := managerTestAllocation()
			allocation.Result = tc.result
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_ = json.NewEncoder(w).Encode(allocation)
			}))
			defer srv.Close()
			t.Setenv("ENPU_MANAGER_URL", srv.URL)
			_, err := allocateENPUManager(managerTestPod(), "vllm", &manager.Device{PhyID: 15}, 16384, 60000, 20, 2)
			if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("error=%v; want containing %q", err, tc.wantError)
			}
		})
	}
}
