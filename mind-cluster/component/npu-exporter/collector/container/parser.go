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
	"bufio"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"ascend-common/api"
	"ascend-common/common-utils/hwlog"
	"ascend-common/common-utils/utils"
	"huawei.com/npu-exporter/v6/collector/container/isula"
	"huawei.com/npu-exporter/v6/collector/container/v1"
	"huawei.com/npu-exporter/v6/utils/logger"
)

const (
	namespaceMoby = "moby"   // Docker
	namespaceK8s  = "k8s.io" // CRI + Containerd
	sliceLen8     = 8
	ascendEnvPart = 2
	charDevice    = "c"

	minus                            = "-"
	comma                            = ","
	ascend                           = "Ascend"
	maxEnvLength                     = 1024
	parsingNpuDefaultTimeoutDuration = 3
)

const (
	// EndpointTypeContainerd K8S + Containerd
	EndpointTypeContainerd = iota
	// EndpointTypeDockerd Docker with or without K8S
	EndpointTypeDockerd
	// EndpointTypeIsula K8S + isula
	EndpointTypeIsula = 2
)

var (
	// ErrFromContext error is from the context
	ErrFromContext = errors.New("error from context")

	npuMajorID               []string
	npuMajorFetchCtrl        sync.Once
	parsingNpuDefaultTimeout = parsingNpuDefaultTimeoutDuration * time.Second
)

var (
	envErrDescribe = func(ctrID, devID, env string, err error) string {
		return fmt.Sprintf("container (%s) has an invalid device ID (%s) in %s, err is %v", ctrID, devID, env, err)
	}
	minusStyle = func(s string) bool {
		return strings.Contains(s, minus)
	}
	commaMinusStyle = func(s string) bool {
		return strings.Contains(s, minus) && strings.Contains(s, comma)
	}
	ascendStyle = func(s string) bool {
		return strings.Contains(s, ascend)
	}
)

// CntNpuMonitorOpts contains setting options for monitoring containers
type CntNpuMonitorOpts struct {
	EndpointType int    // containerd or docker
	CriEndpoint  string // CRI server address
	UseCriBackup bool   // whether try to use cri backup address
	OciEndpoint  string // OCI server, now is containerd address
	UseOciBackup bool   // whether try to use oci backup address
}

// MakeDevicesParser evaluates option settings and make an instance according to it
func MakeDevicesParser(opts CntNpuMonitorOpts) *DevicesParser {
	runtimeOperator := &RuntimeOperatorTool{
		UseCriBackup: opts.UseCriBackup,
		UseOciBackup: opts.UseOciBackup,
		CriEndpoint:  opts.CriEndpoint,
		OciEndpoint:  opts.OciEndpoint,
	}
	parser := &DevicesParser{
		RuntimeOperator: runtimeOperator,
	}

	switch opts.EndpointType {
	case EndpointTypeContainerd:
		runtimeOperator.Namespace = namespaceK8s
	case EndpointTypeDockerd:
		runtimeOperator.Namespace = namespaceMoby
	case EndpointTypeIsula:
		runtimeOperator.Namespace = namespaceK8s
	default:
		logger.Errorf("invalid type value %d", opts.EndpointType)
	}

	return parser
}

// DevicesInfo the container device information struct
type DevicesInfo struct {
	// container id
	ID string
	// container name, the format is: PodNameSpace_PodName_ContainerName
	Name    string
	Devices []int
}

// DevicesInfos the device information storage map
type DevicesInfos = map[string]DevicesInfo

// DevicesParser the parser which parse device info
type DevicesParser struct {
	// instances
	result chan DevicesInfos
	err    chan error
	// configuration
	RuntimeOperator RuntimeOperator
	Timeout         time.Duration
}

// Init initializes connection to containerd daemon and to CRI server or dockerd daemon based on name fetcher setting
func (dp *DevicesParser) Init() error {
	if err := dp.RuntimeOperator.Init(); err != nil {
		return contactError(err, "connecting to container runtime failed")
	}
	dp.result = make(chan DevicesInfos, 1)
	dp.err = make(chan error, 1)
	return nil
}

