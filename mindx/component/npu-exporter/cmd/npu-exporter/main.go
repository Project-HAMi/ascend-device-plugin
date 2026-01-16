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

// Package main
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/influxdata/telegraf/plugins/common/shim"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"ascend-common/api"
	"ascend-common/common-utils/hwlog"
	"ascend-common/common-utils/limiter"
	"ascend-common/devmanager"
	"ascend-common/devmanager/common"
	colcommon "huawei.com/npu-exporter/v6/collector/common"
	"huawei.com/npu-exporter/v6/collector/config"
	"huawei.com/npu-exporter/v6/collector/container"
	_ "huawei.com/npu-exporter/v6/platforms/inputs/npu"
	"huawei.com/npu-exporter/v6/platforms/prom"
	"huawei.com/npu-exporter/v6/plugins"
	"huawei.com/npu-exporter/v6/utils/logger"
	"huawei.com/npu-exporter/v6/versions"
)

var (
	port                int
	updateTime          int
	ip                  = ""
	version             bool
	concurrency         int
	containerMode       = ""
	containerd          = ""
	endpoint            = ""
	limitIPReq          = ""
	platform            = ""
	textMetricsFilePath = ""
	limitIPConn         int
	limitTotalConn      int
	cacheSize           int
	profilingTime       int
	hccsBWProfilingTime int
	pollInterval        time.Duration
	deviceResetTimeout  int
)

const (
	portConst               = 8082
	updateTimeConst         = 5
	cacheTime               = 100 * time.Second
	portLeft                = 1025
	portRight               = 40000
	oneMinute               = 60
	defaultConcurrency      = 5
	defaultLogFile          = "/var/log/mindx-dl/npu-exporter/npu-exporter.log"
	containerModeDocker     = "docker"
	containerModeContainerd = "containerd"
	containerModeIsula      = "isula"
	unixPre                 = "unix://"
	timeout                 = 10
	maxHeaderBytes          = 1024
	// tenDays ten days
	tenDays                = 10
	maxIPConnLimit         = 128
	maxConcurrency         = 512
	defaultConnection      = 20
	maxProfilingTime       = 2000
	minHccsBWProfilingTime = 1
	maxHccsBWProfilingTime = 1000
	defaultShutDownTimeout = 30 * time.Second
)

const (
	prometheusPlatform         = "Prometheus"
	telegrafPlatform           = "Telegraf"
	pollIntervalStr            = "poll_interval"
	platformStr                = "platform"
	defaultProfilingTime       = 200
	defaultHccsBwProfilingTime = 200
)

func main() {
	flag.Parse()
	if version {
		fmt.Printf("NPU-exporter version: %s \n", versions.BuildVersion)
		return
	}
	err := logger.InitLogger(platform)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)
		return
	}
	initPaprams()
	err = paramValid(platform)
	if err != nil {
		return
	}
	dmgr, err := devmanager.AutoInit("", deviceResetTimeout)
	if err != nil {
		logger.Errorf("new npu collector failed, error is %v", err)
		return
	}
	logger.Infof("npu exporter starting and the version is %s", versions.BuildVersion)
	deviceParser := container.MakeDevicesParser(readCntMonitoringFlags())
	defer deviceParser.Close()

	if err := deviceParser.Init(); err != nil {
		logger.Errorf("failed to init devices parser: %v", err)
	}
	deviceParser.Timeout = time.Duration(updateTime) * time.Second

	colcommon.Collector = colcommon.NewNpuCollector(cacheTime, time.Duration(updateTime)*time.Second, deviceParser, dmgr)
	plugins.InitTextMetricsDesc(textMetricsFilePath)
	plugins.RegisterPlugin()
	config.Register(colcommon.Collector)

	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	colcommon.InitCardInfo(wg, ctx, colcommon.Collector)
	colcommon.StartContainerInfoCollect(ctx, cancel, wg, colcommon.Collector)

	colcommon.StartCollect(wg, ctx, colcommon.Collector)
	switch platform {
	case prometheusPlatform:
		prometheusProcss(wg, ctx, cancel)
	case telegrafPlatform:
		telegrafProcess()
	default:
		err = fmt.Errorf("err platform input")
	}
	wg.Wait()
}

func prometheusProcss(wg *sync.WaitGroup, ctx context.Context, cancel context.CancelFunc) {
	c := prom.NewPrometheusCollector(colcommon.Collector)
	reg := prometheus.NewRegistry()
	reg.MustRegister(c)

	wg.Add(1)
	go func() {
		startServe(ctx, cancel, reg)
		wg.Done()
	}()
}

