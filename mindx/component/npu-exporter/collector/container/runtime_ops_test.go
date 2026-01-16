/* Copyright(C) 2025. Huawei Technologies Co.,Ltd. All rights reserved.
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

// Package container provides utilities for container monitoring and testing.
package container

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/smartystreets/goconvey/convey"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	criv1 "k8s.io/cri-api/pkg/apis/runtime/v1"
	"k8s.io/cri-api/pkg/apis/runtime/v1alpha2"

	"ascend-common/common-utils/utils"
	"huawei.com/npu-exporter/v6/collector/container/isula"
	"huawei.com/npu-exporter/v6/collector/container/v1"
)

const (
	// Test constants for runtime operations
	testNamespace = "test-namespace"

	// Test error messages
	testInitCriError                    = "init CRI client failed"
	testInitOciError                    = "init OCI client failed"
	testSockCheckError                  = "socket check failed"
	testCriClientEmptyError             = "criClient is empty"
	testOciClientEmptyError             = "oci client is empty"
	testUnexpectedClientError           = "unexpected client type"
	testUnexpectedContainerdClientError = "unexpected containerd client"
	testUnexpectedIsulaClientError      = "unexpected isula client"
	testCriV1alpha2                     = "runtime.v1alpha2.RuntimeService"
	testCriV1                           = "runtime.v1.RuntimeService"
)

func TestRuntimeOperatorToolInit(t *testing.T) {
	r := &RuntimeOperatorTool{
		CriEndpoint: testContainerdEndpoint,
		OciEndpoint: testContainerdEndpoint,
	}
	convey.Convey("should initialize successfully when all components succeed", t, func() {
		operator := r
		patches := gomonkey.ApplyFuncReturn(sockCheck, nil)
		defer patches.Reset()
		patches.ApplyFuncReturn((*RuntimeOperatorTool).initCriClient, nil)
		patches.ApplyFuncReturn((*RuntimeOperatorTool).initOciClient, nil)
		err := operator.Init()
		convey.So(err, convey.ShouldBeNil)
	})
	convey.Convey("should return error when socket check fails", t, func() {
		operator := r
		patches := gomonkey.ApplyFuncReturn(sockCheck, errors.New(testSockCheckError))
		defer patches.Reset()
		err := operator.Init()
		convey.So(err, convey.ShouldNotBeNil)
		convey.So(err.Error(), convey.ShouldContainSubstring, testSockCheckError)
	})
	convey.Convey("should return error when CRI client init fails", t, func() {
		operator := r
		patches := gomonkey.ApplyFuncReturn(sockCheck, nil)
		defer patches.Reset()
		patches.ApplyFuncReturn((*RuntimeOperatorTool).initCriClient, errors.New(testInitCriError))
		patches.ApplyFuncReturn((*RuntimeOperatorTool).initOciClient, nil)
		err := operator.Init()
		convey.So(err, convey.ShouldNotBeNil)
		convey.So(err.Error(), convey.ShouldContainSubstring, testInitCriError)
	})
	convey.Convey("should return error when OCI client init fails", t, func() {
		operator := r
		patches := gomonkey.ApplyFuncReturn(sockCheck, nil)
		defer patches.Reset()
		patches.ApplyFuncReturn((*RuntimeOperatorTool).initCriClient, nil)
		patches.ApplyFuncReturn((*RuntimeOperatorTool).initOciClient, errors.New(testInitOciError))
		err := operator.Init()
		convey.So(err, convey.ShouldNotBeNil)
		convey.So(err.Error(), convey.ShouldContainSubstring, testInitOciError)
	})
}

func TestRuntimeOperatorToolInitCriClient(t *testing.T) {
	convey.Convey("TestRuntimeOperatorToolInitCriClient", t, func() {
		convey.Convey("should initialize CRI client successfully for containerd", func() {
			operator := &RuntimeOperatorTool{
				CriEndpoint:  testContainerdEndpoint,
				UseOciBackup: false,
				UseCriBackup: false,
			}

			patches := gomonkey.ApplyFuncReturn(GetConnection, &grpc.ClientConn{}, nil)
			defer patches.Reset()

			err := operator.initCriClient()
			convey.So(err, convey.ShouldBeNil)
		})

		convey.Convey("should initialize CRI client successfully for isulad", func() {
			operator := &RuntimeOperatorTool{
				CriEndpoint:  DefaultIsuladAddr,
				UseOciBackup: false,
				UseCriBackup: false,
			}

			patches := gomonkey.ApplyFuncReturn(GetConnection, &grpc.ClientConn{}, nil)
			defer patches.Reset()

			err := operator.initCriClient()
			convey.So(err, convey.ShouldBeNil)
		})

		convey.Convey("should return error when connection fails and no backup", func() {
			operator := &RuntimeOperatorTool{
				CriEndpoint:  testContainerdEndpoint,
				UseOciBackup: false,
				UseCriBackup: false,
			}

			patches := gomonkey.ApplyFuncReturn(GetConnection, nil, errors.New("connection failed"))
			defer patches.Reset()

			err := operator.initCriClient()
			convey.So(err, convey.ShouldNotBeNil)
		})
	})
}

func TestRuntimeOperatorToolInitOciClient(t *testing.T) {
	testCases := buildInitOciClientTestCases()
	for _, tc := range testCases {
		convey.Convey(tc.name, t, func() {
			operator, patches := tc.setup()
			if patches != nil {
				defer patches.Reset()
			}
			err := operator.initOciClient()
			if tc.hasError {
				convey.So(err, convey.ShouldNotBeNil)
			} else {
				convey.So(err, convey.ShouldBeNil)
			}
		})
	}
}

type initOciClientTestCase struct {
	name     string
	setup    func() (*RuntimeOperatorTool, *gomonkey.Patches)
	hasError bool
}

func buildInitOciClientTestCases() []initOciClientTestCase {
	return []initOciClientTestCase{
		{name: "should initialize OCI client successfully for containerd",
			setup: func() (*RuntimeOperatorTool, *gomonkey.Patches) {
				op := &RuntimeOperatorTool{OciEndpoint: testContainerdEndpoint, UseOciBackup: false}
				p := gomonkey.ApplyFuncReturn(GetConnection, &grpc.ClientConn{}, nil)
				return op, p
			},
			hasError: false},
		{name: "should initialize OCI client successfully for isulad",
			setup: func() (*RuntimeOperatorTool, *gomonkey.Patches) {
				op := &RuntimeOperatorTool{OciEndpoint: DefaultIsuladAddr, UseOciBackup: false}
				p := gomonkey.ApplyFuncReturn(GetConnection, &grpc.ClientConn{}, nil)
				return op, p
			},
			hasError: false},
		{name: "should return error when connection fails and no backup",
			setup: func() (*RuntimeOperatorTool, *gomonkey.Patches) {
				op := &RuntimeOperatorTool{OciEndpoint: testContainerdEndpoint, UseOciBackup: false}
				p := gomonkey.ApplyFuncReturn(GetConnection, nil, errors.New("connection failed"))
				return op, p
			},
			hasError: true},
		{name: "should return error when OCI endpoint is empty",
			setup: func() (*RuntimeOperatorTool, *gomonkey.Patches) {
				op := &RuntimeOperatorTool{OciEndpoint: "", UseOciBackup: false}
				return op, nil
			},
			hasError: true},
		{name: "should try backup when primary connection fails",
			setup: func() (*RuntimeOperatorTool, *gomonkey.Patches) {
				op := &RuntimeOperatorTool{OciEndpoint: testContainerdEndpoint, UseOciBackup: true}
				p := gomonkey.ApplyFunc(GetConnection, func(endpoint string) (*grpc.ClientConn, error) {
					if endpoint == testContainerdEndpoint {
						return nil, errors.New("primary failed")
					}
					return nil, errors.New("backup failed")
				})
				return op, p
			},
			hasError: true},
		{name: "should return error when all connections fail",
			setup: func() (*RuntimeOperatorTool, *gomonkey.Patches) {
				op := &RuntimeOperatorTool{OciEndpoint: testContainerdEndpoint, UseOciBackup: true}
				p := gomonkey.ApplyFuncReturn(GetConnection, nil, errors.New("all failed"))
				return op, p
			},
			hasError: true},
	}
}

func TestSockCheck(t *testing.T) {
	convey.Convey("TestSockCheck", t, func() {
		convey.Convey("should pass when socket paths are valid", func() {
			operator := &RuntimeOperatorTool{
				CriEndpoint: testContainerdEndpoint,
				OciEndpoint: testContainerdEndpoint,
			}

			patches := gomonkey.ApplyFuncReturn(utils.CheckPath, "/run/containerd.sock", nil)
			defer patches.Reset()
			patches.ApplyFuncReturn(utils.DoCheckOwnerAndPermission, nil)

			err := sockCheck(operator)
			convey.So(err, convey.ShouldBeNil)
		})

		convey.Convey("should return error when CRI endpoint check fails", func() {
			operator := &RuntimeOperatorTool{
				CriEndpoint: testContainerdEndpoint,
				OciEndpoint: testContainerdEndpoint,
			}

			patches := gomonkey.ApplyFuncReturn(utils.CheckPath, "", errors.New("path check failed"))
			defer patches.Reset()

			err := sockCheck(operator)
			convey.So(err, convey.ShouldNotBeNil)
		})

		convey.Convey("should return error when CRI endpoint permission check fails", func() {
			operator := &RuntimeOperatorTool{
				CriEndpoint: testContainerdEndpoint,
				OciEndpoint: testContainerdEndpoint,
			}

			patches := gomonkey.ApplyFuncReturn(utils.CheckPath, "/run/containerd.sock", nil)
			defer patches.Reset()
			patches.ApplyFuncReturn(utils.DoCheckOwnerAndPermission, errors.New("permission check failed"))

			err := sockCheck(operator)
			convey.So(err, convey.ShouldNotBeNil)
		})
	})
}

func TestRuntimeOperatorToolClose(t *testing.T) {
	convey.Convey("TestRuntimeOperatorToolClose", t, func() {
		convey.Convey("should close connections successfully", func() {
			operator := &RuntimeOperatorTool{
				conn:    &grpc.ClientConn{},
				criConn: &grpc.ClientConn{},
			}

			patches := gomonkey.ApplyFunc((*grpc.ClientConn).Close, func(*grpc.ClientConn) error {
				return nil
			})
			defer patches.Reset()

			err := operator.Close()
			convey.So(err, convey.ShouldBeNil)
		})

		convey.Convey("should return error when OCI connection close fails", func() {
			operator := &RuntimeOperatorTool{
				conn:    &grpc.ClientConn{},
				criConn: &grpc.ClientConn{},
			}

			patches := gomonkey.ApplyFunc((*grpc.ClientConn).Close, func(*grpc.ClientConn) error {
				return errors.New("close failed")
			})
			defer patches.Reset()

			err := operator.Close()
			convey.So(err, convey.ShouldNotBeNil)
		})
	})
}

func TestRuntimeOperatorToolGetContainers(t *testing.T) {
	convey.Convey("TestRuntimeOperatorToolGetContainers", t, func() {
		convey.Convey("should return error when CRI client is empty", func() {
			operator := &RuntimeOperatorTool{}

			patches := gomonkey.ApplyFuncReturn(utils.IsNil, true)
			defer patches.Reset()

			containers, err := operator.GetContainers(context.Background())
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldEqual, testCriClientEmptyError)
			convey.So(containers, convey.ShouldBeNil)
		})

		convey.Convey("should return error when CRI connection is nil", func() {
			operator := &RuntimeOperatorTool{
				criClient: "mock-client",
			}

			patches := gomonkey.ApplyFuncReturn(utils.IsNil, false)
			defer patches.Reset()

			containers, err := operator.GetContainers(context.Background())
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldEqual, testCriClientEmptyError)
			convey.So(containers, convey.ShouldBeNil)
		})

		convey.Convey("should return error when client type is unexpected", func() {
			operator := &RuntimeOperatorTool{
				criClient: "unexpected",
				criConn:   &grpc.ClientConn{},
			}

			patches := gomonkey.ApplyFuncReturn(utils.IsNil, false)
			defer patches.Reset()

			containers, err := operator.GetContainers(context.Background())
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldEqual, testUnexpectedClientError)
			convey.So(containers, convey.ShouldBeNil)
		})
	})
}

func TestIsUnimplementedError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		serviceName string
		want        bool
	}{
		{
			name:        "nil error returns false",
			err:         nil,
			serviceName: testCriV1alpha2,
			want:        false,
		},
		{
			name:        "non-grpc error returns false",
			err:         errors.New("unknown service " + testCriV1alpha2),
			serviceName: testCriV1alpha2,
			want:        false,
		},
		{
			name:        "mismatched code returns false",
			err:         status.Error(codes.NotFound, "unknown service "+testCriV1alpha2),
			serviceName: testCriV1alpha2,
			want:        false,
		},
		{
			name:        "mismatched message returns false",
			err:         status.Error(codes.Unimplemented, "unknown service "+testCriV1),
			serviceName: testCriV1alpha2,
			want:        false,
		},
		{
			name:        "matched unimplemented error returns true",
			err:         status.Error(codes.Unimplemented, "unknown service "+testCriV1alpha2),
			serviceName: testCriV1alpha2,
			want:        true,
		},
		{
			name:        "real grpc error format returns true",
			err:         fmt.Errorf("rpc error: code = Unimplemented desc = unknown service " + testCriV1alpha2),
			serviceName: testCriV1alpha2,
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isUnimplementedError(tt.err, tt.serviceName); got != tt.want {
				t.Errorf("isUnimplementedError() = %v, want %v (err: %v)", got, tt.want, tt.err)
			}
		})
	}
}

func TestRuntimeOperatorToolGetContainerInfoByID(t *testing.T) {
	convey.Convey("TestRuntimeOperatorToolGetContainerInfoByID", t, func() {
		convey.Convey("should return error when OCI client is empty", func() {
			operator := &RuntimeOperatorTool{}
			patches := gomonkey.ApplyFuncReturn(utils.IsNil, true)
			defer patches.Reset()
			spec, err := operator.GetContainerInfoByID(context.Background(), testContainerID)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldEqual, testOciClientEmptyError)
			convey.So(spec, convey.ShouldResemble, v1.Spec{})
		})
		convey.Convey("should return error when OCI connection is nil", func() {
			operator := &RuntimeOperatorTool{client: "mock-client"}
			patches := gomonkey.ApplyFuncReturn(utils.IsNil, false)
			defer patches.Reset()
			spec, err := operator.GetContainerInfoByID(context.Background(), testContainerID)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldEqual, testOciClientEmptyError)
			convey.So(spec, convey.ShouldResemble, v1.Spec{})
		})
		convey.Convey("should return error when client type is unexpected", func() {
			operator := &RuntimeOperatorTool{client: "unexpected", conn: &grpc.ClientConn{}}
			patches := gomonkey.ApplyFuncReturn(utils.IsNil, false)
			defer patches.Reset()
			spec, err := operator.GetContainerInfoByID(context.Background(), testContainerID)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldEqual, testUnexpectedContainerdClientError)
			convey.So(spec, convey.ShouldResemble, v1.Spec{})
		})
		convey.Convey("should return error when GetContainer call fails", func() {
			operator := &RuntimeOperatorTool{client: "mock-containers-client", conn: &grpc.ClientConn{}}
			patches := gomonkey.ApplyFuncReturn(utils.IsNil, false)
			defer patches.Reset()
			spec, err := operator.GetContainerInfoByID(context.Background(), testContainerID)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(spec, convey.ShouldResemble, v1.Spec{})
		})
		convey.Convey("should return error when JSON unmarshal fails", func() {
			operator := &RuntimeOperatorTool{client: "mock-containers-client", conn: &grpc.ClientConn{}}
			patches := gomonkey.ApplyFuncReturn(utils.IsNil, false)
			defer patches.Reset()
			spec, err := operator.GetContainerInfoByID(context.Background(), testContainerID)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(spec, convey.ShouldResemble, v1.Spec{})
		})

	})
}

func TestRuntimeOperatorToolGetIsulaContainerInfoByID(t *testing.T) {
	convey.Convey("TestRuntimeOperatorToolGetIsulaContainerInfoByID", t, func() {
		convey.Convey("should return error when OCI client is empty", func() {
			operator := &RuntimeOperatorTool{}
			patches := gomonkey.ApplyFuncReturn(utils.IsNil, true)
			defer patches.Reset()
			containerInfo, err := operator.GetIsulaContainerInfoByID(context.Background(), testContainerID)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldEqual, testOciClientEmptyError)
			convey.So(containerInfo, convey.ShouldResemble, isula.ContainerJson{})
		})
		convey.Convey("should return error when OCI connection is nil", func() {
			operator := &RuntimeOperatorTool{client: "mock-client"}
			patches := gomonkey.ApplyFuncReturn(utils.IsNil, false)
			defer patches.Reset()
			containerInfo, err := operator.GetIsulaContainerInfoByID(context.Background(), testContainerID)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldEqual, testOciClientEmptyError)
			convey.So(containerInfo, convey.ShouldResemble, isula.ContainerJson{})
		})
		convey.Convey("should return error when client type is unexpected", func() {
			operator := &RuntimeOperatorTool{client: "unexpected", conn: &grpc.ClientConn{}}
			patches := gomonkey.ApplyFuncReturn(utils.IsNil, false)
			defer patches.Reset()
			containerInfo, err := operator.GetIsulaContainerInfoByID(context.Background(), testContainerID)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldEqual, testUnexpectedIsulaClientError)
			convey.So(containerInfo, convey.ShouldResemble, isula.ContainerJson{})
		})
		convey.Convey("should return error when Inspect call fails", func() {
			operator := &RuntimeOperatorTool{client: "mock-isula-client", conn: &grpc.ClientConn{}}
			patches := gomonkey.ApplyFuncReturn(utils.IsNil, false)
			defer patches.Reset()
			containerInfo, err := operator.GetIsulaContainerInfoByID(context.Background(), testContainerID)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(containerInfo, convey.ShouldResemble, isula.ContainerJson{})
		})
		convey.Convey("should return error when JSON unmarshal fails", func() {
			operator := &RuntimeOperatorTool{client: "mock-isula-client", conn: &grpc.ClientConn{}}
			patches := gomonkey.ApplyFuncReturn(utils.IsNil, false)
			defer patches.Reset()
			containerInfo, err := operator.GetIsulaContainerInfoByID(context.Background(), testContainerID)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(containerInfo, convey.ShouldResemble, isula.ContainerJson{})
		})

	})
}

func TestRuntimeOperatorToolGetContainerType(t *testing.T) {
	convey.Convey("TestRuntimeOperatorToolGetContainerType", t, func() {
		convey.Convey("should return isula when endpoint is isulad", func() {
			operator := &RuntimeOperatorTool{
				OciEndpoint: DefaultIsuladAddr,
			}

			containerType := operator.GetContainerType()
			convey.So(containerType, convey.ShouldEqual, IsulaContainer)
		})

		convey.Convey("should return default when endpoint is not isulad", func() {
			operator := &RuntimeOperatorTool{
				OciEndpoint: testContainerdEndpoint,
			}

			containerType := operator.GetContainerType()
			convey.So(containerType, convey.ShouldEqual, DefaultContainer)
		})
	})
}

func TestSetGrpcNamespaceHeader(t *testing.T) {
	convey.Convey("TestSetGrpcNamespaceHeader", t, func() {
		convey.Convey("should set namespace header when context has no metadata", func() {
			ctx := context.Background()
			result := setGrpcNamespaceHeader(ctx, testNamespace)
			convey.So(result, convey.ShouldNotBeNil)
		})

		convey.Convey("should set namespace header when context has existing metadata", func() {
			ctx := context.Background()
			ctx = context.WithValue(ctx, "test", "value")
			result := setGrpcNamespaceHeader(ctx, testNamespace)
			convey.So(result, convey.ShouldNotBeNil)
		})
	})
}

func TestGenContainerRequestV1alpha2(t *testing.T) {
	convey.Convey("TestGenContainerRequestV1alpha2", t, func() {
		convey.Convey("should generate valid container request", func() {
			request := genContainerRequestV1alpha2()
			convey.So(request, convey.ShouldNotBeNil)
			convey.So(request.Filter, convey.ShouldNotBeNil)
			convey.So(request.Filter.State, convey.ShouldNotBeNil)
			convey.So(request.Filter.State.State, convey.ShouldEqual, v1alpha2.ContainerState_CONTAINER_RUNNING)
		})
	})
}

func TestGenContainerRequestV1(t *testing.T) {
	convey.Convey("TestGenContainerRequestV1", t, func() {
		convey.Convey("should generate valid container request", func() {
			request := genContainerRequestV1()
			convey.So(request, convey.ShouldNotBeNil)
			convey.So(request.Filter, convey.ShouldNotBeNil)
			convey.So(request.Filter.State, convey.ShouldNotBeNil)
			convey.So(request.Filter.State.State, convey.ShouldEqual, criv1.ContainerState_CONTAINER_RUNNING)
		})
	})
}

func TestGenIsulaRequest(t *testing.T) {
	convey.Convey("TestGenIsulaRequest", t, func() {
		convey.Convey("should generate valid isula request", func() {
			request := genIsulaRequest()
			convey.So(request, convey.ShouldNotBeNil)
			convey.So(request.Filter, convey.ShouldNotBeNil)
			convey.So(request.Filter.State, convey.ShouldNotBeNil)
			convey.So(request.Filter.State.State, convey.ShouldEqual, isula.ContainerState_CONTAINER_RUNNING)
		})
	})
}
