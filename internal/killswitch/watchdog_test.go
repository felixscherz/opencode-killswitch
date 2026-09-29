package killswitch

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
)

type fakeSource struct {
	snapshots [][]Process
	err       error
	calls     int
}

func (s *fakeSource) Processes() ([]Process, error) {
	if s.err != nil {
		return nil, s.err
	}
	snapshot := s.snapshots[min(s.calls, len(s.snapshots)-1)]
	s.calls++
	return snapshot, nil
}

type fakeTerminator struct {
	terminated []PID
	failFor    map[PID]error
}

func (t *fakeTerminator) Terminate(p Process) error {
	if err := t.failFor[p.PID]; err != nil {
		return err
	}
	t.terminated = append(t.terminated, p.PID)
	return nil
}

func policy() Policy {
	return Policy{Limit: 16 * GiB, Names: []string{"opencode"}}
}

func TestOpencodeOverTheLimitIsTerminated(t *testing.T) {
	source := &fakeSource{snapshots: [][]Process{{
		{PID: 65012, Name: "opencode", Footprint: 246 * GiB},
	}}}
	terminator := &fakeTerminator{}
	w := Watchdog{Policy: policy(), Source: source, Terminator: terminator}

	kills, err := w.Check()

	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(terminator.terminated, []PID{65012}) {
		t.Errorf("terminated = %v, want [65012]", terminator.terminated)
	}
	if len(kills) != 1 || kills[0].Process.PID != 65012 || kills[0].Err != nil {
		t.Errorf("kills = %+v, want one successful kill of 65012", kills)
	}
}

func TestOnlyTheLeakingOpencodeProcessIsTerminated(t *testing.T) {
	source := &fakeSource{snapshots: [][]Process{{
		{PID: 1, Name: "opencode", Footprint: 1 * GiB},
		{PID: 2, Name: "opencode", Footprint: 20 * GiB},
		{PID: 3, Name: "opencode", Footprint: 300 * MiB},
	}}}
	terminator := &fakeTerminator{}
	w := Watchdog{Policy: policy(), Source: source, Terminator: terminator}

	if _, err := w.Check(); err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(terminator.terminated, []PID{2}) {
		t.Errorf("terminated = %v, want [2]", terminator.terminated)
	}
}

func TestOtherProcessesOverTheLimitAreLeftAlone(t *testing.T) {
	source := &fakeSource{snapshots: [][]Process{{
		{PID: 1, Name: "Microsoft Teams", Footprint: 64 * GiB},
		{PID: 2, Name: "opencode-helper", Footprint: 64 * GiB},
	}}}
	terminator := &fakeTerminator{}
	w := Watchdog{Policy: policy(), Source: source, Terminator: terminator}

	kills, err := w.Check()

	if err != nil {
		t.Fatal(err)
	}
	if len(kills) != 0 || len(terminator.terminated) != 0 {
		t.Errorf("expected nothing terminated, got kills=%+v terminated=%v", kills, terminator.terminated)
	}
}

func TestProcessExactlyAtTheLimitIsTolerated(t *testing.T) {
	source := &fakeSource{snapshots: [][]Process{{
		{PID: 1, Name: "opencode", Footprint: 16 * GiB},
	}}}
	terminator := &fakeTerminator{}
	w := Watchdog{Policy: policy(), Source: source, Terminator: terminator}

	if _, err := w.Check(); err != nil {
		t.Fatal(err)
	}

	if len(terminator.terminated) != 0 {
		t.Errorf("terminated = %v, want none", terminator.terminated)
	}
}

func TestAdditionalProcessNamesCanBeWatched(t *testing.T) {
	source := &fakeSource{snapshots: [][]Process{{
		{PID: 1, Name: "opencode", Footprint: 32 * GiB},
		{PID: 2, Name: "bun", Footprint: 32 * GiB},
		{PID: 3, Name: "node", Footprint: 32 * GiB},
	}}}
	terminator := &fakeTerminator{}
	p := Policy{Limit: 16 * GiB, Names: []string{"opencode", "bun"}}
	w := Watchdog{Policy: p, Source: source, Terminator: terminator}

	if _, err := w.Check(); err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(terminator.terminated, []PID{1, 2}) {
		t.Errorf("terminated = %v, want [1 2]", terminator.terminated)
	}
}

func TestFailedTerminationIsReportedAndOthersStillTerminated(t *testing.T) {
	source := &fakeSource{snapshots: [][]Process{{
		{PID: 1, Name: "opencode", Footprint: 32 * GiB},
		{PID: 2, Name: "opencode", Footprint: 32 * GiB},
	}}}
	denied := errors.New("operation not permitted")
	terminator := &fakeTerminator{failFor: map[PID]error{1: denied}}
	w := Watchdog{Policy: policy(), Source: source, Terminator: terminator}

	kills, err := w.Check()

	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(terminator.terminated, []PID{2}) {
		t.Errorf("terminated = %v, want [2]", terminator.terminated)
	}
	if len(kills) != 2 || !errors.Is(kills[0].Err, denied) || kills[1].Err != nil {
		t.Errorf("kills = %+v, want failure for 1 and success for 2", kills)
	}
}

func TestListingFailureIsReturned(t *testing.T) {
	listErr := errors.New("proc_listallpids failed")
	w := Watchdog{Policy: policy(), Source: &fakeSource{err: listErr}, Terminator: &fakeTerminator{}}

	if _, err := w.Check(); !errors.Is(err, listErr) {
		t.Errorf("err = %v, want %v", err, listErr)
	}
}

func TestRunReportsEachKillUntilCancelled(t *testing.T) {
	source := &fakeSource{snapshots: [][]Process{
		{{PID: 1, Name: "opencode", Footprint: 2 * GiB}},
		{{PID: 1, Name: "opencode", Footprint: 40 * GiB}},
		{{PID: 2, Name: "opencode", Footprint: 1 * GiB}},
	}}
	terminator := &fakeTerminator{}
	w := Watchdog{Policy: policy(), Source: source, Terminator: terminator}
	ctx, cancel := context.WithCancel(context.Background())
	var reported []Kill

	err := w.Run(ctx, time.Millisecond, func(k Kill) {
		reported = append(reported, k)
		cancel()
	}, func(error) {})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if len(reported) != 1 || reported[0].Process.PID != 1 {
		t.Errorf("reported = %+v, want one kill of pid 1", reported)
	}
}

func TestRunKeepsWatchingAfterAListingFailure(t *testing.T) {
	source := &fakeSource{err: errors.New("transient")}
	w := Watchdog{Policy: policy(), Source: source, Terminator: &fakeTerminator{}}
	ctx, cancel := context.WithCancel(context.Background())
	failures := 0

	_ = w.Run(ctx, time.Millisecond, func(Kill) {}, func(error) {
		failures++
		if failures == 3 {
			cancel()
		}
	})

	if failures != 3 {
		t.Errorf("failures = %d, want 3", failures)
	}
}
