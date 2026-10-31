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
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

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

// npuSmiTimeout bounds a single npu-smi invocation. runNpuSmi is called from
// the watchAndRegister loop while chips wait for device-share, so a hung
// npu-smi must not stall that loop, and with it the kubelet device updates and
// the HAMi node registration. A variable so tests can shorten it.
var npuSmiTimeout = 30 * time.Second

// multiDiePolicyTimeout bounds the multi-die-policy probe, which runs before
// the plugin serves anything.
var multiDiePolicyTimeout = 10 * time.Second

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
	timeout := npuSmiTimeout
	if len(args) == 3 && args[0] == "info" && args[1] == "-t" && args[2] == "multi-die-policy" {
		// The multi-die-policy probe gates startup, so give it a shorter leash
		// than the set commands the retry loop drives.
		timeout = multiDiePolicyTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdin = strings.NewReader("Y\n")
	// Do not wait forever on the output pipe if a killed npu-smi left a child
	// holding it open.
	cmd.WaitDelay = time.Second
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return out, fmt.Errorf("npu-smi %s: timed out after %s", strings.Join(args, " "), timeout)
	}
	return out, err
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

// shareTarget is a chip whose device-share still has to be enabled: the
// npu-smi coordinates to set it plus the logic ID under which the driver
// reports the processes running on it.
type shareTarget struct {
	chipKey
	LogicID int32
}

// applyDeviceShare sets device-share on every chip unconditionally; npu-smi
// accepts redundant set commands, so this is cheaper than a query+set round
// trip. Fails fast on the first per-chip error, leaving later chips to be
// re-driven by the next Allocate. The enabled=false path exists only for tests.
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
			return fmt.Errorf("npu-smi set device-share -i %s -c %s -d %s: %w: %s",
				card, chip, flag, err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}

