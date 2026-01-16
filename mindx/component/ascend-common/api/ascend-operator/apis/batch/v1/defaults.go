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
	"errors"
	"fmt"
	"strings"

	commonv1 "github.com/kubeflow/common/pkg/apis/common/v1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"ascend-common/api"
)

// Int32 is a helper routine that allocates a new int32 value
// to store v and returns a pointer to it.
func Int32(v int32) *int32 {
	return &v
}

// addDefaultingFuncs is used to register default funcs
func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

// setDefaultPort sets the default ports for mindxdl container.
func setDefaultPort(spec *v1.PodSpec) {
	index := 0
	for i, container := range spec.Containers {
		if container.Name == api.DefaultContainerName {
			index = i
			break

		}
	}
	hasASJobPort := false
	for _, port := range spec.Containers[index].Ports {
		if port.Name == api.DefaultPortName {
			hasASJobPort = true
			break
		}
	}
	if !hasASJobPort {
		spec.Containers[index].Ports = append(spec.Containers[index].Ports, v1.ContainerPort{
			Name:          api.DefaultPortName,
			ContainerPort: DefaultPort,
		})
	}
}

func setDefaultReplicas(spec *commonv1.ReplicaSpec) {
	if spec.Replicas == nil {
		spec.Replicas = Int32(1)
	}
	if spec.RestartPolicy == "" {
		spec.RestartPolicy = DefaultRestartPolicy
	}
}

// setTypeNamesToCamelCase sets the name of all replica types from any case to correct case.
func setTypeNamesToCamelCase(job *AscendJob) {
	setTypeNameToCamelCase(job, MindSporeReplicaTypeScheduler)
	setTypeNameToCamelCase(job, ReplicaTypeWorker)
	setTypeNameToCamelCase(job, PytorchReplicaTypeMaster)
	setTypeNameToCamelCase(job, TensorflowReplicaTypeChief)
}

// setTypeNameToCamelCase sets the name of the replica type from any case to correct case.
// E.g. from ps to PS; from WORKER to Worker.
func setTypeNameToCamelCase(job *AscendJob, typ commonv1.ReplicaType) {
	for t := range job.Spec.ReplicaSpecs {
		if strings.EqualFold(string(t), string(typ)) && t != typ {
			spec := job.Spec.ReplicaSpecs[t]
			delete(job.Spec.ReplicaSpecs, t)
			job.Spec.ReplicaSpecs[typ] = spec
			return
		}
	}
}

// SetDefaultsAscendJob sets any unspecified values to defaults.
func SetDefaultsAscendJob(job *AscendJob) {
	// Set default cleanpod policy to Running.
	if job == nil {
		return
	}

	if job.Spec.RunPolicy.CleanPodPolicy == nil {
		running := commonv1.CleanPodPolicyNone
		job.Spec.RunPolicy.CleanPodPolicy = &running
	}
	// Set default success policy to "".
	if job.Spec.SuccessPolicy == nil {
		defaultPolicy := SuccessPolicyDefault
		job.Spec.SuccessPolicy = &defaultPolicy
	}

	// Update the key of replicaSpecs to camel case.
	setTypeNamesToCamelCase(job)

	for rt, spec := range job.Spec.ReplicaSpecs {
		// Set default replicas to 1.
		setDefaultReplicas(spec)
		// Set default port to ml container.
		if rt == MindSporeReplicaTypeScheduler || rt == PytorchReplicaTypeMaster || rt == TensorflowReplicaTypeChief {
			setDefaultPort(&spec.Template.Spec)
		}
	}
}

// GetJobFramework get framework name of ascendjob
func GetJobFramework(job *AscendJob) (string, error) {
	if job == nil || job.Labels == nil {
		return "", errors.New("job or job labels is nil")
	}
	frame, ok := job.Labels[FrameworkKey]
	if !ok {
		return "", fmt.Errorf("job<%s-%s> label framework is not set", job.Namespace, job.Name)
	}
	return frame, nil
}
