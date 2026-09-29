// Package darwin implements the killswitch ports on macOS.
package darwin

/*
#include <libproc.h>
#include <sys/resource.h>
#include <stdlib.h>

static int footprint(int pid, uint64_t *out) {
	struct rusage_info_v2 ri;
	if (proc_pid_rusage(pid, RUSAGE_INFO_V2, (rusage_info_t *)&ri) != 0) {
		return -1;
	}
	*out = ri.ri_phys_footprint;
	return 0;
}
*/
import "C"

import (
	"fmt"
	"path/filepath"
	"unsafe"

	"github.com/felixscherz/opencode-killswitch/internal/killswitch"
)

// Processes lists the processes visible to the current user through libproc.
type Processes struct{}

func (Processes) Processes() ([]killswitch.Process, error) {
	pids, err := listPIDs()
	if err != nil {
		return nil, err
	}
	procs := make([]killswitch.Process, 0, len(pids))
	pathBuf := make([]byte, C.PROC_PIDPATHINFO_MAXSIZE)
	for _, pid := range pids {
		// Processes of other users, and those that exited since listing,
		// cannot be inspected. They are skipped.
		var footprint C.uint64_t
		if C.footprint(pid, &footprint) != 0 {
			continue
		}
		name, ok := processName(pid, pathBuf)
		if !ok {
			continue
		}
		procs = append(procs, killswitch.Process{
			PID:       killswitch.PID(pid),
			Name:      name,
			Footprint: killswitch.Bytes(footprint),
		})
	}
	return procs, nil
}

func listPIDs() ([]C.int, error) {
	count := C.proc_listallpids(nil, 0)
	if count <= 0 {
		return nil, fmt.Errorf("proc_listallpids: could not count processes")
	}
	// Leave headroom for processes started between the two calls.
	pids := make([]C.int, int(count)+64)
	size := C.int(len(pids)) * C.int(unsafe.Sizeof(pids[0]))
	count = C.proc_listallpids(unsafe.Pointer(&pids[0]), size)
	if count <= 0 {
		return nil, fmt.Errorf("proc_listallpids: could not list processes")
	}
	return pids[:count], nil
}

// processName returns the executable's file name. The full path is preferred
// over proc_name, which truncates long names.
func processName(pid C.int, buf []byte) (string, bool) {
	n := C.proc_pidpath(pid, unsafe.Pointer(&buf[0]), C.uint32_t(len(buf)))
	if n > 0 {
		return filepath.Base(string(buf[:n])), true
	}
	n = C.proc_name(pid, unsafe.Pointer(&buf[0]), C.uint32_t(len(buf)))
	if n > 0 {
		return string(buf[:n]), true
	}
	return "", false
}
