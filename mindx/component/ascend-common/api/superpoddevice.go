// Copyright (c) Huawei Technologies Co., Ltd. 2025-2025. All rights reserved.

// Package api structs for SuperPodDevice
package api

import "k8s.io/apimachinery/pkg/util/sets"

// NpuBaseInfo is the base info of npu
type NpuBaseInfo struct {
	IP            string
	SuperDeviceID uint32
}

// NodeDevice node device info
type NodeDevice struct {
	NodeName   string
	ServerID   string
	ServerType string            `json:"-"`
	DeviceMap  map[string]string // key: dev phyID, value: superPod device id
}

// SuperPodDevice super node device info, key is superPodID, value is NodeDevice
type SuperPodDevice struct {
	Version       string
	SuperPodID    string
	NodeDeviceMap map[string]*NodeDevice
}

// SuperPodFaultInfos super pod fault info
type SuperPodFaultInfos struct {
	SdIds      []string
	FaultNodes sets.String
	NodeNames  []string
	FaultTimes int64
	JobId      string `json:"JobId,omitempty"`
}