// enableNodeDeviceShare enables device sharing for hami-core or ENPU: it turns
// device-share on for every chip on the node when it runs with a runtime
// soft-slice backend. Called once at startup and idempotent (npu-smi accepts
// redundant set commands). On a node without such a backend it is a no-op and
// never writes -d 0.
//
// The driver refuses to switch a chip that still runs a workload, and that
// condition clears on its own once the workload finishes. Chips the driver
// reports as in use (Manager.IsDeviceInUse, the DCMI process list) are
// therefore skipped and queued in ps.pendingDeviceShare for
// retryPendingDeviceShare, so that busy chips do not take every device on the
// node offline, even when every chip is busy. Any other npu-smi failure
// (binary missing, set refused on an idle chip) still aborts startup so
// kubelet restarts and retries, as before.
func (ps *PluginServer) enableNodeDeviceShare() error {
	if !ps.mgr.IsHamiVnpuCore() && !managerUsesENPU(ps.mgr) {
		klog.V(3).Infof("node %s has no runtime soft-slice backend, skipping device-share", ps.nodeName)
		return nil
	}
	seen := map[chipKey]bool{}
	var chips []shareTarget
	for _, d := range ps.mgr.GetDevices() {
		key := chipKey{Card: d.CardID, Chip: d.DeviceID}
		if seen[key] {
			continue
		}
		seen[key] = true
		chips = append(chips, shareTarget{chipKey: key, LogicID: d.LogicID})
	}
	if len(chips) == 0 {
		klog.Warningf("node %s is hami-vnpu-core but no devices found for device-share", ps.nodeName)
		return nil
	}
	var pending []shareTarget
	var errs []error
	for _, c := range chips {
		busy, err := ps.tryEnableDeviceShare(c)
		switch {
		case err != nil:
			errs = append(errs, err)
		case busy:
			klog.Warningf("device-share not enabled on card %d chip %d of node %s yet: a workload still runs on it, will retry once it is free",
				c.Card, c.Chip, ps.nodeName)
			pending = append(pending, c)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("enable node device-share: %w", errors.Join(errs...))
	}
	ps.setPendingDeviceShare(pending)
	klog.Infof("device-share enabled on %d of %d chip(s) of node %s", len(chips)-len(pending), len(chips), ps.nodeName)
	return nil
}

// tryEnableDeviceShare enables device-share on one chip unless the driver
// reports a workload on it. busy=true means the chip has to be retried later;
// a non-nil error means npu-smi failed for some other reason.
//
// The process list is read again after a refused set: a workload can start
// between the check and the set, and the refusal then still only means "chip
// in use", not a reason to abort startup. If the process list itself cannot
// be read the set is attempted anyway and its result decides.
func (ps *PluginServer) tryEnableDeviceShare(c shareTarget) (busy bool, err error) {
	inUse, err := ps.mgr.IsDeviceInUse(c.LogicID)
	if err != nil {
		klog.Warningf("cannot tell whether card %d chip %d is in use, setting device-share anyway: %v", c.Card, c.Chip, err)
	} else if inUse {
		return true, nil
	}
	setErr := applyDeviceShare([]chipKey{c.chipKey}, true)
	if setErr == nil {
		return false, nil
	}
	if inUse, err := ps.mgr.IsDeviceInUse(c.LogicID); err == nil && inUse {
		return true, nil
	}
	return false, setErr
}

// retryPendingDeviceShare re-drives the chips that were in use at startup and
// drops the ones that switch. Called from watchAndRegister on every tick, so a
// chip that is still in use is logged at V(3) only: it stays pending for as
// long as the workload occupying it runs.
func (ps *PluginServer) retryPendingDeviceShare() {
	queued := ps.snapshotPendingDeviceShare()
	if len(queued) == 0 {
		return
	}
	var pending []shareTarget
	for _, c := range queued {
		busy, err := ps.tryEnableDeviceShare(c)
		switch {
		case err != nil:
			klog.Warningf("device-share still not enabled on card %d chip %d, will retry: %v", c.Card, c.Chip, err)
			pending = append(pending, c)
		case busy:
			klog.V(3).Infof("card %d chip %d still in use, device-share will be retried", c.Card, c.Chip)
			pending = append(pending, c)
		}
	}
	if switched := len(queued) - len(pending); switched > 0 {
		klog.Infof("device-share enabled on %d more chip(s) of node %s, %d still pending", switched, ps.nodeName, len(pending))
	}
	ps.setPendingDeviceShare(pending)
}

// setPendingDeviceShare replaces the queue of chips waiting for device-share.
func (ps *PluginServer) setPendingDeviceShare(pending []shareTarget) {
	ps.pendingMu.Lock()
	defer ps.pendingMu.Unlock()
	ps.pendingDeviceShare = pending
}

// snapshotPendingDeviceShare copies the queue so the retry loop can drive
// npu-smi without holding the lock.
func (ps *PluginServer) snapshotPendingDeviceShare() []shareTarget {
	ps.pendingMu.RLock()
	defer ps.pendingMu.RUnlock()
	if len(ps.pendingDeviceShare) == 0 {
		return nil
	}
	return append([]shareTarget(nil), ps.pendingDeviceShare...)
}

// pendingShareChips is the set of chips that are not registered yet. A chip
// whose device-share the driver refused is still running a workload under the
// previous whole-chip layout, so advertising it would let the scheduler place
// sliced workloads on a chip that cannot slice. It is withheld from both
// kubelet and the HAMi node annotation until the switch succeeds.
func (ps *PluginServer) pendingShareChips() map[chipKey]bool {
	ps.pendingMu.RLock()
	defer ps.pendingMu.RUnlock()
	if len(ps.pendingDeviceShare) == 0 {
		return nil
	}
	chips := make(map[chipKey]bool, len(ps.pendingDeviceShare))
	for _, c := range ps.pendingDeviceShare {
		chips[c.chipKey] = true
	}
	return chips
}
