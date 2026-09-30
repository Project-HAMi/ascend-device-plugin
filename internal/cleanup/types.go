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
	"time"

	"github.com/Project-HAMi/ascend-device-plugin/internal/manager"
)

const DefaultCheckpointPath = "/var/lib/kubelet/device-plugins/kubelet_internal_checkpoint"

// IdleVNPUReconciler is the only interface used by the device-plugin server
// for startup and periodic orphan cleanup.  Keeping one interface for both
// paths prevents kubelet restart handling from bypassing the safety checks.
type IdleVNPUReconciler interface {
	Reconcile(ctx context.Context) (ReconcileResult, error)
}

type ReconcilerFunc func(context.Context) (ReconcileResult, error)

func (f ReconcilerFunc) Reconcile(ctx context.Context) (ReconcileResult, error) {
	return f(ctx)
}

// PodAllocationSnapshot is deliberately separate from the hardware manager.
// It returns a complete node-local view or an incomplete/unknown result; an
// incomplete result must never be interpreted as an empty allocation.
type PodAllocationSnapshot interface {
	Snapshot(ctx context.Context) (AllocationSnapshot, error)
}

type DeviceResolver interface {
	GetDeviceByUUID(string) *manager.Device
}

type ClaimState string

const (
	ClaimCandidate ClaimState = "candidate"
	ClaimProtected ClaimState = "protected"
	ClaimUnknown   ClaimState = "unknown"
)

type ClaimEvidence struct {
	PodUID        string
	Namespace     string
	PodName       string
	ContainerName string
	State         ClaimState
	Reason        string
	Template      string
	DeviceUUID    string
	Source        string
	FailingSince  time.Time
}

type CardClaim struct {
	CardID   int32
	State    ClaimState
	PodUIDs  []string
	Evidence []ClaimEvidence
	// CandidateSince is the latest failure time among all candidate containers
	// mapped to this card. A card is reclaimable only after every candidate has
	// remained failed for the grace period, so the latest timestamp is the
	// conservative aggregate.
	CandidateSince time.Time
}

func (c CardClaim) Protected() bool {
	return c.State == ClaimProtected || c.State == ClaimUnknown
}

type AllocationSnapshot struct {
	NodeName       string
	Ready          bool
	ClaimsByCard   map[int32]CardClaim
	UnknownPodUIDs []string
	CollectedAt    time.Time
}

type ReconcileResult struct {
	CardsInspected     int
	CardsProtected     int
	CardsReclaimable   int
	VNPUsInspected     int
	VNPUsDestroyed     int
	VNPUsSkipped       int
	VNPUsIgnored       int
	UnknownSnapshot    bool
	DestroyErrors      int
	ConfirmationResets int
}

// HardwareView is the minimum platform-side hardware surface needed by the
// reconciler.  manager.Manager implements it while retaining its existing
// allocation and registration APIs.
type HardwareView interface {
	UpdateDevice() error
	GetDevices() []*manager.Device
	ListVirtualDevices(logicID int32) ([]manager.VirtualDevice, error)
	DestroyVirtualDevice(logicID int32, vdevID uint32) error
}

type Clock func() time.Time
