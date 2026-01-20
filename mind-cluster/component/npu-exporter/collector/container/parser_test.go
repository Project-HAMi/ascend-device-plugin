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
	"os"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/smartystreets/goconvey/convey"

	"ascend-common/common-utils/utils"
	"huawei.com/npu-exporter/v6/collector/container/isula"
	"huawei.com/npu-exporter/v6/collector/container/v1"
	"huawei.com/npu-exporter/v6/utils/logger"
)

const (
	// Test endpoint constants
	testContainerdEndpoint = "unix:///run/containerd.sock"
	testDockerEndpoint     = "unix:///run/docker.sock"

	device0              = 0
	device1              = 1
	device2              = 2
	device3              = 3
	testDeviceRange      = "0-2"
	testDeviceComma      = "0,1,2"
	testDeviceCommaRange = "0-1,2-3"
	testAscendDevice0    = "Ascend-0"
	testAscendDevices    = "Ascend-0,Ascend-1"
	testMixedDevices     = "0-1,3"

	// Test error constants
	testOriginalError  = "original error"
	testErrorMessage   = "test message"
	testContactedError = "original error->test message"

	// Test path constants
	testDevicePattern = "/dev/npu([0-9]+)"

	// Test duration constants
	testZeroDuration = 0
)

func TestMakeDevicesParser(t *testing.T) {
	testCases := []struct {
		name     string
		opts     CntNpuMonitorOpts
		expected *DevicesParser
	}{
		{name: "should create parser when options are valid for containerd",
			opts: CntNpuMonitorOpts{CriEndpoint: testContainerdEndpoint, EndpointType: EndpointTypeContainerd,
				OciEndpoint: testContainerdEndpoint, UseOciBackup: false, UseCriBackup: false},
			expected: &DevicesParser{RuntimeOperator: &RuntimeOperatorTool{UseOciBackup: false, UseCriBackup: false,
				CriEndpoint: testContainerdEndpoint, OciEndpoint: testContainerdEndpoint}, Timeout: testZeroDuration}},
		{name: "should create parser when options are valid for docker",
			opts: CntNpuMonitorOpts{CriEndpoint: testDockerEndpoint, EndpointType: EndpointTypeDockerd,
				OciEndpoint: testDockerEndpoint, UseOciBackup: true, UseCriBackup: false},
			expected: &DevicesParser{RuntimeOperator: &RuntimeOperatorTool{UseOciBackup: true, UseCriBackup: true,
				CriEndpoint: testDockerEndpoint, OciEndpoint: testDockerEndpoint}, Timeout: testZeroDuration}},
		{name: "should create parser when options are valid for isula",
			opts: CntNpuMonitorOpts{CriEndpoint: testContainerdEndpoint, EndpointType: EndpointTypeIsula,
				OciEndpoint: testContainerdEndpoint, UseOciBackup: true, UseCriBackup: true},
			expected: &DevicesParser{RuntimeOperator: &RuntimeOperatorTool{UseOciBackup: true, UseCriBackup: true,
				CriEndpoint: testContainerdEndpoint, OciEndpoint: testContainerdEndpoint}, Timeout: testZeroDuration}},
	}

	for _, tc := range testCases {
		convey.Convey(tc.name, t, func() {
			result := MakeDevicesParser(tc.opts)
			convey.So(result, convey.ShouldNotBeNil)
			convey.So(result.RuntimeOperator, convey.ShouldNotBeNil)
			convey.So(result.Timeout, convey.ShouldEqual, tc.expected.Timeout)
		})
	}
}

func TestDevicesParserInit(t *testing.T) {
	convey.Convey("TestDevicesParserInit", t, func() {
		convey.Convey("should initialize successfully when runtime operator init succeeds", func() {
			dp := &DevicesParser{
				RuntimeOperator: &RuntimeOperatorTool{},
			}

			patches := gomonkey.ApplyMethodReturn(dp.RuntimeOperator, "Init", nil)
			defer patches.Reset()

			err := dp.Init()
			convey.So(err, convey.ShouldBeNil)
		})

		convey.Convey("should return error when initialization fails", func() {
			dp := &DevicesParser{
				RuntimeOperator: &RuntimeOperatorTool{},
			}
			patches := gomonkey.ApplyMethodReturn(dp.RuntimeOperator, "Init", errors.New("init failed"))
			defer patches.Reset()
			err := dp.Init()
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "init failed")
		})
	})
}

