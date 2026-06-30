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

package server

import (
	"context"
	"fmt"
	"net"
	"path"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/klog/v2"
	"k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/Project-HAMi/HAMi/pkg/device"
	"github.com/Project-HAMi/HAMi/pkg/util"
)

func (ps *PluginServer) watchAndRegister() {
	ps.wg.Add(1)
	defer ps.wg.Done()
	timer := time.After(1 * time.Second)
	for {
		select {
		case <-ps.stopCh:
			klog.Infof("stop watch and register")
			return
		case <-timer:
		}
		unhealthy := ps.mgr.GetUnHealthIDs()
		if len(unhealthy) > 0 {
			if err := ps.mgr.UpdateDevice(); err != nil {
				klog.Errorf("update device error: %v", err)
				timer = time.After(5 * time.Second)
				continue
			}
			ps.healthCh <- unhealthy[0]
		}
		err := ps.registerHAMi()
		if err != nil {
			klog.Errorf("register HAMi error: %v", err)
			timer = time.After(5 * time.Second)
		} else {
			klog.V(3).Infof("register HAMi success")
			timer = time.After(30 * time.Second)
		}
	}
}

func (ps *PluginServer) registerHAMi() error {
	devs := ps.mgr.GetDevices()
	apiDevices := make([]*device.DeviceInfo, 0, len(devs))
	// hami currently believes that the index starts from 0 and is continuous.
	for i, dev := range devs {
		devcore := dev.AICore
		if ps.mgr.IsHamiVnpuCore() {
			devcore = HamiVnpuCoreMaxPercent
		}
		device := &device.DeviceInfo{
			Index:   uint(i),
			ID:      dev.UUID,
			Count:   int32(ps.mgr.VDeviceCount()),
			Devmem:  int32(dev.Memory),
			Devcore: devcore,
			Type:    ps.mgr.CommonWord(),
			Numa:    0,
			Health:  dev.Health,
		}
		if strings.HasPrefix(device.Type, Ascend910Prefix) {
			NetworkID, err := ps.getDeviceNetworkID(i, device.Type)
			if err != nil {
				return fmt.Errorf("get networkID error: %w", err)
			}
			device.CustomInfo = map[string]any{
				"NetworkID": NetworkID,
			}
		}
		apiDevices = append(apiDevices, device)
	}
	annos := make(map[string]string)
	annos[ps.registerAnno] = device.MarshalNodeDevices(apiDevices)
	annos[ps.handshakeAnno] = "Reported_" + time.Now().Add(time.Duration(*reportTimeOffset)*time.Second).Format("2006.01.02 15:04:05")

	if ps.mgr.IsHamiVnpuCore() {
		annos[VNPUNodeSelectorAnnotation] = "true"
		klog.V(4).Infof("Node %s has HamiVnpuCore enabled, patching annotation %s: true", ps.nodeName, VNPUNodeSelectorAnnotation)
	} else {
		annos[VNPUNodeSelectorAnnotation] = "false"
	}

	node, err := util.GetNode(ps.nodeName)
	if err != nil {
		return fmt.Errorf("get node %s error: %w", ps.nodeName, err)
	}
	err = util.PatchNodeAnnotations(node, annos)
	if err != nil {
		return fmt.Errorf("patch node %s annotations error: %w", ps.nodeName, err)
	}
	klog.V(5).Infof("patch node %s annotations: %v", ps.nodeName, annos)
	return nil
}

func (ps *PluginServer) getDeviceNetworkID(idx int, deviceType string) (int, error) {
	// For Ascend910C devices, all modules (dies) are interconnected via HCCS
	if deviceType == Ascend910CType {
		return 0, nil
	}

	if idx > 3 {
		return 1, nil
	}

	return 0, nil
}

func (ps *PluginServer) registerKubelet() error {
	if ps.registerKubeletFunc != nil {
		return ps.registerKubeletFunc()
	}
	conn, err := ps.dial(v1beta1.KubeletSocket, 5*time.Second)
	if err != nil {
		return err
	}
	defer func(conn *grpc.ClientConn) {
		_ = conn.Close()
	}(conn)
	client := v1beta1.NewRegistrationClient(conn)
	reqt := &v1beta1.RegisterRequest{
		Version:      v1beta1.Version,
		Endpoint:     path.Base(ps.socket),
		ResourceName: ps.mgr.ResourceName(),
		Options: &v1beta1.DevicePluginOptions{
			GetPreferredAllocationAvailable: false,
		},
	}

	_, err = client.Register(context.Background(), reqt)
	if err != nil {
		return err
	}
	return nil
}

func (ps *PluginServer) dial(unixSocketPath string, timeout time.Duration) (*grpc.ClientConn, error) {
	if ps.dialFunc != nil {
		return ps.dialFunc(unixSocketPath, timeout)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	target := "passthrough:///" + unixSocketPath
	c, err := grpc.NewClient(target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx2 context.Context, addr string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx2, "unix", addr)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("grpc.NewClient(%s): %w", target, err)
	}

	c.Connect()
	for {
		state := c.GetState()
		if state == connectivity.Ready {
			return c, nil
		}
		if state == connectivity.TransientFailure || state == connectivity.Shutdown {
			c.Close()
			return nil, fmt.Errorf("connection to %s failed (state: %s)", unixSocketPath, state)
		}
		// Block until the state changes or the deadline is exceeded.
		if !c.WaitForStateChange(ctx, state) {
			c.Close()
			return nil, fmt.Errorf("timed out waiting for connection to %s", unixSocketPath)
		}
	}
}
