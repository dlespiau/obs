package obs

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// NamespaceKind is the type of Linux namespaces.
type NamespaceKind int

const (
	// NetworkNS is the network namespace.
	NetworkNS NamespaceKind = iota
	// IPCNS is the SysV IPC namespace.
	IPCNS
	// UTSNS is the UTS namespace.
	UTSNS
	// MountNS is the mount namespace.
	MountNS
	// PIDNS is the PID namespace.
	PIDNS
	// UserNS is the user namespace.
	UserNS
	// CgroupNS is the cgroup namespace.
	CgroupNS
)

var nsProcFiles = []string{"net", "ipc", "uts", "mnt", "pid", "user", "cgroup"}

// Process provides mechanisms to retrieve information about a process.
type Process struct {
	pid int
}

// NewProcess creates a new Process.
func NewProcess(pid int) *Process {
	return &Process{
		pid: pid,
	}
}

// PID returns the PID of the process.
func (p *Process) PID() int {
	return p.pid
}

func parseNS(s string) (uint64, error) {
	start := strings.IndexByte(s, '[')
	end := strings.IndexByte(s, ']')
	if start == -1 || end == -1 {
		return 0, fmt.Errorf("invalid ns link: %s", s)
	}
	ns, err := strconv.ParseUint(s[start+1:end], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("not an unsigned int: %s", s[start+1:end])
	}
	return ns, nil
}

// Namespace returns the namespace inode for the specified ns.
func (p *Process) Namespace(kind NamespaceKind) (uint64, error) {
	f := "/proc/" + strconv.Itoa(p.pid) + "/ns/" + nsProcFiles[kind]
	link, err := os.Readlink(f)
	if err != nil {
		return 0, errors.Wrap(err, "namespace")
	}
	ns, err := parseNS(link)
	if err != nil {
		return 0, errors.Wrap(err, "namespace: parse")
	}
	return ns, nil
}
