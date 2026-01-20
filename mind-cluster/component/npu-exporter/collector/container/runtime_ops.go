/* Copyright(C) 2021-2023. Huawei Technologies Co.,Ltd. All rights reserved.
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

// Package container for monitoring containers' npu allocation
package container

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"syscall"

	"ascend-common/common-utils/hwlog"
	"ascend-common/common-utils/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	criv1 "k8s.io/cri-api/pkg/apis/runtime/v1"
	"k8s.io/cri-api/pkg/apis/runtime/v1alpha2"

	"huawei.com/npu-exporter/v6/collector/container/isula"
	"huawei.com/npu-exporter/v6/collector/container/v1"
	"huawei.com/npu-exporter/v6/utils/logger"
)

const (
	labelK8sPodNamespace = "io.kubernetes.pod.namespace"
	labelK8sPodName      = "io.kubernetes.pod.name"
	labelContainerName   = "io.kubernetes.container.name"

	// DefaultIsuladAddr default isulad sock adress
	DefaultIsuladAddr = "unix:///run/isulad.sock"
	// DefaultDockerShim default docker shim sock address
	DefaultDockerShim = "unix:///run/dockershim.sock"
	// DefaultCRIDockerd default cri-dockerd  sock address
	DefaultCRIDockerd = "unix:///run/cri-dockerd.sock"
	// DefaultContainerdAddr default containerd sock address
	DefaultContainerdAddr = "unix:///run/containerd/containerd.sock"
	// DefaultDockerAddr default docker containerd sock address
	DefaultDockerAddr    = "unix:///run/docker/containerd/docker-containerd.sock"
	defaultDockerOnEuler = "unix:///run/docker/containerd/containerd.sock"
	grpcHeader           = "containerd-namespace"
	unixPre              = "unix://"

	// IsulaContainer represents isula container type
	IsulaContainer = "isula"
	// DefaultContainer represents default container type
	DefaultContainer   = "docker-containerd"
	excludePermissions = 0002

	criV1alpha2 = "runtime.v1alpha2.RuntimeService"
)

// CommonContainer wraps some common container attribute of isulad and containerd
type CommonContainer struct {
	Id     string
	Labels map[string]string
}

// RuntimeOperator wraps operations against container runtime
type RuntimeOperator interface {
	Init() error
	Close() error
	GetContainers(ctx context.Context) ([]*CommonContainer, error)
	GetContainerInfoByID(ctx context.Context, id string) (v1.Spec, error)
	GetIsulaContainerInfoByID(ctx context.Context, id string) (isula.ContainerJson, error)
	GetContainerType() string
}

// RuntimeOperatorTool implements RuntimeOperator interface
type RuntimeOperatorTool struct {
	criConn   *grpc.ClientConn
	conn      *grpc.ClientConn
	criClient interface{}
	client    interface{}
	// CriEndpoint CRI server endpoint
	CriEndpoint string
	// OciEndpoint containerd Server endpoint
	OciEndpoint string
	// Namespace the namespace of containerd
	Namespace string
	// UseCriBackup use cri back up address or not
	UseCriBackup bool
	// UseOciBackup use oci back up address or not
	UseOciBackup bool
}

// Init initializes container runtime operator
func (operator *RuntimeOperatorTool) Init() error {
	start := syscall.Getuid()
	logger.Debugf("the init uid is:%d", start)
	if start != 0 {
		err := syscall.Setuid(0)
		if err != nil {
			return fmt.Errorf("raise uid failed: %v", err)
		}
		logger.Debugf("raise uid to:%d", 0)
		defer func() {
			err = syscall.Setuid(start)
			if err != nil {
				logger.Errorf("recover uid failed: %v", err)
			}
			logger.Debugf("recover uid to:%d", start)
		}()
	}
	if err := sockCheck(operator); err != nil {
		hwlog.RunLog.Error("check socket path failed")
		return err
	}

	if err := operator.initCriClient(); err != nil {
		return fmt.Errorf("init CRI client failed, %s", err)
	}

	if err := operator.initOciClient(); err != nil {
		return fmt.Errorf("init OCI client failed, %s", err)
	}
	return nil
}

func (operator *RuntimeOperatorTool) initCriClient() error {
	criConn, err := GetConnection(operator.CriEndpoint)
	if err != nil || criConn == nil {
		msg := fmt.Sprintf("connecting to CRI server failed: %v", err)
		if operator.UseCriBackup {
			logger.Warnf("%v, will use cri-dockerd address to try again", msg)
			if utils.IsExist(strings.TrimPrefix(DefaultCRIDockerd, unixPre)) {
				criConn, err = GetConnection(DefaultCRIDockerd)
			}
		} else {
			logger.Warn(msg)
		}
	}
	if err != nil {
		return fmt.Errorf("connecting to CRI server failed: %v", err)
	}
	if operator.CriEndpoint == DefaultIsuladAddr {
		operator.criClient = isula.NewRuntimeServiceClient(criConn)
	} else {
		operator.criClient = v1alpha2.NewRuntimeServiceClient(criConn)
	}
	operator.criConn = criConn
	return nil
}

func (operator *RuntimeOperatorTool) initOciClient() error {
	conn, err := GetConnection(operator.OciEndpoint)
	if err != nil || conn == nil {
		msg := fmt.Sprintf("failed to get OCI connection: %v", err)
		if operator.UseOciBackup {
			logger.Warnf("%v, will use backup address to try again", msg)
			if utils.IsExist(strings.TrimPrefix(DefaultContainerdAddr, unixPre)) {
				conn, err = GetConnection(DefaultContainerdAddr)

			} else if utils.IsExist(strings.TrimPrefix(defaultDockerOnEuler, unixPre)) {
				conn, err = GetConnection(defaultDockerOnEuler)
			}
		} else {
			logger.Warn(msg)
		}
	}
	if err != nil {
		return fmt.Errorf("connecting to OCI server failed: %v", err)
	}
	if operator.OciEndpoint == DefaultIsuladAddr {
		operator.client = isula.NewContainerServiceClient(conn)
	} else {
		operator.client = v1.NewContainersClient(conn)
	}
	operator.conn = conn
	return nil
}

func sockCheck(operator *RuntimeOperatorTool) error {
	absPath, err := utils.CheckPath(strings.TrimPrefix(operator.CriEndpoint, unixPre))
	if err != nil {
		return err
	}
	if err := utils.DoCheckOwnerAndPermission(absPath, excludePermissions, 0); err != nil {
		return err
	}

	absPath, err = utils.CheckPath(strings.TrimPrefix(operator.OciEndpoint, unixPre))
	if err != nil {
		return err
	}
	if err := utils.DoCheckOwnerAndPermission(absPath, excludePermissions, 0); err != nil {
		return err
	}
	return nil
}

// Close closes container runtime operator
func (operator *RuntimeOperatorTool) Close() error {
	err := operator.conn.Close()
	if err != nil {
		return err
	}
	err = operator.criConn.Close()
	if err != nil {
		return err
	}
	return nil
}

// GetContainers returns all containers' IDs
func (operator *RuntimeOperatorTool) GetContainers(ctx context.Context) ([]*CommonContainer, error) {
	if utils.IsNil(operator.criClient) || operator.criConn == nil {
		return nil, errors.New("criClient is empty")
	}
	if client, ok := operator.criClient.(v1alpha2.RuntimeServiceClient); ok {
		containers, err := getContainersByContainerdV1alpha2(ctx, client)
		if isUnimplementedError(err, criV1alpha2) {
			v1Client := criv1.NewRuntimeServiceClient(operator.criConn)
			return getContainersByContainerdV1(ctx, v1Client)
		}
		return containers, err
	}
	if client, ok := operator.criClient.(isula.RuntimeServiceClient); ok {
		return getContainersByIsulad(ctx, client)
	}

	logger.Errorf("client %v is unexpected", operator.criClient)
	return nil, errors.New("unexpected client type")
}

func isUnimplementedError(err error, serviceName string) bool {
	if err == nil {
		return false
	}
	st, ok := status.FromError(err)
	if ok {
		return st.Code() == codes.Unimplemented && strings.Contains(st.Message(), serviceName)
	}
	errStr := err.Error()
	if strings.Contains(errStr, "code = Unimplemented") &&
		strings.Contains(errStr, "desc = ") && strings.Contains(errStr, serviceName) {
		return true
	}
	return false
}

// GetContainerInfoByID use oci interface to get container
func (operator *RuntimeOperatorTool) GetContainerInfoByID(ctx context.Context, id string) (v1.Spec, error) {
	if utils.IsNil(operator.client) || operator.conn == nil {
		return v1.Spec{}, errors.New("oci client is empty")
	}

	s := v1.Spec{}
	if client, ok := operator.client.(v1.ContainersClient); ok {
		resp, err := client.Get(setGrpcNamespaceHeader(ctx, operator.Namespace), &v1.GetContainerRequest{
			Id: id,
		})
		if err != nil {
			hwlog.RunLog.Error("get call OCI get method failed")
			return v1.Spec{}, err
		}
		if err = json.Unmarshal(resp.Container.Spec.Value, &s); err != nil {
			hwlog.RunLog.Error("unmarshal OCI response failed")
			return v1.Spec{}, err
		}
		return s, nil
	}

	return s, errors.New("unexpected containerd client")
}

// GetIsulaContainerInfoByID return isula container info
func (operator *RuntimeOperatorTool) GetIsulaContainerInfoByID(ctx context.Context,
	id string) (isula.ContainerJson, error) {
	containerJsonInfo := isula.ContainerJson{}
	if utils.IsNil(operator.client) || operator.conn == nil {
		return containerJsonInfo, errors.New("oci client is empty")
	}

	if client, ok := operator.client.(isula.ContainerServiceClient); ok {
		resp, err := client.Inspect(setGrpcNamespaceHeader(ctx, operator.Namespace), &isula.InspectContainerRequest{
			Id: id,
		})
		if err != nil {
			hwlog.RunLog.Error("call isula OCI Inspect method failed")
			return containerJsonInfo, err
		}
		if err = json.Unmarshal([]byte(resp.ContainerJSON), &containerJsonInfo); err != nil {
			logger.Errorf("unmarshal err: %v", err)
			return containerJsonInfo, err
		}
		return containerJsonInfo, nil
	}

	return containerJsonInfo, errors.New("unexpected isula client")
}

// GetContainerType return container type
func (operator *RuntimeOperatorTool) GetContainerType() string {
	if operator.OciEndpoint == DefaultIsuladAddr {
		return IsulaContainer
	}
	return DefaultContainer
}

type nsKey struct{}

func setGrpcNamespaceHeader(ctx context.Context, namespace string) context.Context {
	context.WithValue(ctx, nsKey{}, namespace)
	ns := metadata.Pairs(grpcHeader, namespace)
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = ns
	} else {
		md = metadata.Join(ns, md)
	}
	return metadata.NewOutgoingContext(ctx, md)
}

func getContainersByContainerdV1alpha2(ctx context.Context,
	client v1alpha2.RuntimeServiceClient) ([]*CommonContainer, error) {
	var allContainers []*CommonContainer
	request := genContainerRequestV1alpha2()
	r, err := client.ListContainers(ctx, request)
	if err != nil {
		hwlog.RunLog.Warn(err)
		return nil, err
	}
	for _, container := range r.Containers {
		allContainers = append(allContainers, &CommonContainer{
			Id:     container.Id,
			Labels: container.Labels,
		})
	}
	return allContainers, nil
}

func getContainersByContainerdV1(ctx context.Context, client criv1.RuntimeServiceClient) ([]*CommonContainer, error) {
	var allContainers []*CommonContainer
	request := genContainerRequestV1()
	r, err := client.ListContainers(ctx, request)
	if err != nil {
		hwlog.RunLog.Error(err)
		return nil, err
	}
	for _, container := range r.Containers {
		allContainers = append(allContainers, &CommonContainer{
			Id:     container.Id,
			Labels: container.Labels,
		})
	}
	return allContainers, nil
}

func getContainersByIsulad(ctx context.Context, client isula.RuntimeServiceClient) ([]*CommonContainer, error) {
	var allContainers []*CommonContainer
	request := genIsulaRequest()
	r, err := client.ListContainers(ctx, request)
	if err != nil {
		hwlog.RunLog.Error(err)
		return nil, err
	}
	for _, container := range r.Containers {
		allContainers = append(allContainers, &CommonContainer{
			Id:     container.Id,
			Labels: container.Labels,
		})
	}
	return allContainers, nil
}

func genContainerRequestV1alpha2() *v1alpha2.ListContainersRequest {
	filter := &v1alpha2.ContainerFilter{}
	st := &v1alpha2.ContainerStateValue{}
	st.State = v1alpha2.ContainerState_CONTAINER_RUNNING
	filter.State = st
	request := &v1alpha2.ListContainersRequest{
		Filter: filter,
	}
	return request
}

func genContainerRequestV1() *criv1.ListContainersRequest {
	filter := &criv1.ContainerFilter{}
	st := &criv1.ContainerStateValue{}
	st.State = criv1.ContainerState_CONTAINER_RUNNING
	filter.State = st
	request := &criv1.ListContainersRequest{
		Filter: filter,
	}
	return request
}

func genIsulaRequest() *isula.ListContainersRequest {
	filter := &isula.ContainerFilter{}
	st := &isula.ContainerStateValue{}
	st.State = isula.ContainerState_CONTAINER_RUNNING
	filter.State = st
	request := &isula.ListContainersRequest{
		Filter: filter,
	}
	return request
}
