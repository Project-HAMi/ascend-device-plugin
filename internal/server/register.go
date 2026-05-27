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
		device := &device.DeviceInfo{
			Index:   uint(i),
			ID:      dev.UUID,
			Count:   int32(ps.mgr.VDeviceCount()),
			Devmem:  int32(dev.Memory),
			Devcore: dev.AICore,
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
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	c, _ := grpc.NewClient(unixSocketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx2 context.Context, addr string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx2, "unix", addr)
		}),
	)

	// NewClient is non-blocking; block here to match the original WithBlock behaviour.
	if !c.WaitForStateChange(ctx, connectivity.Ready) {
		c.Close()
		return nil, ctx.Err()
	}

	return c, nil
}
