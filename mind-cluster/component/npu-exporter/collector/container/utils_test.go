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

// package container test methods in utils
package container

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/smartystreets/goconvey/convey"
	"google.golang.org/grpc"

	"ascend-common/common-utils/hwlog"
	"huawei.com/npu-exporter/v6/utils/logger"
)

const (
	testContainerID          = "container123"
	testPodNamespace         = "default"
	testPodName              = "test-pod"
	testContainerName        = "test-container"
	testUnixSocket           = "unix:///test.sock"
	testInvalidEndpoint      = "invalid://endpoint"
	testDialError            = "dial error"
	testGrpcDialError        = "grpc dial error"
	testInvalidEndpointError = "invalid endpoint"
	testEndpointNotSetError  = "endpoint is not set"
	testDNSContent           = "test-dns"
	testMinDNSContent        = "a"
	testEmptyDNSContent      = ""
	testTarget               = "test"
	testUnixScheme           = "unix"
	testTcpScheme            = "tcp"
	testUnixAddr             = "/tmp/test.sock"
	testTcpAddr              = "localhost:8080"
	testInvalidURL           = "://invalid"
	testEmptyNamespace       = ""
	testEmptyPodName         = ""
	testEmptyContainerName   = ""
)

func init() {
	logger.HwLogConfig = &hwlog.LogConfig{
		OnlyToStdout: true,
	}
	logger.InitLogger("Prometheus")
}

func TestGetConnection(t *testing.T) {
	convey.Convey("TestGetConnection", t, func() {
		convey.Convey("should return error when endpoint is empty", func() {
			testEmptyEndpoint()
		})
		convey.Convey("should return error when endpoint is invalid", func() {
			testInvalidEndpointFunc()
		})
		convey.Convey("should return error when grpc dial context fails", func() {
			testGrpcDialErrorFunc()
		})
		convey.Convey("should return connection when successful", func() {
			testSuccessfulConnection()
		})
	})
}

func testEmptyEndpoint() {
	conn, err := GetConnection("")
	convey.So(conn, convey.ShouldBeNil)
	convey.So(err, convey.ShouldNotBeNil)
	convey.So(err.Error(), convey.ShouldContainSubstring, testEndpointNotSetError)
}

func testInvalidEndpointFunc() {
	patches := gomonkey.ApplyFuncReturn(getAddressAndDialer, "", nil, errors.New(testInvalidEndpointError))
	defer patches.Reset()
	conn, err := GetConnection(testInvalidEndpoint)
	convey.So(conn, convey.ShouldBeNil)
	convey.So(err, convey.ShouldNotBeNil)
	convey.So(err.Error(), convey.ShouldContainSubstring, testInvalidEndpointError)
}

func testGrpcDialErrorFunc() {
	patches := gomonkey.ApplyFunc(getAddressAndDialer,
		func(endpoint string) (string, func(ctx context.Context, addr string) (net.Conn, error), error) {
			return testTarget, func(ctx context.Context, addr string) (net.Conn, error) {
				return nil, errors.New(testDialError)
			}, nil
		})
	defer patches.Reset()
	patches.ApplyFuncReturn(grpc.DialContext, nil, errors.New(testGrpcDialError))
	conn, err := GetConnection(testUnixSocket)
	convey.So(conn, convey.ShouldBeNil)
	convey.So(err, convey.ShouldNotBeNil)
	convey.So(err.Error(), convey.ShouldContainSubstring, testGrpcDialError)
}

func testSuccessfulConnection() {
	mockConn := &grpc.ClientConn{}
	patches := gomonkey.ApplyFunc(getAddressAndDialer,
		func(endpoint string) (string, func(ctx context.Context, addr string) (net.Conn, error), error) {
			return testTarget, func(ctx context.Context, addr string) (net.Conn, error) {
				return nil, nil
			}, nil
		})
	defer patches.Reset()
	patches.ApplyFuncReturn(grpc.DialContext, mockConn, nil)
	conn, err := GetConnection(testUnixSocket)
	convey.So(conn, convey.ShouldEqual, mockConn)
	convey.So(err, convey.ShouldBeNil)
}

