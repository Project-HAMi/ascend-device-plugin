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
	"cmp"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	"k8s.io/klog/v2"
)

// chipKey identifies an NPU chip by npu-smi's -i (card) and -c (chip) coordinates.
type chipKey struct {
	Card int32
	Chip int32
}

// npuSmiCandidates lists host paths where npu-smi may live, in priority order.
// A package var so tests can point it at a temp dir.
var npuSmiCandidates = []string{
	"/usr/local/Ascend/driver/tools/npu-smi",
	"/usr/local/sbin/npu-smi",
	"/usr/local/bin/npu-smi",
}

// runNpuSmi runs npu-smi and returns combined output. A package var so tests
// can substitute a fake.
//
// Enabling device-share (-d 1) prompts "continue setting?(Y/N)" and exits 200
// if stdin is closed; npu-smi has no -y flag, so we feed "Y\n" unconditionally
// (commands that don't prompt ignore the unread stdin).
var runNpuSmi = func(args ...string) ([]byte, error) {
	bin, err := resolveNpuSmi()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(bin, args...)
	cmd.Stdin = strings.NewReader("Y\n")
	return cmd.CombinedOutput()
}

func resolveNpuSmi() (string, error) {
	for _, p := range npuSmiCandidates {
		st, err := os.Stat(p)
		if err == nil && !st.IsDir() && st.Mode()&0111 != 0 {
			return p, nil
		}
	}
	if p, err := exec.LookPath("npu-smi"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("npu-smi not found in %v or PATH", npuSmiCandidates)
}

// errChipBusy marks a device-share refusal from the driver because the chip
// still runs a workload. npu-smi exits 203 with "Failed to set chip
// device-share" in that case; the condition clears once the workload finishes.
var errChipBusy = errors.New("chip is in use")

const npuSmiChipBusyExitCode = 203

// isChipBusy reports whether a failed npu-smi set device-share call was the
// driver refusing to switch a chip that is in use, as opposed to npu-smi being
// missing or failing for any other reason.
func isChipBusy(err error, out []byte) bool {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == npuSmiChipBusyExitCode {
		return true
	}
	return strings.Contains(string(out), "Failed to set chip device-share")
}

// applyDeviceShare sets device-share on every chip unconditionally; npu-smi
// accepts redundant set commands, so this is cheaper than a query+set round
// trip. Fails fast on the first per-chip error, wrapping errChipBusy when the
// driver refused because the chip is in use; enableNodeDeviceShare drives it
// one chip at a time so that a busy chip does not block the others. The
// enabled=false path exists only for tests.
func applyDeviceShare(chips []chipKey, enabled bool) error {
	if len(chips) == 0 {
		return nil
	}
	flag := "0"
	if enabled {
		flag = "1"
	}
	for _, c := range chips {
		card := strconv.Itoa(int(c.Card))
		chip := strconv.Itoa(int(c.Chip))
		out, err := runNpuSmi("set", "-t", "device-share", "-i", card, "-c", chip, "-d", flag)
		if err != nil {
			if isChipBusy(err, out) {
				err = fmt.Errorf("%w: %w", errChipBusy, err)
			}
			return fmt.Errorf("npu-smi set device-share -i %s -c %s -d %s: %w: %s",
				card, chip, flag, err, strings.TrimSpace(string(out)))
		}
		klog.V(4).Infof("device-share card=%s chip=%s set to %s", card, chip, flag)
	}
	return nil
}

// enableNodeDeviceShare turns device-share on for every chip on the node when
// it runs in hami-vnpu-core soft-slice mode. Called once at startup and
// idempotent (npu-smi accepts redundant set commands). On a non-hami-vnpu-core
// node it is a no-op and never writes -d 0.
//
// The driver refuses to switch a chip that still runs a workload (exit 203),
// and that condition clears on its own once the workload finishes. Such chips
// are therefore skipped and queued in ps.pendingDeviceShare for
// retryPendingDeviceShare, so that busy chips do not take every device on the
// node offline, even when every chip is busy. Any other npu-smi failure
// (binary missing, other exit codes) still aborts startup so kubelet restarts
// and retries, as before.
func (ps *PluginServer) enableNodeDeviceShare() error {
	if !ps.mgr.IsHamiVnpuCore() {
		klog.V(3).Infof("node %s is not hami-vnpu-core, skipping device-share", ps.nodeName)
		return nil
	}
	chipSet := map[chipKey]struct{}{}
	for _, d := range ps.mgr.GetDevices() {
		chipSet[chipKey{Card: d.CardID, Chip: d.DeviceID}] = struct{}{}
	}
	if len(chipSet) == 0 {
		klog.Warningf("node %s is hami-vnpu-core but no devices found for device-share", ps.nodeName)
		return nil
	}
	chips := make([]chipKey, 0, len(chipSet))
	for c := range chipSet {
		chips = append(chips, c)
	}
	slices.SortFunc(chips, func(a, b chipKey) int {
		return cmp.Or(cmp.Compare(a.Card, b.Card), cmp.Compare(a.Chip, b.Chip))
	})
	pending, errs := applyDeviceSharePerChip(chips)
	var fatal []error
	for _, err := range errs {
		if !errors.Is(err, errChipBusy) {
			fatal = append(fatal, err)
		}
	}
	if len(fatal) > 0 {
		return fmt.Errorf("enable node device-share: %w", errors.Join(fatal...))
	}
	for _, err := range errs {
		klog.Warningf("device-share not enabled yet, will retry once the chip is free: %v", err)
	}
	ps.pendingDeviceShare = pending
	klog.Infof("device-share enabled on %d of %d chip(s) of node %s", len(chips)-len(pending), len(chips), ps.nodeName)
	return nil
}

// retryPendingDeviceShare re-drives the chips that could not be switched at
// startup and drops the ones that succeed. Called from watchAndRegister on
// every tick, so failures are logged at V(3) only: a chip stays pending for as
// long as the workload occupying it runs.
func (ps *PluginServer) retryPendingDeviceShare() {
	if len(ps.pendingDeviceShare) == 0 {
		return
	}
	pending, errs := applyDeviceSharePerChip(ps.pendingDeviceShare)
	for _, err := range errs {
		klog.V(3).Infof("device-share still not enabled, will retry: %v", err)
	}
	if switched := len(ps.pendingDeviceShare) - len(pending); switched > 0 {
		klog.Infof("device-share enabled on %d more chip(s) of node %s, %d still pending", switched, ps.nodeName, len(pending))
	}
	ps.pendingDeviceShare = pending
}

// applyDeviceSharePerChip enables device-share on each chip independently and
// returns the chips that failed together with their errors.
func applyDeviceSharePerChip(chips []chipKey) ([]chipKey, []error) {
	var failed []chipKey
	var errs []error
	for _, c := range chips {
		if err := applyDeviceShare([]chipKey{c}, true); err != nil {
			failed = append(failed, c)
			errs = append(errs, err)
		}
	}
	return failed, errs
}