// RecvResult exposes the channel used for receiving devices info analyzing result
func (dp *DevicesParser) RecvResult() <-chan DevicesInfos {
	return dp.result
}

// RecvErr exposes the channel used for receiving errors occurred during analyzing
func (dp *DevicesParser) RecvErr() <-chan error {
	return dp.err
}

// Close closes all connections and channels established during initializing
func (dp *DevicesParser) Close() {
	_ = dp.RuntimeOperator.Close()
}

func (dp *DevicesParser) parseDevices(ctx context.Context, c *CommonContainer, rs chan<- DevicesInfo) error {
	if dp.RuntimeOperator.GetContainerType() == IsulaContainer {
		return dp.parseDeviceInIsula(ctx, c, rs)
	}

	return dp.parseDevicesInContainerd(ctx, c, rs)
}

func (dp *DevicesParser) parseDevicesInContainerd(ctx context.Context, c *CommonContainer,
	rs chan<- DevicesInfo) error {
	if rs == nil {
		return errors.New("empty result channel")
	}
	deviceInfo := DevicesInfo{}
	defer func(di *DevicesInfo) {
		rs <- *di
	}(&deviceInfo)

	spec, err := dp.RuntimeOperator.GetContainerInfoByID(ctx, c.Id)
	if err != nil {
		return contactError(err, fmt.Sprintf("cannot get container devices by container id (%s)", c.Id))
	}
	if spec.Linux == nil || spec.Linux.Resources == nil || len(spec.Linux.Resources.Devices) > maxDevicesNum {
		return contactError(errors.New("device error"),
			fmt.Sprintf("devices in container is too much (%v) or empty", maxDevicesNum))
	}
	if spec.Process == nil || len(spec.Process.Env) > maxEnvNum {
		return contactError(errors.New("env error"), fmt.Sprintf("env in container is too much (%v) or empty",
			maxEnvNum))
	}

	envs := spec.Process.Env
	for i := len(envs) - 1; i >= 0; i-- {
		e := envs[i]
		if strings.Contains(e, api.AscendDeviceInfo) {
			deviceInfo, err = dp.getDevicesWithAscendRuntime(e, c)
			return err
		}
	}

	deviceInfo, err = dp.getDevicesWithoutAscendRuntime(spec, c)
	return err
}

func (dp *DevicesParser) getDevicesWithoutAscendRuntime(spec v1.Spec, c *CommonContainer) (DevicesInfo, error) {
	deviceInfo := DevicesInfo{}
	devicesIDs, err := filterNPUDevices(spec)
	if err != nil {
		logger.Debugf("filter npu devices failed by container id (%s), err is %v", c.Id, err)
		return DevicesInfo{}, nil
	}
	logger.Debugf("filter npu devices %v in container (%s)", devicesIDs, c.Id)

	if len(devicesIDs) != 0 {
		if deviceInfo, err = makeUpDeviceInfo(c); err == nil {
			deviceInfo.Devices = devicesIDs
			return deviceInfo, nil
		} else {
			logger.Errorf("makeUpDeviceInfo failed: %s", err)
		}
		return DevicesInfo{}, err
	}

	return DevicesInfo{}, nil
}

func (dp *DevicesParser) getDevicesWithAscendRuntime(ascendDevEnv string, c *CommonContainer) (DevicesInfo, error) {
	logger.Debugf("get device info by env (%s) in %s", ascendDevEnv, c.Id)
	devInfo := strings.Split(ascendDevEnv, "=")
	if len(devInfo) != ascendEnvPart {
		return DevicesInfo{}, fmt.Errorf("an invalid %s env(%s)", api.AscendDeviceInfo, ascendDevEnv)
	}
	devicesIDs := dp.parseDiffEnvFmt(devInfo[1], c.Id)
	if len(devicesIDs) == 0 {
		return DevicesInfo{}, nil
	}

	deviceInfo, err := makeUpDeviceInfo(c)
	if err != nil {
		hwlog.RunLog.Error(err)
		return DevicesInfo{}, err
	}
	deviceInfo.Devices = devicesIDs
	return deviceInfo, nil
}

