/*
 * Copyright 2026 The HAMi Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 */

package cleanup

import "time"

type confirmationState struct {
	firstSeen      time.Time
	candidateSince time.Time
	count          int
}

type confirmationTracker struct {
	states map[int32]confirmationState
}

func newConfirmationTracker() *confirmationTracker {
	return &confirmationTracker{states: make(map[int32]confirmationState)}
}

func (t *confirmationTracker) reset(cardID int32) bool {
	if _, exists := t.states[cardID]; !exists {
		return false
	}
	delete(t.states, cardID)
	return true
}

func (t *confirmationTracker) resetAll() int {
	count := len(t.states)
	t.states = make(map[int32]confirmationState)
	return count
}

func (t *confirmationTracker) observe(cardID int32, now, candidateSince time.Time, grace time.Duration, confirmations int) bool {
	state, exists := t.states[cardID]
	if !exists {
		state = confirmationState{firstSeen: now}
	}
	if !candidateSince.IsZero() && (state.candidateSince.IsZero() || candidateSince.After(state.candidateSince)) {
		state.candidateSince = candidateSince
	}
	state.count++
	t.states[cardID] = state
	if confirmations < 1 {
		confirmations = 1
	}
	// Never let a stale candidateSince (for example, restored from an older
	// snapshot after restart) shorten the current process's grace period.
	// firstSeen is the lower safety bound and is initialized only once.
	graceSince := state.firstSeen
	if state.candidateSince.After(graceSince) {
		graceSince = state.candidateSince
	}
	return state.count >= confirmations && now.Sub(graceSince) >= grace
}
