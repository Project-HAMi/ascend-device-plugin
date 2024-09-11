/*
 * Copyright 2024 The HAMi Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"syscall"

	"github.com/Project-HAMi/ascend-device-plugin/version"
	"github.com/fsnotify/fsnotify"
	"huawei.com/npu-exporter/v6/common-utils/hwlog"
	"k8s.io/klog/v2"
	"k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

var (
	hwLoglevel = flag.Int("hw_loglevel", 0, "huawei log level, -1-debug, 0-info, 1-warning, 2-error 3-critical default value: 0")
	configFile = flag.String("config_file", "", "config file path")
	nodeName   = flag.String("node_name", os.Getenv("NODE_NAME"), "node name")
)

func checkFlags() {
	version.CheckVersionFlag()
	if *configFile == "" {
		klog.Fatalf("config file not set, use --config_file to set config file path")
	}
	if *nodeName == "" {
		klog.Fatalf("node name not set, use --node_name or env NODE_NAME to set node name")
	}
}

func start(ps *PluginServer) error {
	klog.Info("Starting FS watcher.")
	watcher, err := newFSWatcher(v1beta1.DevicePluginPath)
	if err != nil {
		return fmt.Errorf("failed to create FS watcher: %v", err)
	}
	defer func(watcher *fsnotify.Watcher) {
		_ = watcher.Close()
	}(watcher)

	klog.Info("Starting OS watcher.")
	sigs := newOSWatcher(syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	var restarting bool
	//var restartTimeout <-chan time.Time
restart:
	if restarting {
		err := ps.Stop()
		if err != nil {
			klog.Errorf("Failed to stop plugin server: %v", err)
		}
	}
	restarting = true
	klog.Info("Starting Plugins.")
	err = ps.Start()
	if err != nil {
		klog.Errorf("Failed to start plugin server: %v", err)
		return err
	}

	for {
		select {
		//case <-restartTimeout:
		//	goto restart
		case event := <-watcher.Events:
			if event.Name == v1beta1.KubeletSocket && event.Op&fsnotify.Create == fsnotify.Create {
				klog.Infof("inotify: %s created, restarting.", v1beta1.KubeletSocket)
				goto restart
			}
		case err := <-watcher.Errors:
			klog.Errorf("inotify: %s", err)
		case s := <-sigs:
			switch s {
			case syscall.SIGHUP:
				klog.Info("Received SIGHUP, restarting.")
				goto restart
			default:
				klog.Infof("Received signal \"%v\", shutting down.", s)
				goto exit
			}
		}
	}
exit:
	err = ps.Stop()
	if err != nil {
		klog.Errorf("Failed to stop plugin server: %v", err)
		return err
	}
	return nil
}

func main() {
	klog.InitFlags(nil)
	flag.Parse()
	checkFlags()
	klog.Infof("version: %s", version.GetVersion())
	klog.Infof("using config file: %s", *configFile)
	config := &hwlog.LogConfig{
		OnlyToStdout: true,
		LogLevel:     *hwLoglevel,
	}
	err := hwlog.InitRunLogger(config, context.Background())
	if err != nil {
		klog.Fatalf("init huawei run logger failed, %v", err)
	}
	mgr, err := NewAscendManager()
	if err != nil {
		klog.Fatalf("init AscendManager failed, error is %v", err)
	}
	err = mgr.LoadConfig(*configFile)
	if err != nil {
		klog.Fatalf("load config failed, error is %v", err)
	}
	server, err := NewPluginServer(mgr, *nodeName)
	if err != nil {
		klog.Fatalf("init PluginServer failed, error is %v", err)
	}

	err = start(server)
	if err != nil {
		klog.Fatalf("start PluginServer failed, error is %v", err)
	}
}
