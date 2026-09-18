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
	"errors"
	"fmt"
	"os"
	"os/exec"
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

// busyChipNpuSmi returns a fake npu-smi that mimics the driver refusing to
// switch device-share on a chip that is in use (exit status 203), for the
// given card/chip only, and records every invocation.
func busyChipNpuSmi(calls *[][]string, busyCard, busyChip string) func(args ...string) ([]byte, error) {
	return func(args ...string) ([]byte, error) {
		*calls = append(*calls, append([]string(nil), args...))
		if len(args) >= 7 && args[4] == busyCard && args[6] == busyChip {
			return []byte("    Status                         : Fail\n" +
				"    Message                        : Failed to set chip device-share"), fmt.Errorf("exit status 203")
		}
		return nil, nil
	}
}

func hamiVnpuCoreServer(devs ...*manager.Device) *PluginServer {
	return &PluginServer{
		nodeName: "node-1",
		mgr: &FakeManager{
			IsHamiVnpuCoreFunc: func() bool { return true },
			GetDevicesFunc:     func() []*manager.Device { return devs },
		},
	}
}

// TestEnableNodeDeviceShare_BusyChipIsSkipped is the scenario from issue #131:
// one chip still runs a workload, so npu-smi refuses to switch it. The plugin
// must keep going, switch the remaining chips, and remember the busy one for a
// later retry instead of aborting startup and dropping every device.
func TestEnableNodeDeviceShare_BusyChipIsSkipped(t *testing.T) {
	var calls [][]string
	withFakeNpuSmi(t, busyChipNpuSmi(&calls, "0", "1"))
	ps := hamiVnpuCoreServer(
		&manager.Device{CardID: 0, DeviceID: 0},
		&manager.Device{CardID: 0, DeviceID: 1},
		&manager.Device{CardID: 1, DeviceID: 0},
	)

	if err := ps.enableNodeDeviceShare(); err != nil {
		t.Fatalf("a single busy chip must not fail startup, got: %v", err)
	}
	want := [][]string{
		{"set", "-t", "device-share", "-i", "0", "-c", "0", "-d", "1"},
		{"set", "-t", "device-share", "-i", "0", "-c", "1", "-d", "1"},
		{"set", "-t", "device-share", "-i", "1", "-c", "0", "-d", "1"},
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("every chip must be attempted, in card/chip order:\ngot  %v\nwant %v", calls, want)
	}
	if wantPending := []chipKey{{Card: 0, Chip: 1}}; !reflect.DeepEqual(ps.pendingDeviceShare, wantPending) {
		t.Fatalf("busy chip must be queued for retry:\ngot  %v\nwant %v", ps.pendingDeviceShare, wantPending)
	}
}

// TestEnableNodeDeviceShare_AllChipsBusyAreQueued covers a fully loaded node
// (or a single-chip node whose chip is busy): every chip is refused with exit
// 203. The plugin must still come up and queue all of them instead of
// crash-looping and dropping every device.
func TestEnableNodeDeviceShare_AllChipsBusyAreQueued(t *testing.T) {
	var calls [][]string
	withFakeNpuSmi(t, func(args ...string) ([]byte, error) {
		calls = append(calls, append([]string(nil), args...))
		return []byte("Failed to set chip device-share"), fmt.Errorf("exit status 203")
	})
	ps := hamiVnpuCoreServer(
		&manager.Device{CardID: 0, DeviceID: 0},
		&manager.Device{CardID: 1, DeviceID: 0},
	)

	if err := ps.enableNodeDeviceShare(); err != nil {
		t.Fatalf("busy chips must not fail startup even when all are busy, got: %v", err)
	}
	if got := len(calls); got != 2 {
		t.Fatalf("every chip must be attempted, got %d calls: %v", got, calls)
	}
	if want := []chipKey{{Card: 0, Chip: 0}, {Card: 1, Chip: 0}}; !reflect.DeepEqual(ps.pendingDeviceShare, want) {
		t.Fatalf("every busy chip must be queued for retry:\ngot  %v\nwant %v", ps.pendingDeviceShare, want)
	}
}

// TestEnableNodeDeviceShare_OtherFailureStillFailsFast makes sure only the
// driver's busy refusal is tolerated: any other npu-smi failure on a chip
// (here exit status 1) aborts startup as before, even when the other chips
// switched fine, and nothing is queued.
func TestEnableNodeDeviceShare_OtherFailureStillFailsFast(t *testing.T) {
	var calls [][]string
	withFakeNpuSmi(t, func(args ...string) ([]byte, error) {
		calls = append(calls, append([]string(nil), args...))
		if len(args) >= 7 && args[4] == "1" && args[6] == "0" {
			return []byte("E80001 not allowed"), fmt.Errorf("exit status 1")
		}
		return nil, nil
	})
	ps := hamiVnpuCoreServer(
		&manager.Device{CardID: 0, DeviceID: 0},
		&manager.Device{CardID: 1, DeviceID: 0},
	)

	err := ps.enableNodeDeviceShare()
	if err == nil {
		t.Fatal("expected error when npu-smi fails for a reason other than a busy chip, got nil")
	}
	if !strings.Contains(err.Error(), "-i 1 -c 0") {
		t.Fatalf("error should name the failing chip, got: %v", err)
	}
	if errors.Is(err, errChipBusy) {
		t.Fatalf("exit status 1 must not be classified as a busy chip: %v", err)
	}
	if got := len(calls); got != 2 {
		t.Fatalf("every chip must still be attempted, got %d calls: %v", got, calls)
	}
	if len(ps.pendingDeviceShare) != 0 {
		t.Fatalf("nothing should be queued when startup fails, got %v", ps.pendingDeviceShare)
	}
}

