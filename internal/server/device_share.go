/*
 * Copyright 2024 The HAMi Authors.
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
	"os/exec"
	"strconv"
	"strings"

	"k8s.io/klog/v2"
)

// chipKey identifies a single NPU chip by the two coordinates npu-smi takes
// for its -i (card) and -c (chip) flags.
type chipKey struct {
	Card int32
	Chip int32
}

// npuSmiCandidates lists the host paths where npu-smi may live, in priority
// order. The first is what the daemonset's /usr/local/Ascend/driver hostPath
// mount exposes; the others cover hosts that ship npu-smi elsewhere. It is a
// package var so tests can point it at a temp dir.
var npuSmiCandidates = []string{
	"/usr/local/Ascend/driver/tools/npu-smi",
	"/usr/local/sbin/npu-smi",
	"/usr/local/bin/npu-smi",
}

// runNpuSmi executes npu-smi with the given args and returns combined output.
// It is a package var so tests can substitute a fake.
//
// Enabling device-share (-d 1) prints a "There are security risks ... continue
// setting?(Y/N)" prompt and aborts with exit 200 if stdin is closed. npu-smi
// has no -y/--force flag in the driver versions this plugin targets, so we
// feed "Y\n" unconditionally: commands that don't prompt simply ignore the
// unread stdin.
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

// applyDeviceShare flips device-share on every chip in chips. It does not query
// current state first — npu-smi accepts redundant set commands, so the
// unconditional write is cheaper than a query+set round trip and keeps Allocate
// latency predictable.
//
// Fails fast on the first per-chip error. The caller (Allocate) propagates that
// error so kubelet surfaces it on the Pod; partial state on the remaining chips
// will be re-driven by the next Allocate that lands on them.
//
// This plugin only ever calls it with enabled=true; the enabled=false path
// exists for symmetry and test coverage.
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
		klog.V(4).Infof("device-share card=%s chip=%s set to %s", card, chip, flag)
	}
	return nil
}

// enableNodeDeviceShare turns device-share on for every chip on the node when
// the node is configured for hami-vnpu-core soft slicing. It is called once at
// startup (from Start, after UpdateDevice has populated the device list) and is
// idempotent: npu-smi accepts redundant set commands, so a plugin restart
// simply re-applies the same state.
//
// When the node is not hami-vnpu-core it is a no-op — it never writes -d 0, so
// share state set for other purposes is left untouched.
//
// Fail-fast: any per-chip failure is returned to the caller, which aborts
// startup so kubelet restarts the plugin and retries.
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
	if err := applyDeviceShare(chips, true); err != nil {
		return fmt.Errorf("enable node device-share: %w", err)
	}
	klog.Infof("device-share enabled on %d chip(s) of node %s", len(chips), ps.nodeName)
	return nil
}
