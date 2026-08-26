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
	"testing"
	"time"

	"google.golang.org/grpc"
	"k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

type fakeListAndWatchServer struct {
	grpc.ServerStream
	ctx      context.Context
	sendFunc func(*v1beta1.ListAndWatchResponse) error
}

func (s *fakeListAndWatchServer) Context() context.Context {
	return s.ctx
}

func (s *fakeListAndWatchServer) Send(resp *v1beta1.ListAndWatchResponse) error {
	return s.sendFunc(resp)
}

func TestListAndWatchReturnsInitialSendError(t *testing.T) {
	wantErr := errors.New("stream closed")
	stopCh := make(chan any)
	close(stopCh)
	ps := &PluginServer{
		mgr:    &FakeManager{},
		stopCh: stopCh,
	}
	stream := &fakeListAndWatchServer{
		ctx: context.Background(),
		sendFunc: func(*v1beta1.ListAndWatchResponse) error {
			return wantErr
		},
	}

	err := ps.ListAndWatch(&v1beta1.Empty{}, stream)
	if !errors.Is(err, wantErr) {
		t.Fatalf("ListAndWatch() error = %v, want %v", err, wantErr)
	}
}

func TestListAndWatchReturnsHealthUpdateSendError(t *testing.T) {
	wantErr := errors.New("stream closed")
	stopCh := make(chan any)
	healthCh := make(chan int32, 1)
	healthCh <- 0
	ps := &PluginServer{
		mgr:      &FakeManager{},
		stopCh:   stopCh,
		healthCh: healthCh,
	}
	sendCount := 0
	stream := &fakeListAndWatchServer{
		ctx: context.Background(),
		sendFunc: func(*v1beta1.ListAndWatchResponse) error {
			sendCount++
			if sendCount == 2 {
				close(stopCh)
				return wantErr
			}
			return nil
		},
	}

	err := ps.ListAndWatch(&v1beta1.Empty{}, stream)
	if !errors.Is(err, wantErr) {
		t.Fatalf("ListAndWatch() error = %v, want %v", err, wantErr)
	}
}

func TestListAndWatchReturnsWhenStreamIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	sendCount := 0
	ps := &PluginServer{
		mgr:      &FakeManager{},
		stopCh:   make(chan any),
		healthCh: make(chan int32),
	}
	stream := &fakeListAndWatchServer{
		ctx: ctx,
		sendFunc: func(*v1beta1.ListAndWatchResponse) error {
			sendCount++
			return nil
		},
	}

	err := ps.ListAndWatch(&v1beta1.Empty{}, stream)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ListAndWatch() error = %v, want %v", err, context.Canceled)
	}
	if sendCount != 0 {
		t.Fatalf("Send() calls = %d, want 0", sendCount)
	}
}

func TestListAndWatchReturnsWhenStreamIsCanceledWhileWaiting(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ps := &PluginServer{
		mgr:      &FakeManager{},
		stopCh:   make(chan any),
		healthCh: make(chan int32),
	}
	initialSent := make(chan struct{}, 1)
	sendCount := 0
	stream := &fakeListAndWatchServer{
		ctx: ctx,
		sendFunc: func(*v1beta1.ListAndWatchResponse) error {
			sendCount++
			initialSent <- struct{}{}
			return nil
		},
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- ps.ListAndWatch(&v1beta1.Empty{}, stream)
	}()

	select {
	case <-initialSent:
	case <-time.After(time.Second):
		t.Fatal("initial Send() was not called")
	}
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("ListAndWatch() error = %v, want %v", err, context.Canceled)
		}
	case <-time.After(time.Second):
		t.Fatal("ListAndWatch() did not return after stream cancellation")
	}
	if sendCount != 1 {
		t.Fatalf("Send() calls = %d, want 1", sendCount)
	}
}
