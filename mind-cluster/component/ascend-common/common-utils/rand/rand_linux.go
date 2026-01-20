/* Copyright(C) 2023. Huawei Technologies Co.,Ltd. All rights reserved.
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

// Package rand implement the security rand
package rand

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"time"
)

const (
	maxReadSize = 1<<25 - 1
)

// A randomReader satisfies reads by reading the file named name.
type randomReader struct {
	f  io.Reader
	mu sync.Mutex
}

func init() {
	Reader = &randomReader{}
}

func warnBlocked() {
	fmt.Println("mindx-security/rand: blocked for 60 seconds waiting to read random data from the kernel")
}

var supportOs = "linux"

// Read implements the interface of io.Reader
func (r *randomReader) Read(b []byte) (int, error) {
	t := time.AfterFunc(time.Minute, warnBlocked)
	defer t.Stop()
	if len(b) > maxReadSize {
		return 0, errors.New("byte size is too large")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if runtime.GOOS != supportOs {
		return 0, errors.New("not supported")
	}
	f, err := os.Open("/dev/random")
	if err != nil {
		return 0, err
	}
	defer func() {
		err = f.Close()
		if err != nil {
			fmt.Println("close random file failed")
		}
	}()
	return f.Read(b)
}