func TestDevicesParserRecvResult(t *testing.T) {
	convey.Convey("TestDevicesParserRecvResult", t, func() {
		convey.Convey("should return result channel when initialized", func() {
			dp := &DevicesParser{
				result: make(chan DevicesInfos, 1),
			}
			resultChan := dp.RecvResult()
			convey.So(resultChan, convey.ShouldNotBeNil)
		})
	})
}

func TestDevicesParserRecvErr(t *testing.T) {
	convey.Convey("TestDevicesParserRecvErr", t, func() {
		convey.Convey("should return error channel when initialized", func() {
			dp := &DevicesParser{
				err: make(chan error, 1),
			}
			errChan := dp.RecvErr()
			convey.So(errChan, convey.ShouldNotBeNil)
		})
	})
}

func TestDevicesParserClose(t *testing.T) {
	convey.Convey("TestDevicesParserClose", t, func() {
		convey.Convey("should close runtime operator when called", func() {
			mockOperator := &RuntimeOperatorTool{}
			dp := &DevicesParser{
				RuntimeOperator: mockOperator,
			}

			visited := false
			patches := gomonkey.ApplyMethod(mockOperator, "Close", func(*RuntimeOperatorTool) error {
				visited = true
				return nil
			})
			defer patches.Reset()

			dp.Close()
			convey.So(visited, convey.ShouldBeTrue)
		})
	})
}

func TestDevicesParserParseDevices(t *testing.T) {
	convey.Convey("TestDevicesParserParseDevices", t, func() {
		convey.Convey("should parse isula devices when container type is isula", func() {
			dp := &DevicesParser{}
			mockOperator := &RuntimeOperatorTool{}
			dp.RuntimeOperator = mockOperator

			patches := gomonkey.ApplyMethodReturn(mockOperator, "GetContainerType", IsulaContainer).
				ApplyFuncReturn((*DevicesParser).parseDeviceInIsula, nil)
			defer patches.Reset()

			ctx := context.Background()
			container := &CommonContainer{Id: "test-container"}
			resultChan := make(chan DevicesInfo, 1)
			err := dp.parseDevices(ctx, container, resultChan)
			convey.So(err, convey.ShouldBeNil)
		})

		convey.Convey("should parse containerd devices when container type is not isula", func() {
			dp := &DevicesParser{}
			mockOperator := &RuntimeOperatorTool{}
			dp.RuntimeOperator = mockOperator

			patches := gomonkey.ApplyMethodReturn(mockOperator, "GetContainerType", DefaultContainer).
				ApplyFuncReturn((*DevicesParser).parseDevicesInContainerd, nil)
			defer patches.Reset()

			ctx := context.Background()
			container := &CommonContainer{Id: "test-container"}
			resultChan := make(chan DevicesInfo, 1)
			err := dp.parseDevices(ctx, container, resultChan)
			convey.So(err, convey.ShouldBeNil)
		})
	})
}

func TestDevicesParserParseDevicesInContainerd(t *testing.T) {
	convey.Convey("TestDevicesParserParseDevicesInContainerd", t, func() {
		convey.Convey("should return error when result channel is nil", func() {
			dp := &DevicesParser{}
			ctx := context.Background()
			container := &CommonContainer{Id: "test-container"}

			err := dp.parseDevicesInContainerd(ctx, container, nil)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "empty result channel")
		})

		convey.Convey("should return error when get container info fails", func() {
			dp := &DevicesParser{}
			mockOperator := &RuntimeOperatorTool{}
			dp.RuntimeOperator = mockOperator

			patches := gomonkey.ApplyMethod(mockOperator, "GetContainerInfoByID",
				func(*RuntimeOperatorTool, context.Context, string) (v1.Spec, error) {
					return v1.Spec{}, errors.New("get container info failed")
				})
			defer patches.Reset()

			ctx := context.Background()
			container := &CommonContainer{Id: "test-container"}
			resultChan := make(chan DevicesInfo, 1)

			err := dp.parseDevicesInContainerd(ctx, container, resultChan)
			convey.So(err, convey.ShouldNotBeNil)
		})
	})
}