func TestParseSocketEndpoint(t *testing.T) {
	testCases := []struct {
		name           string
		endpoint       string
		expectedScheme string
		expectedAddr   string
		expectedError  bool
	}{
		{name: "should parse unix endpoint when valid", endpoint: "unix:///tmp/test.sock",
			expectedScheme: testUnixScheme, expectedAddr: testUnixAddr, expectedError: false},
		{name: "should parse tcp endpoint when valid", endpoint: "tcp://localhost:8080",
			expectedScheme: testTcpScheme, expectedAddr: testTcpAddr, expectedError: false},
		{name: "should return error when scheme is invalid", endpoint: "http://localhost:8080",
			expectedScheme: "http", expectedAddr: "", expectedError: true},
		{name: "should return error when url is invalid", endpoint: testInvalidURL,
			expectedScheme: "", expectedAddr: "", expectedError: true},
	}

	for _, tc := range testCases {
		convey.Convey(tc.name, t, func() {
			scheme, addr, err := parseSocketEndpoint(tc.endpoint)
			convey.So(scheme, convey.ShouldEqual, tc.expectedScheme)
			convey.So(addr, convey.ShouldEqual, tc.expectedAddr)
			if tc.expectedError {
				convey.So(err, convey.ShouldNotBeNil)
			} else {
				convey.So(err, convey.ShouldBeNil)
			}
		})
	}
}

func TestGetAddressAndDialer(t *testing.T) {
	convey.Convey("TestGetAddressAndDialer", t, func() {
		testCases := []struct {
			name          string
			endpoint      string
			expectedAddr  string
			expectedError bool
		}{
			{
				name:          "should return address when unix endpoint is valid",
				endpoint:      "unix:///tmp/test.sock",
				expectedAddr:  "/tmp/test.sock",
				expectedError: false,
			},
			{
				name:          "should return error when scheme is invalid",
				endpoint:      "tcp://localhost:8080",
				expectedAddr:  "",
				expectedError: true,
			},
			{
				name:          "should return error when parse fails",
				endpoint:      "://invalid",
				expectedAddr:  "",
				expectedError: true,
			},
		}

		for _, tc := range testCases {
			convey.Convey(tc.name, func() {
				addr, dialer, err := getAddressAndDialer(tc.endpoint)
				convey.So(addr, convey.ShouldEqual, tc.expectedAddr)
				if tc.expectedError {
					convey.So(dialer, convey.ShouldBeNil)
					convey.So(err, convey.ShouldNotBeNil)
				} else {
					convey.So(dialer, convey.ShouldNotBeNil)
					convey.So(err, convey.ShouldBeNil)
				}
			})
		}
	})
}

func TestDial(t *testing.T) {
	convey.Convey("should call net.Dialer.DialContext when dialing", t, func() {
		var dialerCalled bool
		patches := gomonkey.ApplyMethod(&net.Dialer{}, "DialContext",
			func(d *net.Dialer, ctx context.Context, network, address string) (net.Conn, error) {
				dialerCalled = true
				return nil, errors.New("mock dial error")
			})
		defer patches.Reset()
		ctx := context.Background()
		conn, err := dial(ctx, "/tmp/test.sock")
		convey.So(conn, convey.ShouldBeNil)
		convey.So(err, convey.ShouldNotBeNil)
		convey.So(dialerCalled, convey.ShouldBeTrue)
	})
}

