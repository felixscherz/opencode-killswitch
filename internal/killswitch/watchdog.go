// Package killswitch terminates watched processes whose memory footprint
// grows past a limit.
package killswitch

import (
	"context"
	"fmt"
	"slices"
	"time"
)

type PID int

// Process is a snapshot of a running process.
type Process struct {
	PID  PID
	Name string // executable name, e.g. "opencode"
	// Footprint is the physical memory footprint, the value Activity Monitor
	// shows in its Memory column. Unlike RSS it includes compressed and
	// swapped memory, so it keeps growing past the size of physical RAM.
	Footprint Bytes
}

// Policy decides which processes are killed.
type Policy struct {
	Limit Bytes
	Names []string // executable names to watch, matched exactly
}

func (p Policy) Exceeds(proc Process) bool {
	return slices.Contains(p.Names, proc.Name) && proc.Footprint > p.Limit
}

type ProcessSource interface {
	Processes() ([]Process, error)
}

type Terminator interface {
	// Terminate stops the process, returning once it is gone.
	Terminate(Process) error
}

// Kill is the outcome of terminating a process that exceeded the limit.
type Kill struct {
	Process Process
	Err     error // nil when the process was terminated
}

type Watchdog struct {
	Policy     Policy
	Source     ProcessSource
	Terminator Terminator
}

// Check terminates every watched process currently over the limit.
func (w Watchdog) Check() ([]Kill, error) {
	procs, err := w.Source.Processes()
	if err != nil {
		return nil, fmt.Errorf("listing processes: %w", err)
	}
	var kills []Kill
	for _, proc := range procs {
		if w.Policy.Exceeds(proc) {
			kills = append(kills, Kill{Process: proc, Err: w.Terminator.Terminate(proc)})
		}
	}
	return kills, nil
}

// Run calls Check every interval until ctx is cancelled. Listing failures
// are reported and do not stop the watchdog.
func (w Watchdog) Run(ctx context.Context, interval time.Duration, onKill func(Kill), onError func(error)) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		kills, err := w.Check()
		if err != nil {
			onError(err)
		}
		for _, k := range kills {
			onKill(k)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