func (dp *DevicesParser) parseDiffEnvFmt(devices, containerID string) []int {
	if len(devices) > maxEnvLength {
		return []int{}
	}
	if ascendStyle(devices) {
		return dp.getDeviceIDsByAscendStyle(devices, containerID)
	}
	if commaMinusStyle(devices) {
		return dp.getDeviceIDsByCommaMinusStyle(devices, containerID)
	}
	if minusStyle(devices) {
		return dp.getDeviceIDsByMinusStyle(devices, containerID)
	}
	return dp.getDeviceIDsByCommaStyle(devices, containerID)
}

func (dp *DevicesParser) getDeviceIDsByCommaStyle(devices, containerID string) []int {
	devList := strings.Split(devices, comma)
	devicesIDs := make([]int, 0, len(devList))
	for _, devID := range devList {
		id, err := strconv.Atoi(devID)
		if err != nil {
			logger.Errorf("container (%s) has an invalid device ID (%v) in %s, error is %s", containerID,
				devID, api.AscendDeviceInfo, err)
			continue
		}
		devicesIDs = append(devicesIDs, id)
	}
	return devicesIDs
}

func (dp *DevicesParser) getDeviceIDsByAscendStyle(devices, containerID string) []int {
	devList := strings.Split(devices, comma)
	deviceIDs := make([]int, 0, len(devList))
	for _, subDevice := range devList {
		deviceName := strings.Split(subDevice, minus)
		if len(deviceName) != ascendEnvPart {
			logger.Errorf(envErrDescribe(containerID, "", api.AscendDeviceInfo, nil))
			continue
		}
		id, err := strconv.Atoi(deviceName[1])
		if err != nil {
			logger.Errorf(envErrDescribe(containerID, deviceName[1], api.AscendDeviceInfo, err))
			continue
		}
		deviceIDs = append(deviceIDs, id)
	}
	return deviceIDs
}

func (dp *DevicesParser) getDeviceIDsByMinusStyle(devices, containerID string) []int {
	deviceIDs := make([]int, 0)
	devIDRange := strings.Split(devices, minus)
	if len(devIDRange) != ascendEnvPart {
		logger.Errorf(envErrDescribe(containerID, "range", api.AscendDeviceInfo, nil))
		return deviceIDs
	}
	minDevID, err := strconv.Atoi(devIDRange[0])
	if err != nil {
		logger.Errorf(envErrDescribe(containerID, devIDRange[0], api.AscendDeviceInfo, err))
		return deviceIDs
	}
	maxDevID, err := strconv.Atoi(devIDRange[1])
	if err != nil {
		logger.Errorf(envErrDescribe(containerID, devIDRange[1], api.AscendDeviceInfo, err))
		return deviceIDs
	}
	if minDevID > maxDevID {
		logger.Errorf(envErrDescribe(containerID, "",
			api.AscendDeviceInfo, errors.New("min id bigger than max id")))
		return deviceIDs
	}
	if maxDevID > math.MaxInt16 {
		logger.Errorf(envErrDescribe(containerID, "", api.AscendDeviceInfo, errors.New("max id invalid")))
		return deviceIDs
	}
	for deviceID := minDevID; deviceID <= maxDevID; deviceID++ {
		deviceIDs = append(deviceIDs, deviceID)
	}
	return deviceIDs
}

func (dp *DevicesParser) getDeviceIDsByCommaMinusStyle(devices, containerID string) []int {
	var deviceIDs []int
	devList := strings.Split(devices, comma)
	for _, subDevices := range devList {
		if minusStyle(subDevices) {
			deviceIDs = append(deviceIDs, dp.getDeviceIDsByMinusStyle(subDevices, containerID)...)
			continue
		}
		deviceIDs = append(deviceIDs, dp.getDeviceIDsByCommaStyle(subDevices, containerID)...)
	}
	return deviceIDs
}

