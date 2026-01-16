// Copyright (c) Huawei Technologies Co., Ltd. 2025-2025. All rights reserved.

// Package api common const
package api

// ResetCmInfo is the reset config info of a task
type ResetCmInfo struct {
	RankList            []*DevFaultnfo
	UpdateTime          int64
	RetryTime           int
	FaultFlushing       bool
	GracefulExit        int
	RestartFaultProcess bool
}

// DevFaultnfo is the device info of a task
type DevFaultnfo struct {
	RankId int
	FaultInfo
}

// FaultInfo is the fault info of device
type FaultInfo struct {
	LogicId       int32
	Status        string
	Policy        string
	InitialPolicy string
	ErrorCode     []int64
	ErrorCodeHex  string
}
