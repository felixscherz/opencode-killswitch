# opencode-killswitch

Kills opencode when it uses more than 16GB of memory, before a leak eats all
your RAM. You can change the limit. macOS only.

## Quick start

You need Go and the Xcode Command Line Tools (`xcode-select --install`).

```sh
git clone https://github.com/felixscherz/opencode-killswitch.git
cd opencode-killswitch
go build -o bin/opencode-killswitch ./cmd/opencode-killswitch
./bin/opencode-killswitch watch
```

Leave that terminal open. It checks every 2 seconds and prints a line when it
kills something. Stop it with Ctrl-C.

Pick a different limit with `--limit`:

```sh
./bin/opencode-killswitch watch --limit 8GB
```

See what opencode is using right now:

```sh
./bin/opencode-killswitch status
```

## Run it in the background

`watch` only protects you while its terminal is open. To have it start at
login and restart if it crashes, install it as a launchd agent:

```sh
./bin/opencode-killswitch install --limit 16GB
```

Logs go to `~/Library/Logs/opencode-killswitch.log`. Remove the agent with:

```sh
./bin/opencode-killswitch uninstall
```

The agent runs the binary from where it was when you installed it. If you
move or rebuild it somewhere else, run `install` again.

## How it works

It reads the same memory number Activity Monitor shows in its Memory column.
That number counts compressed and swapped memory, so it can go past your
physical RAM. A leaking opencode can show 200GB on a 64GB Mac.

When a process goes over the limit it gets SIGTERM, then SIGKILL if it is
still running 5 seconds later. A macOS notification tells you it happened.
The limit is per process, so your other opencode sessions keep running.

## Flags

These work with `watch` and `install`.

| Flag | Default | Meaning |
| --- | --- | --- |
| `--limit` | `16GB` | Kill a process using more than this. 1GB = 1024^3 bytes, like Activity Monitor. |
| `--name` | `opencode` | Executable name to watch. Repeat it to watch several. |
| `--interval` | `2s` | How often to check. |
| `--grace` | `5s` | Wait between SIGTERM and SIGKILL. |
| `--dry-run` | off | Log what would be killed and kill nothing. |
| `--no-notify` | off | Skip the notification. |
