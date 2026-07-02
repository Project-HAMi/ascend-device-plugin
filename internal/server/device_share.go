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
	"os/exec"
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
		klog.V(4).Infof("device-share card=%s chip=%s set to %s", card, chip, flag)
	}
	return nil
}

// enableNodeDeviceShare turns device-share on for every chip on the node when
// it runs in hami-vnpu-core soft-slice mode. Called once at startup and
// idempotent (npu-smi accepts redundant set commands). On a non-hami-vnpu-core
// node it is a no-op and never writes -d 0. Any per-chip failure aborts startup
// so kubelet restarts and retries.
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