func initPaprams() {
	common.SetHccsBWProfilingTime(hccsBWProfilingTime)
	common.SetExternalParams(profilingTime)
}

func paramValid(platform string) error {
	var err error
	switch platform {
	case prometheusPlatform:
		err = paramValidInPrometheus()
	case telegrafPlatform:
		err = paramValidInTelegraf()
	default:
		err = fmt.Errorf("err platform input")
	}
	if err != nil {
		logger.Error(err)
		return err
	}
	return nil
}

func initConfig() *limiter.HandlerConfig {
	conf := &limiter.HandlerConfig{
		PrintLog:         true,
		Method:           http.MethodGet,
		LimitBytes:       limiter.DefaultDataLimit,
		TotalConCurrency: concurrency,
		IPConCurrency:    limitIPReq,
		CacheSize:        limiter.DefaultCacheSize,
	}
	return conf
}

func newServerAndListener(conf *limiter.HandlerConfig) (*http.Server, net.Listener) {
	handler, err := limiter.NewLimitHandlerV2(http.DefaultServeMux, conf)
	if err != nil {
		hwlog.RunLog.Error(err)
		return nil, nil
	}
	s := &http.Server{
		Addr:           ip + ":" + strconv.Itoa(port),
		Handler:        handler,
		ReadTimeout:    timeout * time.Second,
		WriteTimeout:   timeout * time.Second,
		MaxHeaderBytes: maxHeaderBytes,
		ErrorLog:       log.New(&hwlog.SelfLogWriter{}, "", log.Lshortfile),
	}
	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		logger.Errorf("listen ip and port error: %v", err)
		return nil, nil
	}
	limitLs, err := limiter.LimitListener(ln, limitTotalConn, limitIPConn, limiter.DefaultCacheSize)
	if err != nil {
		hwlog.RunLog.Error(err)
		return nil, nil
	}
	return s, limitLs
}

func readCntMonitoringFlags() container.CntNpuMonitorOpts {
	opts := container.CntNpuMonitorOpts{UseOciBackup: true, UseCriBackup: true}
	switch containerMode {
	case containerModeDocker:
		opts.EndpointType = container.EndpointTypeDockerd
		opts.OciEndpoint = container.DefaultDockerAddr
		opts.CriEndpoint = container.DefaultDockerShim
	case containerModeContainerd:
		opts.EndpointType = container.EndpointTypeContainerd
		opts.OciEndpoint = container.DefaultContainerdAddr
		opts.CriEndpoint = container.DefaultContainerdAddr
	case containerModeIsula:
		opts.EndpointType = container.EndpointTypeIsula
		opts.OciEndpoint = container.DefaultIsuladAddr
		opts.CriEndpoint = container.DefaultIsuladAddr
	default:
		hwlog.RunLog.Error("invalid container mode setting,reset to docker")
		opts.EndpointType = container.EndpointTypeDockerd
		opts.OciEndpoint = container.DefaultDockerAddr
		opts.CriEndpoint = container.DefaultDockerShim
	}
	if containerd != "" {
		opts.OciEndpoint = containerd
		opts.UseOciBackup = false
	}
	if endpoint != "" {
		opts.CriEndpoint = endpoint
		opts.UseCriBackup = false
	}
	return opts
}

func checkIPAndPortInPrometheus() error {
	if port < portLeft || port > portRight {
		return errors.New("the port is invalid")
	}
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return errors.New("the listen ip is invalid")
	}
	ip = parsedIP.String()
	logger.Infof("listen on: %s", ip)
	return nil
}

func paramValidInPrometheus() error {
	checks := []func() error{
		checkIPAndPortInPrometheus,
		checkUpdateTime,
		containerSockCheck,
		checkLimitIPReqFormat,
		checkLimitIPConn,
		checkLimitTotalConn,
		checkCacheSize,
		checkConcurrency,
		checkProfilingTime,
		checkHccsBWProfilingTime,
		checkDeviceResetTimeout,
		checkPollIntervalInCmdLine,
	}

	for _, check := range checks {
		if err := check(); err != nil {
			return err
		}
	}
	return nil
}

func checkUpdateTime() error {
	if updateTime > oneMinute || updateTime < 1 {
		return errors.New("the updateTime is invalid")
	}
	return nil
}

func checkLimitIPReqFormat() error {
	reg := regexp.MustCompile(limiter.IPReqLimitReg)
	if !reg.Match([]byte(limitIPReq)) {
		return errors.New("limitIPReq format error")
	}
	return nil
}

