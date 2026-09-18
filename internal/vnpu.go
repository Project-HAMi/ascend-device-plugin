/*
Copyright 2024 The HAMi Authors.

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

package internal

import (
	"encoding/json"
	"errors"
	"math"
	"os"

	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/klog/v2"
)

type Template struct {
	Name   string `json:"name"`
	Memory int64  `json:"memory"`
	AICore int32  `json:"aiCore,omitempty"`
	AICPU  int32  `json:"aiCPU,omitempty"`
}

type VNPUConfig struct {
	CommonWord         string     `json:"commonWord"`
	ChipName           string     `json:"chipName"`
	ResourceName       string     `json:"resourceName"`
	ResourceMemoryName string     `json:"resourceMemoryName"`
	MemoryAllocatable  int64      `json:"memoryAllocatable"`
	MemoryCapacity     int64      `json:"memoryCapacity"`
	AICore             int32      `json:"aiCore"`
	AICPU              int32      `json:"aiCPU"`
	Templates          []Template `json:"templates"`
}

type VNPUsConfig struct {
	HamiVnpuCore bool `json:"hamiVnpuCore,omitempty"`
	// DeviceCoreScaling is the hami-core compute oversell ratio.
	// When hami-core is on, registerHAMi advertises Devcore = round(100 * DeviceCoreScaling).
	// Default 1 keeps a 100-point budget. Values below 1 are not supported and fall back to 1.
	DeviceCoreScaling float64      `json:"deviceCoreScaling,omitempty"`
	Configs           []VNPUConfig `json:"configs"`
}

type Config struct {
	VNPUs VNPUsConfig `json:"vnpus"`
}

// legacyConfig is the device-config.yaml layout of HAMi <= v2.8.x, where vnpus
// is a plain list; v2.9.0 moved that list under vnpus.configs.
type legacyConfig struct {
	HamiVnpuCore bool         `json:"hamiVnpuCore,omitempty"`
	VNPUs        []VNPUConfig `json:"vnpus"`
}

// FilterDevices defines devices that should be ignored by HAMi.
// A device is ignored when its UUID is listed in UUID or its index is listed in Index.
type FilterDevices struct {
	UUID  []string `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	Index []int32  `json:"index,omitempty" yaml:"index,omitempty"`
}

func (fd FilterDevices) IsEmpty() bool {
	return len(fd.UUID) == 0 && len(fd.Index) == 0
}

func (fd FilterDevices) HasUUID() bool {
	return len(fd.UUID) > 0
}

// isLegacyVNPUsLayout reports whether err is the "vnpus holds a list" mismatch
// a HAMi <= v2.8.x device config produces. Any other decoding failure, deeper
// errors inside vnpus included, is a genuine one and keeps its own message.
func isLegacyVNPUsLayout(err error) bool {
	var typeErr *json.UnmarshalTypeError
	return errors.As(err, &typeErr) && typeErr.Field == "vnpus" && typeErr.Value == "array"
}

// LoadConfig reads the hami-scheduler-device config, accepting both the
// HAMi >= v2.9.0 layout and the older one so that upgrading this plugin does
// not require upgrading HAMi at the same time.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var yamlData Config
	err = yaml.Unmarshal(data, &yamlData)
	if err == nil {
		return &yamlData, nil
	}
	if !isLegacyVNPUsLayout(err) {
		return nil, err
	}

	var legacy legacyConfig
	if err := yaml.Unmarshal(data, &legacy); err != nil {
		return nil, err
	}
	klog.Warningf("%s uses the device config layout of HAMi <= v2.8.x (vnpus holds a list); upgrade HAMi "+
		"to v2.9.0+ to get vnpus.hamiVnpuCore and hami-core soft slicing", path)
	return &Config{VNPUs: VNPUsConfig{HamiVnpuCore: legacy.HamiVnpuCore, Configs: legacy.VNPUs}}, nil
}

type NodeConfig struct {
	Name              string        `json:"name" yaml:"name"`
	HamiVnpuCore      bool          `json:"hami-vnpu-core" yaml:"hami-vnpu-core"`
	VDeviceCount      int           `json:"vDeviceCount" yaml:"vDeviceCount"`
	DeviceCoreScaling float64       `json:"deviceCoreScaling,omitempty" yaml:"deviceCoreScaling,omitempty"`
	FilterDevices     FilterDevices `json:"filterDevices,omitempty" yaml:"filterDevices,omitempty"`
}

type NodeListConfig struct {
	Nodes []NodeConfig `json:"nodes" yaml:"nodes"`
}

// hamiCorePercentBase is the fixed 0-100 percentage scale hami-core `-core`
// requests are expressed in.
const hamiCorePercentBase = 100

// AdvertisedDevcore is the core capacity registered to HAMi.
// Template/hard-slice mode keeps the hardware AICore. hami-core uses the
// percentage scale, optionally oversold by deviceCoreScaling.
//
// Scaling below 1 is rejected: HAMi cannot tell an under-provisioned percentage
// such as 50 apart from a hardware AICore count, so the reduced budget would be
// silently ignored on the scheduler side.
func AdvertisedDevcore(isHamiCore bool, scale float64, hardwareAICore int32) int32 {
	if !isHamiCore {
		return hardwareAICore
	}
	switch {
	case math.IsNaN(scale) || math.IsInf(scale, 0):
		klog.Warningf("deviceCoreScaling %v is not a finite number, advertising %d", scale, hamiCorePercentBase)
		return hamiCorePercentBase
	case scale == 0:
		return hamiCorePercentBase
	case scale < 1:
		klog.Warningf("deviceCoreScaling %v is below 1, hami-core cannot under-provision compute, advertising %d", scale, hamiCorePercentBase)
		return hamiCorePercentBase
	}
	budget := math.Round(hamiCorePercentBase * scale)
	if budget > math.MaxInt32 {
		klog.Warningf("deviceCoreScaling %v is out of range, advertising %d", scale, hamiCorePercentBase)
		return hamiCorePercentBase
	}
	return int32(budget)
}

func LoadNodeConfig(path string) (*NodeListConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var yamlData NodeListConfig
	err = yaml.Unmarshal(data, &yamlData)
	if err != nil {
		return nil, err
	}
	return &yamlData, nil
}
