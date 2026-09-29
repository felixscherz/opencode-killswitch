package darwin

import (
	"os/exec"
	"testing"
	"time"

	"github.com/felixscherz/opencode-killswitch/internal/killswitch"
)

func startChild(t *testing.T, name string, args ...string) (*exec.Cmd, <-chan struct{}) {
	t.Helper()
	cmd := exec.Command(name, args...)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	// Reap the child so it does not linger as a zombie after being killed.
	exited := make(chan struct{})
	go func() { _ = cmd.Wait(); close(exited) }()
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	return cmd, exited
}

func TestProcessesIncludesRunningProcessWithNameAndFootprint(t *testing.T) {
	cmd, _ := startChild(t, "sleep", "30")

	procs, err := Processes{}.Processes()
	if err != nil {
		t.Fatal(err)
	}

	for _, p := range procs {
		if p.PID == killswitch.PID(cmd.Process.Pid) {
			if p.Name != "sleep" {
				t.Errorf("name = %q, want sleep", p.Name)
			}
			if p.Footprint == 0 {
				t.Error("footprint = 0, want non-zero")
			}
			return
		}
	}
	t.Fatalf("pid %d not found among %d processes", cmd.Process.Pid, len(procs))
}

func TestTerminateStopsAProcessThatExitsOnSigterm(t *testing.T) {
	cmd, exited := startChild(t, "sleep", "30")

	err := Terminator{Grace: 5 * time.Second}.Terminate(killswitch.Process{PID: killswitch.PID(cmd.Process.Pid)})

	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-exited:
	case <-time.After(time.Second):
		t.Fatal("process still running")
	}
}

func TestTerminateForceKillsAProcessThatIgnoresSigterm(t *testing.T) {
	cmd, exited := startChild(t, "sh", "-c", `trap "" TERM; while :; do sleep 0.01; done`)
	time.Sleep(100 * time.Millisecond) // let the shell install its trap

	start := time.Now()
	err := Terminator{Grace: 200 * time.Millisecond}.Terminate(killswitch.Process{PID: killswitch.PID(cmd.Process.Pid)})

	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-exited:
	case <-time.After(time.Second):
		t.Fatal("process still running")
	}
	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Errorf("killed after %v, want the grace period to be honoured", elapsed)
	}
}
