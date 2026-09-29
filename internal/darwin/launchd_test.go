package darwin

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

func TestLaunchAgentPlistRunsTheGivenCommandAtLoginAndKeepsItAlive(t *testing.T) {
	agent := LaunchAgent{
		Label:   "com.example.killswitch",
		Program: []string{"/usr/local/bin/opencode-killswitch", "watch", "--limit", "8GB & <more>"},
		LogPath: "/tmp/killswitch.log",
	}
	path := filepath.Join(t.TempDir(), "agent.plist")
	if err := os.WriteFile(path, []byte(agent.Plist()), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command("plutil", "-convert", "json", "-o", "-", path).Output()
	if err != nil {
		t.Fatalf("plutil rejected plist: %v", err)
	}
	var parsed struct {
		Label             string
		ProgramArguments  []string
		RunAtLoad         bool
		KeepAlive         bool
		StandardOutPath   string
		StandardErrorPath string
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Label != agent.Label || !slices.Equal(parsed.ProgramArguments, agent.Program) {
		t.Errorf("parsed = %+v, want label and arguments of %+v", parsed, agent)
	}
	if !parsed.RunAtLoad || !parsed.KeepAlive {
		t.Errorf("RunAtLoad=%v KeepAlive=%v, want both true", parsed.RunAtLoad, parsed.KeepAlive)
	}
	if parsed.StandardOutPath != agent.LogPath || parsed.StandardErrorPath != agent.LogPath {
		t.Errorf("log paths = %q/%q, want %q", parsed.StandardOutPath, parsed.StandardErrorPath, agent.LogPath)
	}
}