func TestDevicesParserGetDevicesWithoutAscendRuntime(t *testing.T) {
	convey.Convey("TestDevicesParserGetDevicesWithoutAscendRuntime", t, func() {
		convey.Convey("should return devices when filter succeeds", func() {
			dp := &DevicesParser{}

			patches := gomonkey.ApplyFuncReturn(filterNPUDevices, []int{device0, device1, device2}, nil)
			defer patches.Reset()

			patches.ApplyFuncReturn(makeUpDeviceInfo, DevicesInfo{ID: "test", Name: "test-name"}, nil)

			spec := v1.Spec{}
			container := &CommonContainer{Id: "test-container"}

			result, err := dp.getDevicesWithoutAscendRuntime(spec, container)
			convey.So(err, convey.ShouldBeNil)
			convey.So(result.Devices, convey.ShouldResemble, []int{device0, device1, device2})
		})

		convey.Convey("should return empty when filter fails", func() {
			dp := &DevicesParser{}

			patches := gomonkey.ApplyFuncReturn(filterNPUDevices, nil, errors.New("filter failed"))
			defer patches.Reset()

			spec := v1.Spec{}
			container := &CommonContainer{Id: "test-container"}

			result, err := dp.getDevicesWithoutAscendRuntime(spec, container)
			convey.So(err, convey.ShouldBeNil)
			convey.So(result, convey.ShouldResemble, DevicesInfo{})
		})
	})
}

func TestDevicesParserGetDevicesWithAscendRuntime(t *testing.T) {
	convey.Convey("TestDevicesParserGetDevicesWithAscendRuntime", t, func() {
		convey.Convey("should return error when env format is invalid", func() {
			dp := &DevicesParser{}
			ascendDevEnv := "invalid-env"
			container := &CommonContainer{Id: "test-container"}

			result, err := dp.getDevicesWithAscendRuntime(ascendDevEnv, container)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(result, convey.ShouldResemble, DevicesInfo{})
		})

		convey.Convey("should return devices when env format is valid", func() {
			dp := &DevicesParser{}
			ascendDevEnv := "ASCEND_VISIBLE_DEVICES=0,1,2"
			container := &CommonContainer{Id: "test-container"}

			patches := gomonkey.ApplyFunc(makeUpDeviceInfo, func(*CommonContainer) (DevicesInfo, error) {
				return DevicesInfo{ID: "test", Name: "test-name"}, nil
			})
			defer patches.Reset()

			result, err := dp.getDevicesWithAscendRuntime(ascendDevEnv, container)
			convey.So(err, convey.ShouldBeNil)
			convey.So(result.Devices, convey.ShouldResemble, []int{device0, device1, device2})
		})
	})
}

func TestDevicesParserGetDevWithoutAscendRuntimeInIsula(t *testing.T) {
	convey.Convey("TestDevicesParserGetDevWithoutAscendRuntimeInIsula", t, func() {
		convey.Convey("should return devices when filter succeeds", func() {
			dp := &DevicesParser{}
			containerInfo := isula.ContainerJson{}
			container := &CommonContainer{Id: "test-container"}

			patches := gomonkey.ApplyFuncReturn(filterNPUDevicesInIsula, []int{device0, device1, device2}, nil)
			defer patches.Reset()

			patches.ApplyFuncReturn(makeUpDeviceInfo, DevicesInfo{ID: "test", Name: "test-name"}, nil)

			result, err := dp.getDevWithoutAscendRuntimeInIsula(containerInfo, container)
			convey.So(err, convey.ShouldBeNil)
			convey.So(result.Devices, convey.ShouldResemble, []int{device0, device1, device2})
		})

		convey.Convey("should return empty when filter fails", func() {
			dp := &DevicesParser{}
			containerInfo := isula.ContainerJson{}
			container := &CommonContainer{Id: "test-container"}

			patches := gomonkey.ApplyFuncReturn(filterNPUDevicesInIsula, nil, errors.New("filter failed"))
			defer patches.Reset()

			result, err := dp.getDevWithoutAscendRuntimeInIsula(containerInfo, container)
			convey.So(err, convey.ShouldBeNil)
			convey.So(result, convey.ShouldResemble, DevicesInfo{})
		})
	})
}

