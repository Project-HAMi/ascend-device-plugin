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

// Package v1 is used to define some client- and job-related interfaces, initialization operations,
// and method implementations.
package v1

import (
	"errors"
	"net/http"

	"k8s.io/client-go/rest"

	"ascend-common/api/ascend-operator/apis/batch/v1"
	"ascend-common/api/ascend-operator/client/clientset/versioned/scheme"
)

// BatchV1Interface is a batch client interface.
type BatchV1Interface interface {
	RESTClient() rest.Interface
	JobsGetter
}

// BatchV1Client is a client structure.
type BatchV1Client struct {
	restClient rest.Interface
}

// Jobs returns a JobInterface object instance.
func (c *BatchV1Client) Jobs(namespace string) JobInterface {
	if c == nil {
		return nil
	}
	return newJobs(c, namespace)
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *BatchV1Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}

// NewForConfig creates a new BatchV1alpha1Client for the given config.
// NewForConfig is equivalent to NewForConfigAndClient(c, httpClient),
// where httpClient was generated with rest.HTTPClientFor(c).
func NewForConfig(c *rest.Config) (*BatchV1Client, error) {
	if c == nil {
		return nil, errors.New(nilPointError)
	}
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	httpClient, err := rest.HTTPClientFor(&config)
	if err != nil {
		return nil, err
	}
	return NewForConfigAndClient(&config, httpClient)
}

func setConfigDefaults(config *rest.Config) error {
	gv := v1.SchemeGroupVersion
	config.GroupVersion = &gv
	config.APIPath = "/apis"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()

	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	return nil
}

// NewForConfigAndClient creates a new BatchV1alpha1Client for the given config and http client.
// Note the http client provided takes precedence over the configured transport values.
func NewForConfigAndClient(c *rest.Config, h *http.Client) (*BatchV1Client, error) {
	if c == nil || h == nil {
		return nil, errors.New(nilPointError)
	}
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientForConfigAndClient(&config, h)
	if err != nil {
		return nil, err
	}
	return &BatchV1Client{restClient: client}, nil
}

// New creates a new BatchV1alpha1Client for the given RESTClient.
func New(c rest.Interface) *BatchV1Client {
	return &BatchV1Client{restClient: c}
}
