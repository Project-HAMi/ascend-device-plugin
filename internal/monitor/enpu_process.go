package monitor

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const enpuCgroupMaxBytes = 64 * 1024

var (
	enpuContainerIDPattern  = regexp.MustCompile(`^[[:xdigit:]]{64}$`)
	enpuRuntimeScopePattern = regexp.MustCompile(`^(?:cri-containerd|containerd|docker|crio)-([[:xdigit:]]{64})\.scope$`)
	enpuUnknownScopePattern = regexp.MustCompile(`^.+-[[:xdigit:]]{64}\.scope$`)
	enpuPodComponentPattern = regexp.MustCompile(`^pod[[:xdigit:]]{8}-[[:xdigit:]]{4}-[[:xdigit:]]{4}-[[:xdigit:]]{4}-[[:xdigit:]]{12}$`)
	enpuControllersPattern  = regexp.MustCompile(`^(?:[A-Za-z0-9_]+|name=[A-Za-z0-9_.-]+)(?:,(?:[A-Za-z0-9_]+|name=[A-Za-z0-9_.-]+))*$`)
)

// enpuProcessContainer resolves only recognized runtime cgroups. A successful
// empty result means the process has no container cgroup; malformed or ambiguous
// container membership must not be mistaken for a host process.
func enpuProcessContainer(procRoot string, pid int32) (string, error) {
	if pid <= 0 {
		return "", fmt.Errorf("invalid process ID %d", pid)
	}
	f, err := os.Open(filepath.Join(procRoot, strconv.FormatInt(int64(pid), 10), "cgroup"))
	if err != nil {
		return "", fmt.Errorf("read cgroup for PID %d: %w", pid, err)
	}
	defer func() {
		_ = f.Close()
	}()
	data, err := io.ReadAll(io.LimitReader(f, enpuCgroupMaxBytes+1))
	if err != nil {
		return "", fmt.Errorf("read cgroup for PID %d: %w", pid, err)
	}
	if len(data) == 0 || len(data) > enpuCgroupMaxBytes || !utf8.Valid(data) {
		return "", fmt.Errorf("invalid cgroup data for PID %d", pid)
	}

	containerID := ""
	for i, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		fields := strings.SplitN(line, ":", 3)
		if len(fields) != 3 || fields[0] == "" || strings.IndexFunc(line, func(r rune) bool {
			return unicode.IsControl(r) || unicode.IsSpace(r)
		}) >= 0 {
			return "", fmt.Errorf("malformed cgroup line %d", i+1)
		}
		for _, digit := range fields[0] {
			if digit < '0' || digit > '9' {
				return "", fmt.Errorf("invalid cgroup hierarchy at line %d", i+1)
			}
		}
		hierarchy, err := strconv.ParseUint(fields[0], 10, 32)
		if err != nil || (hierarchy == 0 && fields[1] != "") ||
			(hierarchy != 0 && !enpuControllersPattern.MatchString(fields[1])) {
			return "", fmt.Errorf("invalid cgroup hierarchy or controllers at line %d", i+1)
		}
		id, err := enpuContainerFromCgroupPath(fields[2])
		if err != nil {
			return "", fmt.Errorf("cgroup line %d: %w", i+1, err)
		}
		if id != "" {
			if containerID != "" && containerID != id {
				return "", fmt.Errorf("conflicting container IDs in process cgroups")
			}
			containerID = id
		}
	}
	return containerID, nil
}

func enpuContainerFromCgroupPath(path string) (string, error) {
	for strings.HasPrefix(path, "/../") {
		path = strings.TrimPrefix(path, "/..")
	}
	if path == "/" || path == "/.." {
		return "", nil
	}
	if !strings.HasPrefix(path, "/") {
		return "", fmt.Errorf("cgroup path is not absolute")
	}
	components := strings.Split(path[1:], "/")
	containerID := ""
	kubernetes := false
	addID := func(id string) error {
		if !enpuContainerIDPattern.MatchString(id) {
			return fmt.Errorf("container ID must contain exactly 64 hexadecimal characters")
		}
		id = strings.ToLower(id)
		if containerID != "" && containerID != id {
			return fmt.Errorf("conflicting container IDs in cgroup path")
		}
		containerID = id
		return nil
	}
	for i, component := range components {
		if component == "" || component == "." || component == ".." {
			return "", fmt.Errorf("malformed cgroup path component")
		}
		if component == "kubepods" || component == "kubepods.slice" ||
			(strings.HasPrefix(component, "kubepods-") && strings.HasSuffix(component, ".slice")) {
			kubernetes = true
		}
		if match := enpuRuntimeScopePattern.FindStringSubmatch(component); match != nil {
			if err := addID(match[1]); err != nil {
				return "", err
			}
			continue
		}
		if enpuUnknownScopePattern.MatchString(component) || enpuLooksLikeRuntimeScope(component) {
			return "", fmt.Errorf("unrecognized container scope")
		}
		if component == "docker" {
			if i+1 == len(components) {
				return "", fmt.Errorf("docker cgroup has no container ID")
			}
			if err := addID(components[i+1]); err != nil {
				return "", err
			}
		}
		if kubernetes && enpuPodComponentPattern.MatchString(component) {
			if i+1 == len(components) {
				return "", fmt.Errorf("pod cgroup has no container ID")
			}
			if err := addID(components[i+1]); err != nil {
				return "", err
			}
		}
		if component == "containerd" || component == "cri-containerd" || component == "crio" ||
			component == "libpod" || component == "podman" {
			return "", fmt.Errorf("unrecognized runtime cgroup layout")
		}
	}
	if kubernetes && containerID == "" {
		return "", fmt.Errorf("unrecognized Kubernetes container cgroup layout")
	}
	return containerID, nil
}

func enpuLooksLikeRuntimeScope(component string) bool {
	if !strings.Contains(component, ".scope") {
		return false
	}
	for _, prefix := range []string{"cri-containerd-", "containerd-", "docker-", "crio-", "libpod-", "podman-", "runc-"} {
		if strings.Contains(component, prefix) {
			return true
		}
	}
	return false
}