func TestDevicesParserParseDeviceInIsula(t *testing.T) {
	convey.Convey("TestDevicesParserParseDeviceInIsula", t, func() {
		convey.Convey("should return error when result channel is nil", func() {
			dp := &DevicesParser{}
			ctx := context.Background()
			container := &CommonContainer{Id: "test-container"}

			err := dp.parseDeviceInIsula(ctx, container, nil)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "empty result channel")
		})

		convey.Convey("should return error when container id is too long", func() {
			dp := &DevicesParser{}
			ctx := context.Background()
			longId := string(make([]byte, maxCgroupPath+1))
			container := &CommonContainer{Id: longId}
			resultChan := make(chan DevicesInfo, 1)

			err := dp.parseDeviceInIsula(ctx, container, resultChan)
			convey.So(err, convey.ShouldNotBeNil)
		})
	})
}

func TestDevicesParserCollect(t *testing.T) {
	convey.Convey("TestDevicesParserCollect", t, func() {
		convey.Convey("should return error when receiving channel is nil", func() {
			dp := &DevicesParser{}
			ctx := context.Background()

			result, err := dp.collect(ctx, nil, 1)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "receiving channel is empty")
			convey.So(result, convey.ShouldBeNil)
		})

		convey.Convey("should return nil when count is negative", func() {
			dp := &DevicesParser{}
			ctx := context.Background()
			resultChan := make(chan DevicesInfo)

			result, err := dp.collect(ctx, resultChan, -1)
			convey.So(err, convey.ShouldBeNil)
			convey.So(result, convey.ShouldBeNil)
		})
	})
}

func TestDevicesParserDoParse(t *testing.T) {
	convey.Convey("TestDevicesParserDoParse", t, func() {
		const time100ms = 100 * time.Millisecond
		convey.Convey("should handle error when get containers fails", func() {
			dp := &DevicesParser{
				err: make(chan error, 1),
			}
			mockOperator := &RuntimeOperatorTool{}
			dp.RuntimeOperator = mockOperator

			patches := gomonkey.ApplyMethod(mockOperator, "GetContainers",
				func(*RuntimeOperatorTool, context.Context) ([]*CommonContainer, error) {
					return nil, errors.New("get containers failed")
				})
			defer patches.Reset()

			resultChan := make(chan DevicesInfos, 1)
			dp.doParse(resultChan)

			select {
			case err := <-dp.err:
				convey.So(err, convey.ShouldNotBeNil)
			case <-time.After(time100ms):
				convey.So("timeout", convey.ShouldEqual, "should receive error")
			}
		})
	})
}

func TestDevicesParserFetchAndParse(t *testing.T) {
	const time10ms = 10 * time.Millisecond
	convey.Convey("TestDevicesParserFetchAndParse", t, func() {
		convey.Convey("should return early when err channel is nil", func() {
			dp := &DevicesParser{
				err: nil,
			}
			visited := make(chan bool, 1)
			patches := gomonkey.ApplyPrivateMethod(dp, "doParse",
				func(*DevicesParser, chan<- DevicesInfos) error {
					visited <- true
					return nil
				})
			defer patches.Reset()

			dp.FetchAndParse(nil)
			time.Sleep(time10ms)
			convey.So(len(visited), convey.ShouldEqual, 0)
		})

		convey.Convey("should start parsing when initialized", func() {
			dp := &DevicesParser{
				err:             make(chan error, 1),
				RuntimeOperator: &RuntimeOperatorTool{},
			}
			visited := make(chan bool, 1)
			patches := gomonkey.ApplyPrivateMethod(dp, "doParse",
				func(*DevicesParser, chan<- DevicesInfos) error {
					visited <- true
					return nil
				})
			defer patches.Reset()

			dp.FetchAndParse(nil)
			time.Sleep(time10ms)
			convey.So(len(visited), convey.ShouldEqual, 1)
		})
	})
}

