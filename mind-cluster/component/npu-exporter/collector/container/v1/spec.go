/* Copyright(C) 2021-2023. Huawei Technologies Co.,Ltd. All rights reserved.
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

// Package v1 implement the containerd client
package v1

// Spec is the base configuration for the container.
type Spec struct {
	// Linux is platform-specific configuration for Linux based containers.
	Linux *Linux `json:"linux,omitempty" platform:"linux"`
	// Process for get capabilities
	Process *Process `json:"process,omitempty" platform:"linux"`
}

// Process is the base configuration for the container.
type Process struct {
	// Env for container env
	Env []string `json:"env,omitempty" platform:"linux"`
}

// Linux contains platform-specific configuration for Linux based containers.
type Linux struct {
	// Resources contain cgroup information for handling resource constraints
	// for the container
	Resources *LinuxResources `json:"resources,omitempty"`
	// Devices are a list of device nodes that are created for the container
}

// LinuxResources has container runtime resource constraints
type LinuxResources struct {
	// Devices configures the device allowlist.
	Devices []LinuxDeviceCgroup `json:"devices,omitempty"`
}

// LinuxDeviceCgroup represents a device rule for the devices specified to
// the device controller
type LinuxDeviceCgroup struct {
	// Allow or deny
	Allow bool `json:"allow"`
	// Device type, block, char, etc.
	Type string `json:"type,omitempty"`
	// Major is the device's major number.
	Major *int64 `json:"major,omitempty"`
	// Minor is the device's minor number.
	Minor *int64 `json:"minor,omitempty"`
	// Cgroup access permissions format, rwm.
	Access string `json:"access,omitempty"`
}