func checkLimitIPConn() error {
	if limitIPConn < 1 || limitIPConn > maxIPConnLimit {
		return errors.New("limitIPConn is invalid")
	}
	return nil
}

func checkLimitTotalConn() error {
	if limitTotalConn < 1 || limitTotalConn > maxConcurrency {
		return errors.New("limitTotalConn is invalid")
	}
	return nil
}

func checkCacheSize() error {
	if cacheSize < 1 || cacheSize > limiter.DefaultCacheSize*tenDays {
		return errors.New("cacheSize is invalid")
	}
	return nil
}

func checkConcurrency() error {
	if concurrency < 1 || concurrency > maxConcurrency {
		return errors.New("concurrency is invalid")
	}
	return nil
}

func checkProfilingTime() error {
	if profilingTime < 1 || profilingTime > maxProfilingTime {
		return errors.New("profilingTime range error")
	}
	return nil
}

func checkHccsBWProfilingTime() error {
	if hccsBWProfilingTime < minHccsBWProfilingTime || hccsBWProfilingTime > maxHccsBWProfilingTime {
		return errors.New("hccsBWProfilingTime range error")
	}
	return nil
}

func checkDeviceResetTimeout() error {
	if deviceResetTimeout < api.MinDeviceResetTimeout || deviceResetTimeout > api.MaxDeviceResetTimeout {
		return errors.New("deviceResetTimeout range error")
	}
	return nil
}

func checkPollIntervalInCmdLine() error {
	cmdLine := strings.Join(os.Args[1:], "")
	if strings.Contains(cmdLine, pollIntervalStr) {
		return fmt.Errorf("%s is not support this scene", pollIntervalStr)
	}
	return nil
}

func containerSockCheck() error {
	if endpoint != "" && !strings.Contains(endpoint, ".sock") {
		return errors.New("endpoint file is not sock address")
	}
	if containerd != "" && !strings.Contains(containerd, ".sock") {
		return errors.New("containerd file is not sock address")
	}
	if endpoint != "" && !strings.Contains(endpoint, unixPre) {
		endpoint = unixPre + endpoint
	}
	if containerd != "" && !strings.Contains(containerd, unixPre) {
		containerd = unixPre + containerd
	}
	return nil
}

func init() {
	flag.IntVar(&port, "port", portConst,
		"The server port of the http service,range[1025-40000]")
	flag.StringVar(&ip, "ip", "",
		"The listen ip of the service,0.0.0.0 is not recommended when install on Multi-NIC host")
	flag.IntVar(&updateTime, "updateTime", updateTimeConst,
		"Interval (seconds) to update the npu metrics cache,range[1-60]")
	flag.BoolVar(&version, "version", false,
		"If true,query the version of the program (default false)")
	flag.StringVar(&containerMode, "containerMode", containerModeDocker,
		"Set 'docker' for monitoring docker containers or 'containerd' for CRI & containerd")
	flag.StringVar(&containerd, "containerd", "",
		"The endpoint of containerd used for listening containers' events")
	flag.StringVar(&endpoint, "endpoint", "",
		"The endpoint of the CRI  server to which will be connected")
	flag.IntVar(&concurrency, "concurrency", defaultConcurrency,
		"The max concurrency of the http server, range is [1-512]")
	// hwlog configuration
	flag.IntVar(&logger.HwLogConfig.LogLevel, "logLevel", 0,
		"Log level, -1-debug, 0-info, 1-warning, 2-error, 3-critical(default 0)")
	flag.IntVar(&logger.HwLogConfig.MaxAge, "maxAge", hwlog.DefaultMinSaveAge,
		"Maximum number of days for backup log files, range [7, 700] days")
	flag.StringVar(&logger.HwLogConfig.LogFileName, "logFile", defaultLogFile,
		"Log file path. If the file size exceeds 20MB, will be rotated")
	flag.IntVar(&logger.HwLogConfig.MaxBackups, "maxBackups", hwlog.DefaultMaxBackups,
		"Maximum number of backup log files, range is (0, 30]")
	flag.IntVar(&cacheSize, "cacheSize", limiter.DefaultCacheSize, "the cacheSize for ip limit,"+
		"range  is [1,1024000],keep default normally")
	flag.IntVar(&limitIPConn, "limitIPConn", defaultConcurrency, "the tcp connection limit for each Ip,"+
		"range  is [1,128]")
	flag.IntVar(&limitTotalConn, "limitTotalConn", defaultConnection, "the tcp connection limit for all"+
		" request,range  is [1,512]")
	flag.StringVar(&limitIPReq, "limitIPReq", "20/1",
		"the http request limit counts for each Ip,20/1 means allow 20 request in 1 seconds")
	flag.StringVar(&platform, "platform", "Prometheus", "the data reporting platform, "+
		"just support Prometheus and Telegraf")
	flag.StringVar(&textMetricsFilePath, "textMetricsFilePath", "",
		"text indicator collection path, only support specified one file path")
	flag.DurationVar(&pollInterval, pollIntervalStr, 1*time.Second,
		"how often to send metrics when use Telegraf plugin, "+
			"needs to be used with -platform=Telegraf, otherwise, it does not take effect")
	flag.IntVar(&profilingTime, "profilingTime", defaultProfilingTime,
		"config pcie bandwidth profiling time, range is [1, 2000]")
	flag.IntVar(&hccsBWProfilingTime, api.HccsBWProfilingTimeStr, defaultHccsBwProfilingTime,
		"config "+api.Hccs+" bandwidth profiling time, range is [1, 1000]")
	flag.IntVar(&deviceResetTimeout, api.DeviceResetTimeout, api.DefaultDeviceResetTimeout,
		"when npu-exporter starts, if the number of chips is insufficient, the maximum duration to wait for "+
			"the driver to report all chips, unit second, range [10, 600]")
}