func TestDevicesParserGetDeviceIDsByMinusStyle(t *testing.T) {
	convey.Convey("TestDevicesParserGetDeviceIDsByMinusStyle", t, func() {
		testCases := []struct {
			name     string
			devices  string
			expected []int
		}{
			{name: "should return empty slice when devices string is invalid", devices: "invalid-devices", expected: []int{}},
			{name: "should return empty slice when min device ID is invalid", devices: "invalid-5", expected: []int{}},
			{name: "should return empty slice when max device ID is invalid", devices: "0-invalid", expected: []int{}},
			{name: "should return empty slice when min ID is bigger than max ID", devices: "5-3", expected: []int{}},
			{name: "should return empty slice when max ID is too large", devices: "0-99999", expected: []int{}},
			{name: "should return device IDs when range is valid", devices: "0-2", expected: []int{0, 1, 2}},
			{name: "should return single device ID when min equals max", devices: "1-1", expected: []int{1}},
		}
		for _, tc := range testCases {
			convey.Convey(tc.name, func() {
				dp := &DevicesParser{}
				result := dp.getDeviceIDsByMinusStyle(tc.devices, "test-container")
				convey.So(result, convey.ShouldResemble, tc.expected)
			})
		}
	})
}

func TestGetNPUMajorID(t *testing.T) {
	testCases := builderTestGetNPUMajorIDCases()
	for _, tc := range testCases {
		convey.Convey(tc.name, t, func() {
			_, cleanup := tc.setup(t)
			defer cleanup()
			result, err := getNPUMajorID()
			if tc.hasError {
				convey.So(err, convey.ShouldNotBeNil)
			} else {
				convey.So(err, convey.ShouldBeNil)
			}
			convey.So(result, convey.ShouldResemble, tc.expected)
		})
	}
}

type TestGetNPUMajorIDCase struct {
	name     string
	setup    func(*testing.T) (*gomonkey.Patches, func())
	expected []string
	hasError bool
}

func builderTestGetNPUMajorIDCases() []TestGetNPUMajorIDCase {
	testCases := []TestGetNPUMajorIDCase{{name: "should return error when path check fails",
		setup: func(*testing.T) (*gomonkey.Patches, func()) {
			patches := gomonkey.ApplyFuncReturn(utils.CheckPath, "", errors.New("path check failed"))
			return patches, func() { patches.Reset() }
		}, expected: nil, hasError: true},
		{name: "should return error when file open fails",
			setup: func(*testing.T) (*gomonkey.Patches, func()) {
				p1 := gomonkey.ApplyFuncReturn(utils.CheckPath, "/proc/devices", nil)
				p1.ApplyFuncReturn(os.Open, nil, errors.New("file open failed"))
				return p1, func() { p1.Reset() }
			}, expected: []string{}, hasError: true},
		{name: "should return empty slice when no NPU devices found",
			setup: func(t *testing.T) (*gomonkey.Patches, func()) {
				tmpFile, clean, err := mkTemp("1 mem\n2 pty\n")
				if err != nil {
					t.Fatalf("failed to create temp file: %v", err)
				}
				p1 := gomonkey.ApplyFuncReturn(utils.CheckPath, tmpFile, nil)
				return p1, func() { clean(); p1.Reset() }
			}, expected: []string{}, hasError: false},
		{name: "should return major IDs when NPU devices found",
			setup: func(t *testing.T) (*gomonkey.Patches, func()) {
				tmpFile, clean, err := mkTemp("195 devdrv-cdev\n196 devdrv-cdev\n")
				if err != nil {
					t.Fatalf("failed to create temp file: %v", err)
				}
				p1 := gomonkey.ApplyFuncReturn(utils.CheckPath, tmpFile, nil)
				return p1, func() { clean(); p1.Reset() }
			}, expected: []string{"195", "196"}, hasError: false},
		{name: "should return major IDs when mixed devices found",
			setup: func(t *testing.T) (*gomonkey.Patches, func()) {
				tmpFile, clean, err := mkTemp("1 mem\n195 devdrv-cdev\n2 pty\n196 devdrv-cdev\n")
				if err != nil {
					t.Fatalf("failed to create temp file: %v", err)
				}
				p1 := gomonkey.ApplyFuncReturn(utils.CheckPath, tmpFile, nil)
				return p1, func() { clean(); p1.Reset() }
			}, expected: []string{"195", "196"}, hasError: false},
	}
	return testCases
}

func TestNpuMajor(t *testing.T) {
	convey.Convey("TestNpuMajor", t, func() {
		convey.Convey("should return cached major IDs", func() {
			patches := gomonkey.ApplyFuncReturn(getNPUMajorID, []string{"123", "456"}, nil)
			defer patches.Reset()

			result := npuMajor()
			convey.So(result, convey.ShouldNotBeNil)
		})
	})
}

