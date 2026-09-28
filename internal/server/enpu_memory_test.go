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
	"strings"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestENPUMemoryRequestAccounting(t *testing.T) {
	for _, request := range []string{"128", "512"} {
		t.Run(request, func(t *testing.T) {
			pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
				enpuMemoryRequestAnnotation: request,
				enpuMemoryLimitAnnotation:   "65536",
			}}}
			_, _, err := enpuMemoryRequestLimit(pod, 256, 3)
			if err == nil || !strings.Contains(err.Error(), "must equal HAMi allocated memory 256 MB") {
				t.Fatalf("request=%s must not differ from scheduler reservation: %v", request, err)
			}
		})
	}
}

func TestENPUMemorySwapLimitUsesAllocatedRequest(t *testing.T) {
	for _, explicitRequest := range []bool{false, true} {
		annotations := map[string]string{enpuMemoryLimitAnnotation: "65536"}
		if explicitRequest {
			annotations[enpuMemoryRequestAnnotation] = "256"
		}
		pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: annotations}}
		request, limit, err := enpuMemoryRequestLimit(pod, 256, 3)
		if err != nil || request != 256 || limit != 65536 {
			t.Fatalf("explicitRequest=%v: got request=%d limit=%d error=%v", explicitRequest, request, limit, err)
		}
		if _, _, err := enpuMemoryRequestLimit(pod, 256, 1); err == nil {
			t.Fatal("fixed-share must continue rejecting memory oversubscription")
		}
	}
}

func TestENPUMemoryLegacyQuotaUnchanged(t *testing.T) {
	for _, policy := range []int{1, 2, 3} {
		request, limit, err := enpuMemoryRequestLimit(&v1.Pod{}, 16384, policy)
		if err != nil || request != 16384 || limit != 16384 {
			t.Fatalf("policy=%d: legacy memory quota changed: request=%d limit=%d error=%v", policy, request, limit, err)
		}
	}
}
