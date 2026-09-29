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

package cleanup

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Project-HAMi/ascend-device-plugin/internal/manager"
	"k8s.io/klog/v2"
)

const (
	DefaultGracePeriod   = 10 * time.Minute
	DefaultConfirmations = 3
)

type ReconcilerOptions struct {
	GracePeriod   time.Duration
	Confirmations int
	Clock         Clock
}

type Reconciler struct {
	hardware      HardwareView
	snapshotter   PodAllocationSnapshot
	gracePeriod   time.Duration
	confirmations int
	clock         Clock

	mu                  sync.Mutex
	confirmationsByCard *confirmationTracker
}

func NewReconciler(hardware HardwareView, snapshotter PodAllocationSnapshot, options ReconcilerOptions) *Reconciler {
	if options.GracePeriod <= 0 {
		options.GracePeriod = DefaultGracePeriod
	}
	if options.Confirmations <= 0 {
		options.Confirmations = DefaultConfirmations
	}
	if options.Clock == nil {
		options.Clock = time.Now
	}
	return &Reconciler{
		hardware:            hardware,
		snapshotter:         snapshotter,
		gracePeriod:         options.GracePeriod,
		confirmations:       options.Confirmations,
		clock:               options.Clock,
		confirmationsByCard: newConfirmationTracker(),
	}
}

func (r *Reconciler) Reconcile(ctx context.Context) (ReconcileResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var result ReconcileResult
	if r.hardware == nil || r.snapshotter == nil {
		r.resetAll(&result)
		result.UnknownSnapshot = true
		return result, fmt.Errorf("idle vNPU reconciler is not fully configured")
	}
	if err := r.hardware.UpdateDevice(); err != nil {
		r.resetAll(&result)
		result.UnknownSnapshot = true
		return result, fmt.Errorf("refresh Ascend device view: %w", err)
	}

	snapshot, snapshotErr := r.snapshotter.Snapshot(ctx)
	if snapshotErr != nil || !snapshot.Ready || len(snapshot.UnknownPodUIDs) > 0 {
		r.resetAll(&result)
		result.UnknownSnapshot = true
		if snapshotErr == nil {
			snapshotErr = fmt.Errorf("allocation snapshot is not ready")
		}
		return result, snapshotErr
	}

	now := r.clock()
	var destroyErrors []error
	devices := r.hardware.GetDevices()
	for _, device := range devices {
		if device == nil {
			continue
		}
		result.CardsInspected++
		claim := snapshot.ClaimsByCard[device.CardID]
		if claim.Protected() {
			if r.confirmationsByCard.reset(device.CardID) {
				result.ConfirmationResets++
			}
			result.CardsProtected++
			continue
		}

		virtualDevices, err := r.hardware.ListVirtualDevices(device.LogicID)
		if err != nil {
			if r.confirmationsByCard.reset(device.CardID) {
				result.ConfirmationResets++
			}
			result.UnknownSnapshot = true
			destroyErrors = append(destroyErrors, fmt.Errorf("list virtual devices for card %d: %w", device.CardID, err))
			continue
		}

		idle := make([]manager.VirtualDevice, 0, len(virtualDevices))
		for _, virtualDevice := range virtualDevices {
			result.VNPUsInspected++
			if virtualDevice.Ignored {
				result.VNPUsIgnored++
				result.VNPUsSkipped++
				continue
			}
			if virtualDevice.IsContainerUsed != 0 {
				result.VNPUsSkipped++
				continue
			}
			idle = append(idle, virtualDevice)
		}
		if len(idle) == 0 {
			if r.confirmationsByCard.reset(device.CardID) {
				result.ConfirmationResets++
			}
			continue
		}

		if !r.confirmationsByCard.observe(device.CardID, now, claim.CandidateSince, r.gracePeriod, r.confirmations) {
			continue
		}
		result.CardsReclaimable++
		for _, virtualDevice := range idle {
			if err := r.hardware.DestroyVirtualDevice(device.LogicID, uint32(virtualDevice.VDevID)); err != nil {
				result.DestroyErrors++
				destroyErrors = append(destroyErrors, fmt.Errorf("destroy idle vNPU card=%d logicID=%d vdevID=%d: %w", device.CardID, device.LogicID, virtualDevice.VDevID, err))
				klog.Errorf("failed to destroy idle vNPU card=%d logicID=%d vdevID=%d: %v", device.CardID, device.LogicID, virtualDevice.VDevID, err)
				continue
			}
			result.VNPUsDestroyed++
			klog.Infof("destroyed orphan idle vNPU card=%d logicID=%d vdevID=%d template=%s", device.CardID, device.LogicID, virtualDevice.VDevID, virtualDevice.TemplateName)
		}
	}

	if len(destroyErrors) > 0 {
		return result, errors.Join(destroyErrors...)
	}
	return result, nil
}

// Reset clears confirmation state before a kubelet/plugin restart. A restart
// is a lifecycle boundary, so observations collected before it must not be
// reused to immediately destroy an idle vNPU.
func (r *Reconciler) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.confirmationsByCard.resetAll()
}

func (r *Reconciler) resetAll(result *ReconcileResult) {
	result.ConfirmationResets += r.confirmationsByCard.resetAll()
}

var _ IdleVNPUReconciler = (*Reconciler)(nil)
