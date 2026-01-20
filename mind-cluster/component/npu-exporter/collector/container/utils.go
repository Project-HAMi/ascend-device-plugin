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
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"google.golang.org/grpc"

	"ascend-common/common-utils/hwlog"
	"ascend-common/common-utils/utils"
	"huawei.com/npu-exporter/v6/utils/logger"
)

const (
	defaultTimeout = 5 * time.Second
	unixPrefix     = "unix"
	// MaxLenDNS configName max len
	MaxLenDNS = 512
	// MinLenDNS configName min len
	MinLenDNS     = 1
	maxContainers = 1024
	maxCgroupPath = 2048

	maxDevicesNum = 100000
	maxEnvNum     = 10000
)

// CgroupVersion is the cgroups mode of the host system
type CgroupVersion int

// GetConnection return the grpc connection
func GetConnection(endPoint string) (*grpc.ClientConn, error) {
	if endPoint == "" {
		return nil, fmt.Errorf("endpoint is not set")
	}
	logger.Debugf("connect using endpoint '%s' with '%s' timeout",
		utils.MaskPrefix(strings.TrimPrefix(endPoint, unixPrefix+"://")), defaultTimeout)
	addr, dialer, err := getAddressAndDialer(endPoint)
	if err != nil {
		hwlog.RunLog.Error(err)
		return nil, err
	}
	ctx, cancelFn := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancelFn()
	conn, err := grpc.DialContext(ctx, addr, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithContextDialer(dialer))
	if err != nil {
		return nil, err
	}
	logger.Debugf("connected successfully using endpoint: %s",
		utils.MaskPrefix(strings.TrimPrefix(endPoint, unixPrefix+"://")))
	return conn, nil
}

func parseSocketEndpoint(endpoint string) (string, string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", "", err
	}

	switch u.Scheme {
	case "unix":
		return "unix", u.Path, nil
	case "tcp":
		return "tcp", u.Host, nil
	default:
		return u.Scheme, "", fmt.Errorf("protocol %q not supported", u.Scheme)
	}
}

// getAddressAndDialer returns the address parsed from the given socket endpoint and  dialer
func getAddressAndDialer(endpoint string) (string, func(ctx context.Context, addr string) (net.Conn, error), error) {
	prefix, addr, err := parseSocketEndpoint(endpoint)
	if err != nil {
		return "", nil, err
	}
	if prefix != unixPrefix {
		return "", nil, fmt.Errorf("only support unix socket")
	}
	return addr, dial, nil
}

// dial  return the context dialer
func dial(ctx context.Context, addr string) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, unixPrefix, addr)
}

func validDNSRe(dnsContent string) error {
	if len(dnsContent) < MinLenDNS || len(dnsContent) > MaxLenDNS {
		return errors.New("param len invalid")
	}
	return nil
}

func makeUpDeviceInfo(c *CommonContainer) (DevicesInfo, error) {
	deviceInfo := DevicesInfo{}
	var names []string

	ns := c.Labels[labelK8sPodNamespace]
	names = append(names, ns)
	podName := c.Labels[labelK8sPodName]
	names = append(names, podName)
	containerName := c.Labels[labelContainerName]
	names = append(names, containerName)
	for _, v := range names {
		if err := validDNSRe(v); err != nil {
			return DevicesInfo{}, err
		}
	}

	deviceInfo.ID = c.Id
	deviceInfo.Name = ns + "_" + podName + "_" + containerName
	return deviceInfo, nil
}
