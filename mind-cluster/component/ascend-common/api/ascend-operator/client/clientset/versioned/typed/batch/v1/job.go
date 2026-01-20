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
	"context"
	"errors"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"

	"ascend-common/api"
	"ascend-common/api/ascend-operator/apis/batch/v1"
	"ascend-common/api/ascend-operator/client/clientset/versioned/scheme"
)

const (
	nilPointError = "nil pointer"
)

// JobsGetter has a method to return a JobInterface.
// A group's client should implement this interface.
type JobsGetter interface {
	Jobs(namespace string) JobInterface
}

// JobInterface has methods to work with Job resources.
type JobInterface interface {
	Create(ctx context.Context, job *v1.AscendJob, opts metav1.CreateOptions) (*v1.AscendJob, error)
	Update(ctx context.Context, job *v1.AscendJob, opts metav1.UpdateOptions) (*v1.AscendJob, error)
	UpdateStatus(ctx context.Context, job *v1.AscendJob, opts metav1.UpdateOptions) (*v1.AscendJob, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1.AscendJob, error)
	List(ctx context.Context, opts metav1.ListOptions) (*v1.AscendJobList, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions,
		subresources ...string) (result *v1.AscendJob, err error)
	// JobExpansion
}

// jobs implements JobInterface
type jobs struct {
	client rest.Interface
	ns     string
}

func (j *jobs) Create(ctx context.Context, job *v1.AscendJob, opts metav1.CreateOptions) (*v1.AscendJob, error) {
	if j == nil {
		return nil, errors.New(nilPointError)
	}
	result := &v1.AscendJob{}
	err := j.client.Post().
		Namespace(j.ns).
		Resource(api.AscendJobsLowerCase).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(job).
		Do(ctx).
		Into(result)
	return result, err
}

func (j *jobs) Update(ctx context.Context, job *v1.AscendJob, opts metav1.UpdateOptions) (*v1.AscendJob,
	error) {
	if j == nil || job == nil {
		return nil, errors.New(nilPointError)
	}
	result := &v1.AscendJob{}
	err := j.client.Put().
		Namespace(j.ns).
		Resource(api.AscendJobsLowerCase).
		Name(job.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(job).
		Do(ctx).
		Into(result)
	return result, err
}

func (j *jobs) UpdateStatus(ctx context.Context, job *v1.AscendJob, opts metav1.UpdateOptions) (*v1.AscendJob,
	error) {
	if j == nil || job == nil {
		return nil, errors.New(nilPointError)
	}
	result := &v1.AscendJob{}
	err := j.client.Put().
		Namespace(j.ns).
		Resource(api.AscendJobsLowerCase).
		Name(job.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(job).
		Do(ctx).
		Into(result)
	return result, err
}

func (j *jobs) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	if j == nil {
		return errors.New(nilPointError)
	}
	return j.client.Delete().
		Namespace(j.ns).
		Resource(api.AscendJobsLowerCase).
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

func (j *jobs) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	if j == nil {
		return errors.New(nilPointError)
	}
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return j.client.Delete().
		Namespace(j.ns).
		Resource(api.AscendJobsLowerCase).
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

func (j *jobs) Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1.AscendJob, error) {
	if j == nil {
		return nil, errors.New(nilPointError)
	}
	result := &v1.AscendJob{}
	err := j.client.Get().
		Namespace(j.ns).
		Resource(api.AscendJobsLowerCase).
		Name(name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return result, err
}

func (j *jobs) List(ctx context.Context, opts metav1.ListOptions) (*v1.AscendJobList, error) {
	if j == nil {
		return nil, errors.New(nilPointError)
	}
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result := &v1.AscendJobList{}
	err := j.client.Get().
		Namespace(j.ns).
		Resource(api.AscendJobsLowerCase).
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return result, err
}

func (j *jobs) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	if j == nil {
		return nil, errors.New(nilPointError)
	}
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return j.client.Get().
		Namespace(j.ns).
		Resource(api.AscendJobsLowerCase).
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

func (j *jobs) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions,
	subresources ...string) (*v1.AscendJob, error) {
	if j == nil {
		return nil, errors.New(nilPointError)
	}
	result := &v1.AscendJob{}
	err := j.client.Patch(pt).
		Namespace(j.ns).
		Resource(api.AscendJobsLowerCase).
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return result, err
}

// newJobs returns a Jobs
func newJobs(c *BatchV1Client, namespace string) *jobs {
	return &jobs{
		client: c.RESTClient(),
		ns:     namespace,
	}
}