func indexHandler(w http.ResponseWriter, _ *http.Request) {
	var proposal = "http"
	_, err := w.Write([]byte(
		`<html>
			<head><title>NPU-Exporter</title></head>
			<body>
			<h1 align="center">NPU-Exporter</h1>
			<p align="center">Welcome to use NPU-Exporter,the Prometheus metrics url is ` + proposal + `://ip:` +
			strconv.Itoa(port) + `/metrics: <a href="./metrics">Metrics</a></p>
			</body>
			</html>`))
	if err != nil {
		logger.Errorf("Write to response error: %v", err)
	}
}

func prometheusProcess() {

}

func startServe(ctx context.Context, cancel context.CancelFunc, reg *prometheus.Registry) {
	http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{ErrorHandling: promhttp.ContinueOnError}))
	http.Handle("/", http.HandlerFunc(indexHandler))
	conf := initConfig()
	s, limitLs := newServerAndListener(conf)
	if s == nil || limitLs == nil {
		cancel()
		return
	}

	go func() {
		logger.Warn("enable unsafe http server")
		if err := s.Serve(limitLs); err != nil {
			logger.Errorf("Http server error: %v and stopped", err)
			cancel()
		}
	}()

	<-ctx.Done()
	shutErr := func() error {
		logger.Info("received stop signal, STOP http server")
		ctxShutDown, timeOut := context.WithTimeout(context.Background(), defaultShutDownTimeout)
		defer timeOut()
		return s.Shutdown(ctxShutDown)
	}()
	if shutErr != nil {
		logger.Errorf("shutdown http server error: %v", shutErr)
	}
}

func paramValidInTelegraf() error {
	// cmdLine here must contain "-platform=Telegraf", otherwise, it will enter the Prometheus process
	cmdLine := os.Args[1:]

	// store the preset parameter names in the map
	presetParamsMap := map[string]bool{
		platformStr:                true,
		pollIntervalStr:            true,
		api.HccsBWProfilingTimeStr: true,
	}

	if len(cmdLine) > len(presetParamsMap) {
		return errors.New("too many parameters")
	}

	var paramLen = 2
	// check every input params
	for _, param := range cmdLine {
		param = strings.TrimPrefix(param, "-")
		split := strings.Split(param, "=")
		if len(split) != paramLen {
			return fmt.Errorf("the param [%s] is a wrong format", param)
		}
		paramName := split[0]
		if !presetParamsMap[paramName] {
			return fmt.Errorf("not support [%s] in Telegraf", paramName)
		}
	}

	if hccsBWProfilingTime < minHccsBWProfilingTime || hccsBWProfilingTime > maxHccsBWProfilingTime {
		return errors.New(api.Hccs + "BWProfilingTime range error")
	}
	return nil
}

func telegrafProcess() {
	// create the shim. This is what will run your plugins.
	shim := shim.New()

	// If no config is specified, all imported plugins are loaded.
	// otherwise follow what the config asks for.
	// Check for settings from a config toml file,
	// (or just use whatever plugins were imported above)
	configFile := ""
	err := shim.LoadConfig(&configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Err loading input: %s\n", err)
		return
	}

	// run the input plugin(s) until stdin closes, or we receive a termination signal
	if err := shim.Run(pollInterval); err != nil {
		fmt.Fprintf(os.Stderr, "Err: %s\n", err)
		return
	}
}
