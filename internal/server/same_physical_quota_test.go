package server

import (
	"context"
	"flag"
	"testing"

	"github.com/Project-HAMi/HAMi/pkg/device"
	"github.com/Project-HAMi/HAMi/pkg/device/ascend"
	"github.com/Project-HAMi/HAMi/pkg/util"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

func TestSamePhysicalCardKeepsPerContainerQuota(t *testing.T) {
	const common = "Ascend910B3"
	const physicalID = "card-seven"
	const cardResource = "huawei.com/Ascend910B3"
	const memoryResource = cardResource + "-memory"
	const coreResource = cardResource + "-core"

	flags := flag.NewFlagSet("ascend-quota-regression", flag.ContinueOnError)
	ascend.ParseConfig(flags)
	if err := flags.Set("enable-ascend", "true"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = flags.Set("enable-ascend", "false") })
	for _, annotations := range []map[string]string{device.InRequestDevices, device.SupportDevices, util.HandshakeAnnos} {
		value, existed := annotations[common]
		t.Cleanup(func() {
			if existed {
				annotations[common] = value
			} else {
				delete(annotations, common)
			}
		})
	}
	backends := ascend.InitDevices(ascend.VNPUs{
		HamiVnpuCore: true,
		Configs: []ascend.VNPUConfig{{
			ChipName: "910B3", CommonWord: common, ResourceName: cardResource,
			ResourceMemoryName: memoryResource, ResourceCoreName: coreResource,
			MemoryAllocatable: 65536, MemoryCapacity: 65536, AICore: 20,
		}},
	})
	if len(backends) != 1 {
		t.Fatalf("expected one Ascend backend, got %d", len(backends))
	}
	backend := backends[0]
	t.Cleanup(setupInRequestDevices(common))

	ps, _ := hamiTestServer(t)
	ps.nodeName = "test-node"
	ps.commonWord = common
	ps.toAllocDeviceAnno = device.InRequestDevices[common]
	ann := map[string]string{
		VNPUModeAnnotation:       VNPUModeHamiCore,
		util.BindTimeAnnotations: "2024-01-01T00:00:00Z",
		util.DeviceBindPhase:     util.DeviceBindAllocating,
	}
	node, pod, cleanup := setupAllocateEnv(ps.nodeName, "test-pod", "default", 3, ann)
	cleanup()

	card := &device.DeviceUsage{
		ID: physicalID, Type: common, Count: 10, Totalmem: 65536, Totalcore: 100, Health: true,
	}
	allocated := device.PodDevices{}
	desired := []struct{ slot, memory, compute string }{
		{physicalID + "-0", "4096", "25"},
		{physicalID + "-1", "8192", "50"},
	}
	for i, want := range desired {
		if i > 0 {
			// The middle container requests no NPU and has no runtime entry.
			allocated[common] = append(allocated[common], nil)
		}
		container := &pod.Spec.Containers[i*2]
		container.Resources.Limits = v1.ResourceList{
			v1.ResourceName(cardResource):   resource.MustParse("1"),
			v1.ResourceName(memoryResource): resource.MustParse(want.memory),
			v1.ResourceName(coreResource):   resource.MustParse(want.compute),
		}
		request := backend.GenerateResourceRequests(container)
		fit, devices, reason := backend.Fit(
			[]*device.DeviceUsage{card}, request, pod, &device.NodeInfo{Node: node}, &allocated,
		)
		selected := devices[common]
		if !fit || len(selected) != 1 || selected[0].UUID != physicalID {
			t.Fatalf("scheduler Fit for %s: fit=%v selected=%+v reason=%q", container.Name, fit, selected, reason)
		}
		if err := backend.AddResourceUsage(pod, card, &selected[0]); err != nil {
			t.Fatal(err)
		}
		allocated[common] = append(allocated[common], selected)
		t.Logf("scheduler %s => %s mem=%d core=%d", container.Name, selected[0].UUID, selected[0].Usedmem, selected[0].Usedcores)
	}
	backend.PatchAnnotations(pod, &ann, allocated)
	t.Logf("scheduler runtime annotation: %s", ann[ps.allocAnno])
	pod.Annotations = ann
	node.ObjectMeta.ResourceVersion = ""
	pod.ObjectMeta.ResourceVersion = ""
	requests := make([]*v1beta1.ContainerAllocateRequest, len(desired))
	for i, want := range desired {
		requests[i] = &v1beta1.ContainerAllocateRequest{DevicesIds: []string{want.slot}}
	}
	assertQuota := func(t *testing.T, response *v1beta1.ContainerAllocateResponse, index int) {
		t.Helper()
		want := desired[index]
		memory := response.Envs["NPU_MEMORY_LIMIT_MB"]
		compute := response.Envs["NPU_COMPUTE_LIMIT"]
		if memory != want.memory || compute != want.compute {
			t.Errorf("%s received %s MiB/%s%%; want %s MiB/%s%%", want.slot, memory, compute, want.memory, want.compute)
		}
	}
	for _, batchSize := range []int{1, len(requests)} {
		name := "separate_calls"
		if batchSize == len(requests) {
			name = "batched_call"
		}
		t.Run(name, func(t *testing.T) {
			t.Cleanup(setupFakeClient([]*v1.Pod{pod}, []*v1.Node{node}))
			for i := 0; i < len(requests); i += batchSize {
				response, err := ps.Allocate(context.Background(), &v1beta1.AllocateRequest{
					ContainerRequests: requests[i : i+batchSize],
				})
				if err != nil {
					t.Fatalf("Allocate: %v", err)
				}
				if len(response.ContainerResponses) != batchSize {
					t.Fatalf("Allocate returned %d container responses, want %d", len(response.ContainerResponses), batchSize)
				}
				for j, container := range response.ContainerResponses {
					assertQuota(t, container, i+j)
				}
			}
		})
	}
}
