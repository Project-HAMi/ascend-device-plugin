/*
Copyright 2023 Huawei Technologies Co., Ltd.

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

package v1

import (
	"github.com/kubeflow/common/pkg/apis/common/v1"
)

const (
	// GroupName is the group name used in this package.
	GroupName = "mindxdl.gitee.com"

	// FrameworkKey the key of the laebl
	FrameworkKey = "framework"

	// DefaultPort is default value of the port.
	DefaultPort = 2222

	// MindSporeFrameworkName is the name of ML Framework
	MindSporeFrameworkName = "mindspore"
	// MindSporeReplicaTypeScheduler is the type for Scheduler of distribute ML
	MindSporeReplicaTypeScheduler v1.ReplicaType = "Scheduler"

	// PytorchFrameworkName is the name of ML Framework
	PytorchFrameworkName = "pytorch"
	// PytorchReplicaTypeMaster is the type for Scheduler of distribute ML
	PytorchReplicaTypeMaster v1.ReplicaType = "Master"

	// TensorflowFrameworkName is the name of ML Framework
	TensorflowFrameworkName = "tensorflow"
	// TensorflowReplicaTypeChief is the type for Scheduler of distribute ML
	TensorflowReplicaTypeChief v1.ReplicaType = "Chief"

	// ReplicaTypeWorker this is also used for non-distributed AscendJob
	ReplicaTypeWorker v1.ReplicaType = "Worker"

	// DefaultRestartPolicy is default RestartPolicy for MSReplicaSpec.
	DefaultRestartPolicy = v1.RestartPolicyNever
)