func TestFilterNPUDevices(t *testing.T) {
	convey.Convey("TestFilterNPUDevices", t, func() {
		const mockMajorID = 236
		convey.Convey("should return error when spec is empty", func() {
			spec := v1.Spec{}
			result, err := filterNPUDevices(spec)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "empty spec info")
			convey.So(result, convey.ShouldBeNil)
		})

		convey.Convey("should return devices when spec is valid", func() {
			spec := v1.Spec{
				Linux: &v1.Linux{
					Resources: &v1.LinuxResources{
						Devices: []v1.LinuxDeviceCgroup{{Type: "c", Major: int64Ptr(mockMajorID), Minor: int64Ptr(0)}},
					},
				},
			}
			patches := gomonkey.ApplyFuncReturn(npuMajor, []string{"236"})
			defer patches.Reset()

			result, err := filterNPUDevices(spec)
			convey.So(err, convey.ShouldBeNil)
			convey.So(result, convey.ShouldNotBeNil)
		})
	})
}

// mkTemp creates a temporary file with the given content and returns the file name,
// a cleanup function, and an error. The file is closed before returning.
func mkTemp(content string) (string, func(), error) {
	f, err := os.CreateTemp("", "test_*")
	if err != nil {
		return "", func() {}, err
	}
	if _, err = f.WriteString(content); err != nil {
		clean(f)
		return "", func() {}, err
	}
	if _, err = f.Seek(0, 0); err != nil {
		clean(f)
		return "", func() {}, err
	}
	name := f.Name()
	return name, func() { clean(f) }, nil
}

func clean(f *os.File) {
	if f == nil {
		return
	}
	if err := f.Close(); err != nil {
		logger.Errorf("an error occurred where close file [%v],err :%v", f.Name(), err)
	}
	if err := os.Remove(f.Name()); err != nil {
		logger.Errorf("an error occurred where remove file [%v],err :%v", f.Name(), err)
	}
}

func TestFilterNPUDevicesInIsula(t *testing.T) {
	convey.Convey("TestFilterNPUDevicesInIsula", t, func() {
		convey.Convey("should return error when container is privileged", func() {
			containerInfo := isula.ContainerJson{
				HostConfig: &isula.HostConfig{
					Privileged: true,
				},
			}

			result, err := filterNPUDevicesInIsula(containerInfo)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "privileged container")
			convey.So(result, convey.ShouldBeNil)
		})

		convey.Convey("should return devices when container is not privileged", func() {
			containerInfo := isula.ContainerJson{
				HostConfig: &isula.HostConfig{
					Privileged: false,
					Devices: []isula.DeviceInfo{
						{
							PathInContainer: "/dev/npu0",
						},
					},
				},
			}

			patches := gomonkey.ApplyFuncReturn(getDevIdFromPath, 0, nil)
			defer patches.Reset()

			result, err := filterNPUDevicesInIsula(containerInfo)
			convey.So(err, convey.ShouldBeNil)
			convey.So(result, convey.ShouldNotBeNil)
		})
	})
}

// Helper function for creating int64 pointers
func int64Ptr(v int64) *int64 {
	return &v
}

func TestParseDiffEnvFmt(t *testing.T) {
	convey.Convey("TestParseDiffEnvFmt", t, func() {
		dp := &DevicesParser{}
		testCases := []struct {
			name        string
			devices     string
			containerID string
			expected    []int
		}{
			{name: "should parse comma style devices when valid",
				devices:     testDeviceComma,
				containerID: "test-container",
				expected:    []int{device0, device1, device2},
			},
			{name: "should parse minus style devices when valid",
				devices:     testDeviceRange,
				containerID: "test-container",
				expected:    []int{device0, device1, device2},
			},
			{name: "should parse ascend style devices when valid",
				devices:     testAscendDevices,
				containerID: "test-container",
				expected:    []int{device0, device1},
			},
			{name: "should parse comma minus style devices when valid",
				devices:     testDeviceCommaRange,
				containerID: "test-container",
				expected:    []int{device0, device1, device2, device3},
			},
			{name: "should return empty slice when devices are empty",
				devices:     "",
				containerID: "test-container",
				expected:    []int{},
			},
		}

		for _, tc := range testCases {
			convey.Convey(tc.name, func() {
				result := dp.parseDiffEnvFmt(tc.devices, tc.containerID)
				convey.So(result, convey.ShouldResemble, tc.expected)
			})
		}
	})
}