// realExitError runs a shell that exits with code so the test gets the
// *exec.ExitError the real runNpuSmi returns.
func realExitError(t *testing.T, code int) error {
	t.Helper()
	err := exec.Command("sh", "-c", fmt.Sprintf("exit %d", code)).Run()
	if err == nil {
		t.Fatalf("expected exit %d to fail", code)
	}
	return err
}

func TestIsChipBusy(t *testing.T) {
	cases := []struct {
		name string
		err  error
		out  string
		want bool
	}{
		{"real exit 203 without output", realExitError(t, 203), "", true},
		{"real exit 203 with driver message", realExitError(t, 203), "Status : Fail\nMessage : Failed to set chip device-share", true},
		{"driver message with plain error", fmt.Errorf("exit status 203"), "Failed to set chip device-share", true},
		{"real exit 1 not allowed", realExitError(t, 1), "E80001 not allowed", false},
		{"plain exit 1", fmt.Errorf("exit status 1"), "E80001 not allowed", false},
		{"npu-smi missing", fmt.Errorf("npu-smi not found"), "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isChipBusy(tc.err, []byte(tc.out)); got != tc.want {
				t.Fatalf("isChipBusy(%v, %q) = %v, want %v", tc.err, tc.out, got, tc.want)
			}
		})
	}
}

func TestRetryPendingDeviceShare_DropsChipsOnceSwitched(t *testing.T) {
	var calls [][]string
	withFakeNpuSmi(t, busyChipNpuSmi(&calls, "1", "0"))
	ps := hamiVnpuCoreServer()
	ps.pendingDeviceShare = []chipKey{{Card: 0, Chip: 1}, {Card: 1, Chip: 0}}

	// First tick: card 0 chip 1 is free again, card 1 chip 0 is still busy.
	ps.retryPendingDeviceShare()
	if want := []chipKey{{Card: 1, Chip: 0}}; !reflect.DeepEqual(ps.pendingDeviceShare, want) {
		t.Fatalf("after first retry:\ngot  %v\nwant %v", ps.pendingDeviceShare, want)
	}
	want := [][]string{
		{"set", "-t", "device-share", "-i", "0", "-c", "1", "-d", "1"},
		{"set", "-t", "device-share", "-i", "1", "-c", "0", "-d", "1"},
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("first retry calls:\ngot  %v\nwant %v", calls, want)
	}

	// Second tick: the workload finished, the last chip switches.
	calls = nil
	withFakeNpuSmi(t, busyChipNpuSmi(&calls, "", ""))
	ps.retryPendingDeviceShare()
	if len(ps.pendingDeviceShare) != 0 {
		t.Fatalf("expected no pending chips, got %v", ps.pendingDeviceShare)
	}
	if want := [][]string{{"set", "-t", "device-share", "-i", "1", "-c", "0", "-d", "1"}}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("second retry calls:\ngot  %v\nwant %v", calls, want)
	}

	// Steady state: nothing pending, npu-smi is not invoked.
	calls = nil
	ps.retryPendingDeviceShare()
	if len(calls) != 0 {
		t.Fatalf("npu-smi must not run when nothing is pending, got %v", calls)
	}
}

// TestStart_ProceedsWhenOneChipIsBusy drives the full Start() path: with one
// busy chip the plugin must still serve, register and keep advertising devices.
func TestStart_ProceedsWhenOneChipIsBusy(t *testing.T) {
	var calls [][]string
	withFakeNpuSmi(t, busyChipNpuSmi(&calls, "0", "1"))
	ps := setupRestartablePluginServer(t)
	ps.mgr = &FakeManager{
		ResourceNameFunc:   func() string { return "test-ascend" },
		IsHamiVnpuCoreFunc: func() bool { return true },
		GetDevicesFunc: func() []*manager.Device {
			return []*manager.Device{{CardID: 0, DeviceID: 0}, {CardID: 0, DeviceID: 1}}
		},
	}

	if err := ps.Start(); err != nil {
		t.Fatalf("Start() must not fail because one chip is busy: %v", err)
	}
	defer func() {
		if err := ps.Stop(); err != nil {
			t.Fatalf("Stop() failed: %v", err)
		}
	}()
	if got := len(calls); got != 2 {
		t.Fatalf("expected both chips to be attempted at startup, got %d calls: %v", got, calls)
	}
}

// TestWatchAndRegister_RetriesPendingDeviceShare checks that the periodic
// loop re-drives chips that were busy at startup.
func TestWatchAndRegister_RetriesPendingDeviceShare(t *testing.T) {
	var calls [][]string
	withFakeNpuSmi(t, busyChipNpuSmi(&calls, "", ""))
	mgr := newCachingFakeManager(fakeDevices(2, true)...)
	mgr.IsHamiVnpuCoreFunc = func() bool { return true }
	ps := newWatchRegisterServer(t, mgr)
	ps.pendingDeviceShare = []chipKey{{Card: 1, Chip: 0}}

	runWatchAndRegister(t, ps, nil, watchRegisterRunFor)

	if len(ps.pendingDeviceShare) != 0 {
		t.Fatalf("pending chip should have been switched by the loop, still pending: %v", ps.pendingDeviceShare)
	}
	if want := [][]string{{"set", "-t", "device-share", "-i", "1", "-c", "0", "-d", "1"}}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("retry calls:\ngot  %v\nwant %v", calls, want)
	}
}
