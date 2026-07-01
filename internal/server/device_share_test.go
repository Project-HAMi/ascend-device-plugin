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
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Project-HAMi/ascend-device-plugin/internal/manager"
)

// withFakeNpuSmi swaps runNpuSmi for a fake and restores it after the test.
// Shared with the Allocate device-share tests in server_test.go.
func withFakeNpuSmi(t *testing.T, fn func(args ...string) ([]byte, error)) {
	t.Helper()
	orig := runNpuSmi
	runNpuSmi = fn
	t.Cleanup(func() { runNpuSmi = orig })
}

func sampleChips() []chipKey {
	return []chipKey{
		{Card: 0, Chip: 0},
		{Card: 0, Chip: 1},
		{Card: 1, Chip: 0},
	}
}

func TestApplyDeviceShare_Enable(t *testing.T) {
	var calls [][]string
	withFakeNpuSmi(t, func(args ...string) ([]byte, error) {
		calls = append(calls, append([]string(nil), args...))
		return nil, nil
	})

	if err := applyDeviceShare(sampleChips(), true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := [][]string{
		{"set", "-t", "device-share", "-i", "0", "-c", "0", "-d", "1"},
		{"set", "-t", "device-share", "-i", "0", "-c", "1", "-d", "1"},
		{"set", "-t", "device-share", "-i", "1", "-c", "0", "-d", "1"},
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls mismatch:\ngot  %v\nwant %v", calls, want)
	}
}

func TestApplyDeviceShare_Disable(t *testing.T) {
	var calls [][]string
	withFakeNpuSmi(t, func(args ...string) ([]byte, error) {
		calls = append(calls, append([]string(nil), args...))
		return nil, nil
	})

	if err := applyDeviceShare(sampleChips(), false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := [][]string{
		{"set", "-t", "device-share", "-i", "0", "-c", "0", "-d", "0"},
		{"set", "-t", "device-share", "-i", "0", "-c", "1", "-d", "0"},
		{"set", "-t", "device-share", "-i", "1", "-c", "0", "-d", "0"},
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls mismatch:\ngot  %v\nwant %v", calls, want)
	}
}

// TestApplyDeviceShare_FailFast checks that the first per-chip failure stops
// the loop, leaving later chips untouched.
func TestApplyDeviceShare_FailFast(t *testing.T) {
	var calls [][]string
	withFakeNpuSmi(t, func(args ...string) ([]byte, error) {
		calls = append(calls, append([]string(nil), args...))
		// Fail on card=0 chip=1 (the second call).
		if len(args) >= 7 && args[4] == "0" && args[6] == "1" {
			return []byte("E80001 not allowed"), fmt.Errorf("exit status 1")
		}
		return nil, nil
	})

	err := applyDeviceShare(sampleChips(), true)
	if err == nil {
		t.Fatal("expected error from per-chip failure, got nil")
	}
	if !strings.Contains(err.Error(), "-i 0") || !strings.Contains(err.Error(), "-c 1") {
		t.Fatalf("error should identify failing chip via npu-smi flags, got: %v", err)
	}
	if got := len(calls); got != 2 {
		t.Fatalf("expected loop to stop after first failure (2 calls), got %d: %v", got, calls)
	}
}

func TestApplyDeviceShare_NoChips(t *testing.T) {
	called := false
	withFakeNpuSmi(t, func(args ...string) ([]byte, error) {
		called = true
		return nil, nil
	})
	if err := applyDeviceShare(nil, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Fatal("npu-smi should not be invoked when chip list is empty")
	}
}

// TestRunNpuSmi_AnswersDeviceShareConfirmation exercises the real exec.Command
// path: the fake npu-smi prompts Y/N and exits 200 unless "Y" arrives on stdin.
func TestRunNpuSmi_AnswersDeviceShareConfirmation(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "npu-smi")
	script := "#!/bin/sh\n" +
		"echo 'There are security risks when opening device sharing,'\n" +
		"echo 'Are you sure you want to continue setting?(Y/N)'\n" +
		"read ans\n" +
		"[ \"$ans\" = \"Y\" ] || exit 200\n" +
		"echo ok\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake npu-smi: %v", err)
	}

	saved := npuSmiCandidates
	npuSmiCandidates = []string{fake}
	t.Cleanup(func() { npuSmiCandidates = saved })

	out, err := runNpuSmi("set", "-t", "device-share", "-i", "0", "-c", "0", "-d", "1")
	if err != nil {
		t.Fatalf("expected success after Y answer, got err=%v out=%q", err, out)
	}
	if !strings.Contains(string(out), "ok") {
		t.Fatalf("expected confirmation to be consumed and command to print ok, got %q", out)
	}
}

func TestEnableNodeDeviceShare_NotHamiVnpuCore(t *testing.T) {
	called := false
	withFakeNpuSmi(t, func(args ...string) ([]byte, error) {
		called = true
		return nil, nil
	})
	ps := &PluginServer{
		nodeName: "node-1",
		mgr: &FakeManager{
			IsHamiVnpuCoreFunc: func() bool { return false },
			GetDevicesFunc: func() []*manager.Device {
				return []*manager.Device{{CardID: 0, DeviceID: 0}}
			},
		},
	}
	if err := ps.enableNodeDeviceShare(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Fatal("npu-smi must not be invoked when node is not hami-vnpu-core")
	}
}

func TestEnableNodeDeviceShare_FlipsAllChipsDeduped(t *testing.T) {
	type ic struct{ card, chip string }
	seen := map[ic]int{}
	withFakeNpuSmi(t, func(args ...string) ([]byte, error) {
		// args: set -t device-share -i <card> -c <chip> -d <flag>
		if len(args) != 9 || args[8] != "1" {
			t.Errorf("unexpected npu-smi args: %v", args)
			return nil, nil
		}
		seen[ic{args[4], args[6]}]++
		return nil, nil
	})
	ps := &PluginServer{
		nodeName: "node-1",
		mgr: &FakeManager{
			IsHamiVnpuCoreFunc: func() bool { return true },
			GetDevicesFunc: func() []*manager.Device {
				return []*manager.Device{
					{CardID: 0, DeviceID: 0},
					{CardID: 0, DeviceID: 1},
					{CardID: 0, DeviceID: 0}, // duplicate chip — must be flipped once
					{CardID: 1, DeviceID: 0},
				}
			},
		},
	}
	if err := ps.enableNodeDeviceShare(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[ic]int{
		{"0", "0"}: 1,
		{"0", "1"}: 1,
		{"1", "0"}: 1,
	}
	if !reflect.DeepEqual(seen, want) {
		t.Fatalf("device-share calls mismatch:\ngot  %v\nwant %v", seen, want)
	}
}

func TestEnableNodeDeviceShare_FlipFailureFailsFast(t *testing.T) {
	withFakeNpuSmi(t, func(args ...string) ([]byte, error) {
		return []byte("E80001 not allowed"), fmt.Errorf("exit status 1")
	})
	ps := &PluginServer{
		nodeName: "node-1",
		mgr: &FakeManager{
			IsHamiVnpuCoreFunc: func() bool { return true },
			GetDevicesFunc: func() []*manager.Device {
				return []*manager.Device{{CardID: 0, DeviceID: 0}}
			},
		},
	}
	if err := ps.enableNodeDeviceShare(); err == nil {
		t.Fatal("expected error when npu-smi flip fails, got nil")
	}
}