func TestGetDeviceIDsByCommaStyle(t *testing.T) {
	convey.Convey("TestGetDeviceIDsByCommaStyle", t, func() {
		dp := &DevicesParser{}
		testCases := []struct {
			name        string
			devices     string
			containerID string
			expected    []int
		}{
			{name: "should parse comma separated devices when valid",
				devices:     "0,1,2,3",
				containerID: "test-container",
				expected:    []int{device0, device1, device2, device3},
			},
			{name: "should parse single device when valid",
				devices:     "0",
				containerID: "test-container",
				expected:    []int{device0},
			},
			{name: "should return empty slice when devices are empty",
				devices:     "",
				containerID: "test-container",
				expected:    []int{},
			},
			{name: "should parse devices with spaces when valid",
				devices:     testDeviceComma,
				containerID: "test-container",
				expected:    []int{device0, device1, device2},
			},
		}

		for _, tc := range testCases {
			convey.Convey(tc.name, func() {
				result := dp.getDeviceIDsByCommaStyle(tc.devices, tc.containerID)
				convey.So(result, convey.ShouldResemble, tc.expected)
			})
		}
	})
}

func TestGetDeviceIDsByAscendStyle(t *testing.T) {
	convey.Convey("TestGetDeviceIDsByAscendStyle", t, func() {
		dp := &DevicesParser{}

		testCases := []struct {
			name        string
			devices     string
			containerID string
			expected    []int
		}{
			{
				name:        "should parse ascend devices when valid",
				devices:     "Ascend-0,Ascend-1,Ascend-2",
				containerID: "test-container",
				expected:    []int{device0, device1, device2},
			},
			{
				name:        "should parse single ascend device when valid",
				devices:     testAscendDevice0,
				containerID: "test-container",
				expected:    []int{0},
			},
			{
				name:        "should return empty slice when devices are empty",
				devices:     "",
				containerID: "test-container",
				expected:    []int{},
			},
			{
				name:        "should parse mixed case ascend devices when valid",
				devices:     "ascend-0,ASCEND-1",
				containerID: "test-container",
				expected:    []int{device0, device1},
			},
		}

		for _, tc := range testCases {
			convey.Convey(tc.name, func() {
				result := dp.getDeviceIDsByAscendStyle(tc.devices, tc.containerID)
				convey.So(result, convey.ShouldResemble, tc.expected)
			})
		}
	})
}

func TestGetDeviceIDsByMinusStyle(t *testing.T) {
	convey.Convey("TestGetDeviceIDsByMinusStyle", t, func() {
		dp := &DevicesParser{}

		testCases := []struct {
			name        string
			devices     string
			containerID string
			expected    []int
		}{
			{
				name:        "should parse range devices when valid",
				devices:     "0-3",
				containerID: "test-container",
				expected:    []int{device0, device1, device2, device3},
			},
			{
				name:        "should parse single device range when valid",
				devices:     "0-0",
				containerID: "test-container",
				expected:    []int{device0},
			},
			{
				name:        "should return empty slice when devices are empty",
				devices:     "",
				containerID: "test-container",
				expected:    []int{},
			},
		}

		for _, tc := range testCases {
			convey.Convey(tc.name, func() {
				result := dp.getDeviceIDsByMinusStyle(tc.devices, tc.containerID)
				convey.So(result, convey.ShouldResemble, tc.expected)
			})
		}
	})
}

func TestGetDeviceIDsByCommaMinusStyle(t *testing.T) {
	convey.Convey("TestGetDeviceIDsByCommaMinusStyle", t, func() {
		dp := &DevicesParser{}

		testCases := []struct {
			name        string
			devices     string
			containerID string
			expected    []int
		}{
			{
				name:        "should parse comma minus devices when valid",
				devices:     testDeviceCommaRange,
				containerID: "test-container",
				expected:    []int{device0, device1, device2, device3},
			},
			{
				name:        "should parse single range when valid",
				devices:     testDeviceRange,
				containerID: "test-container",
				expected:    []int{device0, device1, device2},
			},
			{
				name:        "should return nil when devices are empty",
				devices:     "",
				containerID: "test-container",
				expected:    nil,
			},
			{
				name:        "should parse mixed ranges when valid",
				devices:     testMixedDevices,
				containerID: "test-container",
				expected:    []int{device0, device1, device3},
			},
		}

		for _, tc := range testCases {
			convey.Convey(tc.name, func() {
				result := dp.getDeviceIDsByCommaMinusStyle(tc.devices, tc.containerID)
				convey.So(result, convey.ShouldResemble, tc.expected)
			})
		}
	})
}