func (dp *DevicesParser) getDevWithoutAscendRuntimeInIsula(containerInfo isula.ContainerJson,
	c *CommonContainer) (DevicesInfo, error) {
	deviceInfo := DevicesInfo{}
	devicesIDs, err := filterNPUDevicesInIsula(containerInfo)
	if err != nil {
		logger.Debugf("filter npu devices failed by container id (%s), err is %v", c.Id, err)
		return DevicesInfo{}, nil
	}
	logger.Debugf("filter npu devices %v in container (%s)", devicesIDs, c.Id)

	if len(devicesIDs) == 0 {
		return DevicesInfo{}, nil
	}

	deviceInfo, err = makeUpDeviceInfo(c)
	if err != nil {
		hwlog.RunLog.Error(err)
		return DevicesInfo{}, err
	}
	deviceInfo.Devices = devicesIDs
	return deviceInfo, nil
}

func (dp *DevicesParser) parseDeviceInIsula(ctx context.Context, c *CommonContainer, rs chan<- DevicesInfo) error {
	if rs == nil {
		return errors.New("empty result channel")
	}

	deviceInfo := DevicesInfo{}
	defer func(di *DevicesInfo) {
		rs <- *di
	}(&deviceInfo)

	if len(c.Id) > maxCgroupPath {
		return fmt.Errorf("the containerId (%s) is too long", c.Id)
	}
	containerInfo, err := dp.RuntimeOperator.GetIsulaContainerInfoByID(ctx, c.Id)
	if err != nil {
		return contactError(err, fmt.Sprintf("getting config of container(%s) fail", c.Id))
	}
	if containerInfo.HostConfig == nil || containerInfo.Config == nil {
		return errors.New("empty container info")
	}

	envs := containerInfo.Config.Env
	for i := len(envs) - 1; i >= 0; i-- {
		e := envs[i]
		if strings.Contains(e, api.AscendDeviceInfo) {
			deviceInfo, err = dp.getDevicesWithAscendRuntime(e, c)
			return err
		}
	}

	deviceInfo, err = dp.getDevWithoutAscendRuntimeInIsula(containerInfo, c)
	return err
}

func (dp *DevicesParser) collect(ctx context.Context, r <-chan DevicesInfo, ct int32) (DevicesInfos, error) {
	if r == nil {
		return nil, errors.New("receiving channel is empty")
	}
	if ct < 0 {
		return nil, nil
	}

	results := make(map[string]DevicesInfo, ct)
	for {
		select {
		case info, ok := <-r:
			if !ok {
				return nil, nil
			}
			if info.ID != "" {
				results[info.ID] = info
			}
			if ct -= 1; ct <= 0 {
				return results, nil
			}
		case <-ctx.Done():
			hwlog.RunLog.Error("ctx is timeout")
			dp.err <- ErrFromContext
			return nil, nil
		}
	}
}

func (dp *DevicesParser) doParse(resultOut chan<- DevicesInfos) {
	var result DevicesInfos = nil
	defer func(rslt DevicesInfos) {
		if resultOut != nil {
			resultOut <- rslt
			close(resultOut)
		}
	}(result)

	ctx := context.Background()
	containers, err := dp.RuntimeOperator.GetContainers(ctx)
	if err != nil {
		dp.err <- err
		return
	}

	l := len(containers)
	if l == 0 || l > maxContainers {
		logger.Debugf("get %d containers from cri interface, return empty data", l)
		dp.result <- make(DevicesInfos)
		return
	}

	r := make(chan DevicesInfo)
	defer close(r)
	wg := sync.WaitGroup{}
	wg.Add(l)

	for _, container := range containers {
		go func(container *CommonContainer, c context.Context) {
			if err := dp.parseDevices(c, container, r); err != nil {
				dp.err <- err
			}
			wg.Done()
		}(container, ctx)
	}
	ctx, cancelFn := context.WithTimeout(ctx, withDefault(dp.Timeout, parsingNpuDefaultTimeout))
	defer cancelFn()
	result, err = dp.collect(ctx, r, int32(l))
	if err != nil {
		logger.Errorf("collect info error: %v", err)
	}

	if result != nil {
		dp.result <- result
	}
	wg.Wait()
}

