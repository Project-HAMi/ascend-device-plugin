/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2021-2024. All rights reserved.
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

// Package isula for monitoring isula' npu allocation
package isula

// Config represents env
type Config struct {
	Env []string `json:"Env,omitempty" platform:"linux"`
}

// DeviceInfo represents device info
type DeviceInfo struct {
	PathInContainer string `json:"PathInContainer,omitempty" platform:"linux"`
}

// HostConfig represents host config content
type HostConfig struct {
	Devices    []DeviceInfo `json:"Devices,omitempty" platform:"linux"`
	Privileged bool         `json:"Privileged,omitempty" platform:"linux"`
}

// ContainerJson represents container json content
type ContainerJson struct {
	Config     *Config     `json:"Config,omitempty" platform:"linux"`
	HostConfig *HostConfig `json:"HostConfig,omitempty" platform:"linux"`
}
