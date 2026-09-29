# opencode-killswitch

Kills opencode when its memory footprint exceeds a limit (default 16GB), so a
memory leak cannot take down the machine. macOS only.

Memory is measured as the physical footprint, the value Activity Monitor shows
in its Memory column. It includes compressed and swapped memory, unlike RSS,
which is capped by physical RAM and would miss a runaway leak.

## Build

```sh
go build -o bin/opencode-killswitch ./cmd/opencode-killswitch
```

## Use

```sh
# Show opencode processes and their memory
opencode-killswitch status

# Watch in the foreground
opencode-killswitch watch --limit 16GB

# Run in the background at login (launchd agent), logs to ~/Library/Logs/opencode-killswitch.log
opencode-killswitch install --limit 16GB
opencode-killswitch uninstall
```

`watch` and `install` flags:

| Flag | Default | Meaning |
| --- | --- | --- |
| `--limit` | `16GB` | Kill a watched process whose footprint exceeds this. Binary units: `16GB` = 16 x 1024^3 bytes. |
| `--name` | `opencode` | Executable name to watch. Repeat to watch several. |
| `--interval` | `2s` | How often to check. |
| `--grace` | `5s` | Wait after SIGTERM before sending SIGKILL. |
| `--dry-run` | off | Only log what would be killed. |
| `--no-notify` | off | Skip the macOS notification on kill. |

The limit applies to each process on its own, so only the leaking opencode
instance is killed. `install` records the binary's current path, so re-run it
after moving the binary.