// FetchAndParse triggers the asynchronous process of querying and analyzing all containers
// resultOut channel is for fetching the current result
func (dp *DevicesParser) FetchAndParse(resultOut chan<- DevicesInfos) {
	if dp.err == nil {
		logger.Debug("device paster is not initialized")
		return
	}
	go dp.doParse(resultOut)
}

func withDefault(v time.Duration, d time.Duration) time.Duration {
	if v == 0 {
		return d
	}

	return v
}

// query the MajorID of NPU devices
func getNPUMajorID() ([]string, error) {
	const (
		deviceCount   = 2
		maxSearchLine = 512
	)

	path, err := utils.CheckPath("/proc/devices")
	if err != nil {
		return nil, err
	}
	majorID := make([]string, 0, deviceCount)
	f, err := os.Open(path)
	if err != nil {
		return majorID, err
	}
	defer func() {
		err = f.Close()
		if err != nil {
			hwlog.RunLog.Error(err)
		}
	}()
	s := bufio.NewScanner(f)
	count := 0
	for s.Scan() {
		// prevent from searching too many lines
		if count > maxSearchLine {
			break
		}
		count++
		text := s.Text()
		matched, err := regexp.MatchString("^[0-9]{1,3}\\s[v]?devdrv-cdev$", text)
		if err != nil {
			return majorID, err
		}
		if !matched {
			continue
		}
		fields := strings.Fields(text)
		majorID = append(majorID, fields[0])
	}
	return majorID, nil
}

func npuMajor() []string {
	npuMajorFetchCtrl.Do(func() {
		var err error
		npuMajorID, err = getNPUMajorID()
		if err != nil {
			return
		}
	})
	return npuMajorID
}

func contains(slice []string, target string) bool {
	for _, v := range slice {
		if v == target {
			return true
		}
	}
	return false
}

func contactError(err error, msg string) error {
	return fmt.Errorf("%s->%s", err.Error(), msg)
}

func filterNPUDevices(spec v1.Spec) ([]int, error) {
	if spec.Linux == nil || spec.Linux.Resources == nil {
		return nil, errors.New("empty spec info")
	}

	const base = 10
	devIDs := make([]int, 0, sliceLen8)
	majorIDs := npuMajor()
	for _, dev := range spec.Linux.Resources.Devices {
		if dev.Minor == nil || dev.Major == nil {
			// do not monitor privileged container
			continue
		}
		if *dev.Minor > math.MaxInt32 {
			return nil, fmt.Errorf("get wrong device ID (%v)", dev.Minor)
		}
		major := strconv.FormatInt(*dev.Major, base)
		if dev.Type == charDevice && contains(majorIDs, major) {
			devIDs = append(devIDs, int(*dev.Minor))
		}
	}

	return devIDs, nil
}

// filterNPUDevicesInIsula get id of device from containerJson(containerInfo)
func filterNPUDevicesInIsula(containerInfo isula.ContainerJson) ([]int, error) {
	privileged := containerInfo.HostConfig.Privileged
	if privileged {
		return nil, errors.New("it's a privileged container and skip it")
	}

	devIDs := make([]int, 0, sliceLen8)
	devices := containerInfo.HostConfig.Devices
	for _, dev := range devices {
		Id, err := getDevIdFromPath(api.DevicePathPattern, dev.PathInContainer)
		if err != nil {
			logger.Warn(err)
			continue
		}
		devIDs = append(devIDs, Id)
	}

	return devIDs, nil
}

func getDevIdFromPath(pattern, path string) (int, error) {
	if match, err := regexp.MatchString(pattern, path); err != nil || !match {
		return -1, fmt.Errorf("unexpected path of device: %s or match error: %v", path, err)
	}
	number := regexp.MustCompile(`\d+`)
	IdStr := number.FindString(path)
	Id, err := strconv.Atoi(IdStr)
	if err != nil {
		return -1, fmt.Errorf("unexpected device ID (%v)", IdStr)
	}
	if Id > math.MaxInt32 {
		return -1, fmt.Errorf("get wrong device ID (%v)", Id)
	}
	return Id, nil
}
