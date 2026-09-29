package darwin

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// LaunchAgent is a per-user launchd job that starts at login and is
// restarted if it exits.
type LaunchAgent struct {
	Label   string
	Program []string
	LogPath string
}

func (a LaunchAgent) Plist() string {
	var args strings.Builder
	for _, arg := range a.Program {
		fmt.Fprintf(&args, "\n\t\t<string>%s</string>", escape(arg))
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>%s
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>ProcessType</key>
	<string>Background</string>
	<key>StandardOutPath</key>
	<string>%s</string>
	<key>StandardErrorPath</key>
	<string>%s</string>
</dict>
</plist>
`, escape(a.Label), args.String(), escape(a.LogPath), escape(a.LogPath))
}

func escape(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// PlistPath is where the agent's definition lives for the current user.
func (a LaunchAgent) PlistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", a.Label+".plist"), nil
}

// Install writes the agent definition and (re)starts it.
func (a LaunchAgent) Install() error {
	path, err := a.PlistPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(a.LogPath), 0o755); err != nil {
		return err
	}
	// Stop a previously installed version so the new definition takes effect.
	_ = launchctl("bootout", a.target())
	if err := os.WriteFile(path, []byte(a.Plist()), 0o644); err != nil {
		return err
	}
	return launchctl("bootstrap", a.domain(), path)
}

// Uninstall stops the agent and removes its definition.
func (a LaunchAgent) Uninstall() error {
	path, err := a.PlistPath()
	if err != nil {
		return err
	}
	_ = launchctl("bootout", a.target())
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (a LaunchAgent) domain() string { return fmt.Sprintf("gui/%d", os.Getuid()) }
func (a LaunchAgent) target() string { return a.domain() + "/" + a.Label }

func launchctl(args ...string) error {
	out, err := exec.Command("launchctl", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl %s: %w: %s", strings.Join(args, " "), err, bytes.TrimSpace(out))
	}
	return nil
}
