//go:build !linux || !cgo

package manager

import "fmt"

func newDcmiV2Client() (dcmiV2API, error) {
	return nil, fmt.Errorf("Ascend950DT DCMI v2 requires Linux with cgo enabled")
}
