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

// Package scheme is used to add runtime.Scheme
package scheme

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"ascend-common/api/ascend-operator/apis/batch/v1"
)

// RuntimeScheme is a Scheme object instance.
var RuntimeScheme = runtime.NewScheme()

// Codecs is a CodecFactory object instance.
var Codecs = serializer.NewCodecFactory(RuntimeScheme)

// ParameterCodec is a parameterCodec object instance.
var ParameterCodec = runtime.NewParameterCodec(RuntimeScheme)

func init() {
	utilruntime.Must(v1.AddToScheme(RuntimeScheme))
}
