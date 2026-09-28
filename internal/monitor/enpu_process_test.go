package monitor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnpuProcessContainer(t *testing.T) {
	id := strings.Repeat("a1", 32)
	other := strings.Repeat("b2", 32)
	pod := "pod12345678-1234-1234-1234-123456789abc"
	systemd := "/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-" + pod + ".slice/cri-containerd-" + id + ".scope"
	cases := []struct {
		name string
		data string
		want string
		fail bool
	}{
		{"v2 containerd systemd", "0::" + systemd + "\n", id, false},
		{"v1 containerd systemd", "4:cpu,cpuacct:" + systemd + "\n", id, false},
		{"docker systemd", "0::/system.slice/docker-" + id + ".scope\n", id, false},
		{"outside cgroup namespace", "0::/../../system.slice/docker-" + id + ".scope\n", id, false},
		{"outside Kubernetes cgroup namespace", "0::/../../.." + systemd + "\n", id, false},
		{"namespace parent", "0::/../..\n", "", false},
		{"outside namespace host", "0::/../../init.scope\n", "", false},
		{"crio systemd", "0::/kubepods.slice/crio-" + id + ".scope\n", id, false},
		{"containerd scope", "0::/system.slice/containerd-" + id + ".scope\n", id, false},
		{"uppercase normalized", "0::/docker/" + strings.ToUpper(id) + "\n", id, false},
		{"cgroupfs guaranteed", "0::/kubepods/" + pod + "/" + id + "\n", id, false},
		{"cgroupfs burstable", "5:memory:/kubepods/burstable/" + pod + "/" + id + "\n", id, false},
		{"cgroupfs besteffort", "5:memory:/kubepods/besteffort/" + pod + "/" + id + "\n", id, false},
		{"docker cgroupfs", "0::/docker/" + id + "\n", id, false},
		{"nested process cgroup", "0::/docker/" + id + "/app/workers\n", id, false},
		{"consistent controllers", "8:memory:/docker/" + id + "\n4:cpu,cpuacct:" + systemd + "\n1:name=systemd:/\n", id, false},
		{"hybrid hierarchies", "0::" + systemd + "\n1:name=systemd:/docker/" + strings.ToUpper(id) + "\n", id, false},
		{"host root", "0::/\n", "", false},
		{"host daemon", "1:name=systemd:/system.slice/docker.service\n", "", false},
		{"host session", "0::/user.slice/user-1000.slice/session-1.scope\n", "", false},
		{"arbitrary host hex directory", "0::/system.slice/host.service/" + id + "\n", "", false},
		{"substring parent is host", "0::/notdocker/" + id + "\n", "", false},
		{"no trailing newline", "0::/docker/" + id, id, false},
		{"conflicting controllers", "8:memory:/docker/" + id + "\n4:cpu:/docker/" + other + "\n", "", true},
		{"conflicting nested scopes", "0::/docker/" + id + "/crio-" + other + ".scope\n", "", true},
		{"short docker ID", "0::/docker/abcdef123456\n", "", true},
		{"short scope ID", "0::/system.slice/cri-containerd-abcdef123456.scope\n", "", true},
		{"short pod ID", "0::/kubepods/" + pod + "/abcdef123456\n", "", true},
		{"nonhex ID", "0::/docker/" + strings.Repeat("z", 64) + "\n", "", true},
		{"long ID", "0::/docker/" + id + "a\n", "", true},
		{"unknown runtime scope", "0::/system.slice/unknown-" + id + ".scope\n", "", true},
		{"unknown pod layout", "0::/kubepods/burstable/" + id + "\n", "", true},
		{"unknown runtime cgroupfs", "0::/containerd/" + id + "\n", "", true},
		{"scope prefix forgery", "0::/system.slice/fake-cri-containerd-" + id + ".scope\n", "", true},
		{"scope suffix forgery", "0::/system.slice/cri-containerd-" + id + ".scope.extra\n", "", true},
		{"docker missing ID", "0::/docker\n", "", true},
		{"pod missing ID", "0::/kubepods/" + pod + "\n", "", true},
		{"relative path", "0::docker/" + id + "\n", "", true},
		{"empty path", "0::\n", "", true},
		{"empty component", "0::/docker//" + id + "\n", "", true},
		{"dot component", "0::/docker/./" + id + "\n", "", true},
		{"parent traversal", "0::/docker/../" + id + "\n", "", true},
		{"empty file", "", "", true},
		{"malformed line", "not-a-cgroup-line\n", "", true},
		{"malformed after valid", "0::/docker/" + id + "\nbad\n", "", true},
		{"empty line", "0::/\n\n", "", true},
		{"negative hierarchy", "-1:cpu:/\n", "", true},
		{"nonnumeric hierarchy", "x:cpu:/\n", "", true},
		{"oversized hierarchy", "4294967296:cpu:/\n", "", true},
		{"v2 with controller", "0:cpu:/\n", "", true},
		{"v1 missing controller", "1::/\n", "", true},
		{"malformed controller", "1:cpu,,memory:/\n", "", true},
		{"NUL byte", "0::/docker/" + id + "\x00\n", "", true},
		{"carriage return", "0::/\r\n", "", true},
		{"whitespace", "0::/system.slice/bad name\n", "", true},
		{"invalid UTF8", "0::/\xff\n", "", true},
		{"file too large", "0::/" + strings.Repeat("x", enpuCgroupMaxBytes) + "\n", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "42")
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "cgroup"), []byte(tc.data), 0600); err != nil {
				t.Fatal(err)
			}
			got, err := enpuProcessContainer(root, 42)
			if (err != nil) != tc.fail || got != tc.want {
				t.Fatalf("got (%q, %v), want (%q, error=%v)", got, err, tc.want, tc.fail)
			}
		})
	}
}

func TestEnpuProcessContainerReadErrors(t *testing.T) {
	root := t.TempDir()
	for _, pid := range []int32{0, -1, 42} {
		if id, err := enpuProcessContainer(root, pid); err == nil || id != "" {
			t.Errorf("PID %d: got (%q, %v), want error", pid, id, err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "43", "cgroup"), 0700); err != nil {
		t.Fatal(err)
	}
	if id, err := enpuProcessContainer(root, 43); err == nil || id != "" {
		t.Errorf("reading directory: got (%q, %v), want error", id, err)
	}
}
