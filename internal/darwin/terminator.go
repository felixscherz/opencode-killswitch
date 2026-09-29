package darwin

import (
	"errors"
	"fmt"
	"syscall"
	"time"

	"github.com/felixscherz/opencode-killswitch/internal/killswitch"
)

const pollInterval = 50 * time.Millisecond

// Terminator asks a process to exit with SIGTERM and sends SIGKILL if it is
// still running after Grace.
type Terminator struct {
	Grace time.Duration
}

func (t Terminator) Terminate(p killswitch.Process) error {
	pid := int(p.PID)
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		return fmt.Errorf("sending SIGTERM to %d: %w", pid, err)
	}
	if waitForExit(pid, t.Grace) {
		return nil
	}
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("sending SIGKILL to %d: %w", pid, err)
	}
	if !waitForExit(pid, 5*time.Second) {
		return fmt.Errorf("process %d still running after SIGKILL", pid)
	}
	return nil
}

func waitForExit(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(pollInterval)
	}
}