func TestContains(t *testing.T) {
	convey.Convey("TestContains", t, func() {
		testCases := []struct {
			name     string
			slice    []string
			target   string
			expected bool
		}{
			{
				name:     "should return true when target exists in slice",
				slice:    []string{"a", "b", "c"},
				target:   "b",
				expected: true,
			},
			{
				name:     "should return false when target does not exist in slice",
				slice:    []string{"a", "b", "c"},
				target:   "d",
				expected: false,
			},
			{
				name:     "should return false when slice is empty",
				slice:    []string{},
				target:   "a",
				expected: false,
			},
			{
				name:     "should return false when slice is nil",
				slice:    nil,
				target:   "a",
				expected: false,
			},
			{
				name:     "should return false when target is empty string",
				slice:    []string{"a", "b", "c"},
				target:   "",
				expected: false,
			},
		}

		for _, tc := range testCases {
			convey.Convey(tc.name, func() {
				result := contains(tc.slice, tc.target)
				convey.So(result, convey.ShouldEqual, tc.expected)
			})
		}
	})
}

func TestContactError(t *testing.T) {
	convey.Convey("TestContactError", t, func() {
		testCases := []struct {
			name     string
			err      error
			msg      string
			expected string
		}{
			{
				name:     "should concatenate error with message when both provided",
				err:      errors.New(testOriginalError),
				msg:      testErrorMessage,
				expected: testContactedError,
			},
		}

		for _, tc := range testCases {
			convey.Convey(tc.name, func() {
				result := contactError(tc.err, tc.msg)
				convey.So(result.Error(), convey.ShouldEqual, tc.expected)
			})
		}
	})
}

func TestGetDevIdFromPath(t *testing.T) {
	convey.Convey("TestGetDevIdFromPath", t, func() {
		testCases := []struct {
			name     string
			pattern  string
			path     string
			expected int
			hasError bool
		}{
			{name: "should extract device id when path is valid",
				pattern:  testDevicePattern,
				path:     "/dev/npu0",
				expected: 0,
				hasError: false,
			},
			{name: "should extract device id when path has multiple digits",
				pattern:  testDevicePattern,
				path:     "/dev/npu123",
				expected: 123,
				hasError: false,
			},
			{name: "should return error when device path is invalid",
				pattern:  testDevicePattern,
				path:     "/dev/cpu0",
				expected: 0,
				hasError: true,
			},
			{name: "should return error when path is empty",
				pattern:  testDevicePattern,
				path:     "",
				expected: 0,
				hasError: true,
			},
		}

		for _, tc := range testCases {
			convey.Convey(tc.name, func() {
				result, err := getDevIdFromPath(tc.pattern, tc.path)
				if tc.hasError {
					convey.So(err, convey.ShouldNotBeNil)
				} else {
					convey.So(err, convey.ShouldBeNil)
					convey.So(result, convey.ShouldEqual, tc.expected)
				}
			})
		}
	})
}

func TestWithDefault(t *testing.T) {
	convey.Convey("TestWithDefault", t, func() {
		const time0s = 0
		const time3s = 3 * time.Second
		const time5s = 5 * time.Second
		testCases := []struct {
			name     string
			v        time.Duration
			d        time.Duration
			expected time.Duration
		}{
			{name: "should return default when duration is zero",
				v:        time0s,
				d:        time5s,
				expected: time5s,
			},
			{name: "should return value when duration is non-zero",
				v:        time3s,
				d:        time5s,
				expected: time3s,
			},
			{name: "should return value when duration is negative",
				v:        -1 * time.Second,
				d:        time5s,
				expected: -1 * time.Second,
			},
		}

		for _, tc := range testCases {
			convey.Convey(tc.name, func() {
				result := withDefault(tc.v, tc.d)
				convey.So(result, convey.ShouldEqual, tc.expected)
			})
		}
	})
}