func TestValidDNSRe(t *testing.T) {
	convey.Convey("TestValidDNSRe", t, func() {
		testCases := []struct {
			name          string
			dnsContent    string
			expectedError bool
		}{
			{name: "should pass validation when dns content has valid length",
				dnsContent: testDNSContent, expectedError: false},
			{name: "should return error when dns content is empty",
				dnsContent: testEmptyDNSContent, expectedError: true},
			{name: "should return error when dns content is too long",
				dnsContent: string(make([]byte, MaxLenDNS+1)), expectedError: true},
			{name: "should pass validation when dns content has minimum valid length",
				dnsContent: testMinDNSContent, expectedError: false},
			{name: "should pass validation when dns content has maximum valid length",
				dnsContent: string(make([]byte, MaxLenDNS)), expectedError: false},
		}

		for _, tc := range testCases {
			convey.Convey(tc.name, func() {
				err := validDNSRe(tc.dnsContent)
				if tc.expectedError {
					convey.So(err, convey.ShouldNotBeNil)
					convey.So(err.Error(), convey.ShouldContainSubstring, "param len invalid")
				} else {
					convey.So(err, convey.ShouldBeNil)
				}
			})
		}
	})
}

func TestMakeUpDeviceInfo(t *testing.T) {
	testCases := getMakeUpDeviceInfoTestCases()
	for _, tc := range testCases {
		convey.Convey(tc.name, t, func() {
			deviceInfo, err := makeUpDeviceInfo(tc.container)
			validateMakeUpDeviceInfoResult(deviceInfo, err, tc)
		})
	}
}

func getMakeUpDeviceInfoTestCases() []struct {
	name          string
	container     *CommonContainer
	expectedError bool
	expectedName  string
} {
	return []struct {
		name          string
		container     *CommonContainer
		expectedError bool
		expectedName  string
	}{
		{name: "should return valid device info when container has all labels",
			container: createValidContainer(), expectedError: false, expectedName: "default_test-pod_test-container"},
		{name: "should return error when container has invalid namespace length",
			container: createContainerWithEmptyNamespace(), expectedError: true, expectedName: ""},
		{name: "should return error when container has invalid pod name length",
			container: createContainerWithEmptyPodName(), expectedError: true, expectedName: ""},
		{name: "should return error when container has invalid container name length",
			container: createContainerWithEmptyContainerName(), expectedError: true, expectedName: ""},
		{name: "should return error when container has too long namespace",
			container: createContainerWithLongNamespace(), expectedError: true, expectedName: ""},
	}
}

func createValidContainer() *CommonContainer {
	return &CommonContainer{Id: testContainerID, Labels: map[string]string{
		labelK8sPodNamespace: testPodNamespace, labelK8sPodName: testPodName,
		labelContainerName: testContainerName}}
}
func createContainerWithEmptyNamespace() *CommonContainer {
	return &CommonContainer{Id: testContainerID, Labels: map[string]string{
		labelK8sPodNamespace: testEmptyNamespace, labelK8sPodName: testPodName,
		labelContainerName: testContainerName}}
}
func createContainerWithEmptyPodName() *CommonContainer {
	return &CommonContainer{Id: testContainerID, Labels: map[string]string{
		labelK8sPodNamespace: testPodNamespace, labelK8sPodName: testEmptyPodName,
		labelContainerName: testContainerName}}
}
func createContainerWithEmptyContainerName() *CommonContainer {
	return &CommonContainer{Id: testContainerID, Labels: map[string]string{
		labelK8sPodNamespace: testPodNamespace, labelK8sPodName: testPodName,
		labelContainerName: testEmptyContainerName}}
}

func createContainerWithLongNamespace() *CommonContainer {
	return &CommonContainer{Id: testContainerID, Labels: map[string]string{
		labelK8sPodNamespace: string(make([]byte, MaxLenDNS+1)),
		labelK8sPodName:      testPodName, labelContainerName: testContainerName}}
}

func validateMakeUpDeviceInfoResult(deviceInfo DevicesInfo, err error, tc struct {
	name          string
	container     *CommonContainer
	expectedError bool
	expectedName  string
}) {
	if tc.expectedError {
		convey.So(err, convey.ShouldNotBeNil)
		convey.So(deviceInfo, convey.ShouldResemble, DevicesInfo{})
	} else {
		convey.So(err, convey.ShouldBeNil)
		convey.So(deviceInfo.ID, convey.ShouldEqual, tc.container.Id)
		convey.So(deviceInfo.Name, convey.ShouldEqual, tc.expectedName)
	}
}
