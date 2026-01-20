// Copyright (c) Huawei Technologies Co., Ltd. 2025-2025. All rights reserved.

// Package api structs for public fault
package api

// PubFaultInfo struct for public fault input
type PubFaultInfo struct {
	Id        string  `json:"id"`
	TimeStamp int64   `json:"timestamp"`
	Version   string  `json:"version"`
	Resource  string  `json:"resource"`
	Faults    []Fault `json:"faults"`
}

// Fault public fault cm item Fault
type Fault struct {
	FaultId       string            `json:"faultId"`
	FaultType     string            `json:"faultType"`
	FaultCode     string            `json:"faultCode"`
	FaultTime     int64             `json:"faultTime"`
	Assertion     string            `json:"assertion"`
	FaultLocation map[string]string `json:"faultLocation"`
	Influence     []Influence       `json:"influence"`
	Description   string            `json:"description"`
}

// Influence public fault cm item Influence
type Influence struct {
	NodeName  string  `json:"nodeName"`
	NodeSN    string  `json:"nodeSN"`
	DeviceIds []int32 `json:"deviceIds"`
}
