// Command opencode-killswitch kills opencode when its memory footprint
// exceeds a limit.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/felixscherz/opencode-killswitch/internal/darwin"
	"github.com/felixscherz/opencode-killswitch/internal/killswitch"
)

const (
	defaultLimit = "16GB"
	agentLabel   = "com.github.felixscherz.opencode-killswitch"
)

const usage = `opencode-killswitch kills opencode when its memory footprint exceeds a limit.

Memory is measured as the physical footprint, the value Activity Monitor shows
in its Memory column. Sizes use binary units (1GB = 1024^3 bytes).

Usage:
  opencode-killswitch watch [flags]     watch in the foreground
  opencode-killswitch status [flags]    show watched processes and their memory
  opencode-killswitch install [flags]   run 'watch' in the background at login (launchd)
  opencode-killswitch uninstall         remove the background agent

Run 'opencode-killswitch <command> -h' for the flags of a command.
`

func main() {
	log.SetFlags(log.LstdFlags)
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	var err error
	switch cmd, args := os.Args[1], os.Args[2:]; cmd {
	case "watch":
		err = watch(args)
	case "status":
		err = status(args)
	case "install":
		err = install(args)
	case "uninstall":
		err = uninstall()
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", cmd, usage)
		os.Exit(2)
	}
	if err != nil && !errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

type watchOptions struct {
	policy   killswitch.Policy
	interval time.Duration
	grace    time.Duration
	dryRun   bool
	notify   bool
}

func parseWatchFlags(name string, args []string) (watchOptions, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	limit := fs.String("limit", defaultLimit, "kill a watched process whose memory exceeds this size, e.g. 16GB, 8GiB, 512MB")
	var names nameList
	fs.Var(&names, "name", "executable name to watch, repeatable (default opencode)")
	interval := fs.Duration("interval", 2*time.Second, "how often to check memory")
	grace := fs.Duration("grace", 5*time.Second, "time to wait after SIGTERM before sending SIGKILL")
	dryRun := fs.Bool("dry-run", false, "only report processes that would be killed")
	noNotify := fs.Bool("no-notify", false, "do not show a macOS notification when killing a process")
	if err := fs.Parse(args); err != nil {
		return watchOptions{}, err
	}
	if fs.NArg() > 0 {
		return watchOptions{}, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}
	parsedLimit, err := killswitch.ParseBytes(*limit)
	if err != nil {
		return watchOptions{}, err
	}
	if *interval <= 0 {
		return watchOptions{}, fmt.Errorf("-interval must be positive")
	}
	if len(names) == 0 {
		names = nameList{"opencode"}
	}
	return watchOptions{
		policy:   killswitch.Policy{Limit: parsedLimit, Names: names},
		interval: *interval,
		grace:    *grace,
		dryRun:   *dryRun,
		notify:   !*noNotify,
	}, nil
}

func watch(args []string) error {
	opts, err := parseWatchFlags("watch", args)
	if err != nil {
		return err
	}
	var terminator killswitch.Terminator = darwin.Terminator{Grace: opts.grace}
	if opts.dryRun {
		terminator = dryRun{}
	}
	w := killswitch.Watchdog{Policy: opts.policy, Source: darwin.Processes{}, Terminator: terminator}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log.Printf("watching %s, limit %s, checking every %s%s",
		strings.Join(opts.policy.Names, ", "), opts.policy.Limit, opts.interval, map[bool]string{true: " (dry run)"}[opts.dryRun])

	err = w.Run(ctx, opts.interval, func(k killswitch.Kill) { reportKill(k, opts) }, func(err error) { log.Print(err) })
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func reportKill(k killswitch.Kill, opts watchOptions) {
	p := k.Process
	switch {
	case opts.dryRun:
		log.Printf("would kill %s (pid %d): using %s, limit %s", p.Name, p.PID, p.Footprint, opts.policy.Limit)
		return
	case k.Err != nil:
		log.Printf("failed to kill %s (pid %d) using %s: %v", p.Name, p.PID, p.Footprint, k.Err)
		return
	}
	log.Printf("killed %s (pid %d): using %s, limit %s", p.Name, p.PID, p.Footprint, opts.policy.Limit)
	if opts.notify {
		msg := fmt.Sprintf("Killed %s (pid %d) using %s, over the %s limit.", p.Name, p.PID, p.Footprint, opts.policy.Limit)
		if err := darwin.Notify("opencode-killswitch", msg); err != nil {
			log.Printf("showing notification: %v", err)
		}
	}
}

type dryRun struct{}

func (dryRun) Terminate(killswitch.Process) error { return nil }

func status(args []string) error {
	opts, err := parseWatchFlags("status", args)
	if err != nil {
		return err
	}
	procs, err := darwin.Processes{}.Processes()
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "PID\tNAME\tMEMORY\tSTATE")
	found := false
	for _, p := range procs {
		if !slices.Contains(opts.policy.Names, p.Name) {
			continue
		}
		found = true
		state := "ok"
		if opts.policy.Exceeds(p) {
			state = "over limit"
		}
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", p.PID, p.Name, p.Footprint, state)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if !found {
		fmt.Printf("no running %s process\n", strings.Join(opts.policy.Names, "/"))
	}
	fmt.Printf("limit: %s\n", opts.policy.Limit)
	return nil
}

func agent(watchArgs []string) (darwin.LaunchAgent, error) {
	exe, err := os.Executable()
	if err != nil {
		return darwin.LaunchAgent{}, err
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return darwin.LaunchAgent{}, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return darwin.LaunchAgent{}, err
	}
	return darwin.LaunchAgent{
		Label:   agentLabel,
		Program: append([]string{exe, "watch"}, watchArgs...),
		LogPath: filepath.Join(home, "Library", "Logs", "opencode-killswitch.log"),
	}, nil
}

func install(args []string) error {
	// Validate the flags now rather than leaving launchd to restart a
	// failing agent.
	if _, err := parseWatchFlags("install", args); err != nil {
		return err
	}
	a, err := agent(args)
	if err != nil {
		return err
	}
	if err := a.Install(); err != nil {
		return err
	}
	path, _ := a.PlistPath()
	fmt.Printf("installed %s\nrunning: %s\nlogs:    %s\n", path, strings.Join(a.Program, " "), a.LogPath)
	return nil
}

func uninstall() error {
	a, err := agent(nil)
	if err != nil {
		return err
	}
	if err := a.Uninstall(); err != nil {
		return err
	}
	fmt.Println("uninstalled", agentLabel)
	return nil
}

type nameList []string

func (n *nameList) String() string { return strings.Join(*n, ",") }

func (n *nameList) Set(v string) error {
	if v == "" {
		return errors.New("name must not be empty")
	}
	*n = append(*n, v)
	return nil
}
