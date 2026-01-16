/* Copyright(C) 2025. Huawei Technologies Co.,Ltd. All rights reserved.
   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

// Package hccn this for npu hccn info
package hccn

import (
	"fmt"
	"strings"
	"testing"
)

func TestBuildHccnErr(t *testing.T) {
	t.Run("normal error", func(t *testing.T) {
		phyID := int32(1)
		msg := "status"
		originalErr := fmt.Errorf("permission denied")

		err := buildHccnErr(phyID, msg, originalErr)

		if !strings.Contains(err.Error(), "phyID(1)") {
			t.Error("should contain phyID")
		}
		if !strings.Contains(err.Error(), "npu status") {
			t.Error("should contain npu message")
		}
		if !strings.Contains(err.Error(), "permission denied") {
			t.Error("should contain original error")
		}
	})

	t.Run("nil error", func(t *testing.T) {
		err := buildHccnErr(0, "", nil)
		if !strings.Contains(err.Error(), "error is :nil") {
			t.Error("should handle nil error")
		}
	})
}
