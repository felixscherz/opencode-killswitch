package darwin

import "os/exec"

// Notify shows a macOS notification. The text is passed as arguments rather
// than interpolated into the script, so it needs no escaping.
func Notify(title, message string) error {
	return exec.Command("osascript",
		"-e", "on run argv",
		"-e", "display notification (item 2 of argv) with title (item 1 of argv)",
		"-e", "end run",
		title, message,
	).Run()
}
