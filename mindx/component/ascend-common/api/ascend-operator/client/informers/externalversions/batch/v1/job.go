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
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"

	batchv1 "ascend-common/api/ascend-operator/apis/batch/v1"
	"ascend-common/api/ascend-operator/client/clientset/versioned"
	"ascend-common/api/ascend-operator/client/informers/externalversions/internalinterfaces"
	batchlister "ascend-common/api/ascend-operator/client/listers/batch/v1"
)

// JobInformer provides access to a shared informer and lister for
// Jobs.
type JobInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() batchlister.JobLister
}

type jobInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewJobInformer constructs a new informer for Job type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewJobInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration,
	indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredJobInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredJobInformer constructs a new informer for Job type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredJobInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration,
	indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.BatchV1().Jobs(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.BatchV1().Jobs(namespace).Watch(context.TODO(), options)
			},
		},
		&batchv1.AscendJob{},
		resyncPeriod,
		indexers,
	)
}

func (f *jobInformer) defaultInformer(client versioned.Interface,
	resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredJobInformer(client, f.namespace, resyncPeriod, cache.Indexers{
		cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *jobInformer) Informer() cache.SharedIndexInformer {
	if f == nil || f.factory == nil {
		return nil
	}
	return f.factory.InformerFor(&batchv1.AscendJob{}, f.defaultInformer)
}

func (f *jobInformer) Lister() batchlister.JobLister {
	if f == nil {
		return nil
	}
	return batchlister.NewJobLister(f.Informer().GetIndexer())
}
