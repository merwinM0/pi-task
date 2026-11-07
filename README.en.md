# pi-task

[中文](README.md) | [English](README.en.md)

An independent **Go task-progress monitor**. Pi reports work and progress; run `pi-task` in another terminal to choose from the five most recently updated runs and watch live progress.

It only records and displays reported state: **no task execution, model requests, or background service**, and no changes to Pi's interface. Viewing and refreshing do not consume model tokens.

## Quick start

Requires Go 1.23+, make, and a Unix terminal. Primarily intended for Linux, SSH, and fish.

```fish
cd ~/mon/my_programs/pi-task
make install
fish_add_path --universal --prepend ~/.local/bin
pi-task
```

Installs to `~/.local/bin/pi-task` with fish completions, without sudo. Reopen an existing viewer after installing an update to use the new behavior.

## Keyboard controls

| Key | Run list | Progress view |
| --- | --- | --- |
| W / S, ↑ / ↓ | Move selection up / down | Scroll one line |
| A / D, ← / → | Move selection quickly | Page up / down |
| Enter | Open selected run | — |
| Esc | Exit | Return to list |
| Q / Ctrl+C | Exit | Exit |

Letter keys are case-insensitive. PageUp / PageDown and Home / End remain available. Non-terminal output is static; `pi-task status --once` and `--json` are also supported.

## Using with Pi

Merge the [reporting instructions](docs/AGENTS.example.md) into project or global `AGENTS.md`, preserving existing instructions. Pi reports state through `start / submit / update / add / finish`, without a plugin. Writes require an explicit run ID; human viewers do not need to remember one.

Models may miss updates, so the display reflects the last reported state, not independent verification. The monitor never silently completes unfinished steps or declares a stale run failed.

## Documentation and verification

- [Full usage, fish examples, storage, and removal](docs/USAGE.md) (Chinese)
- [Project progress and scope](READMAP.md) (Chinese)
- [Maintenance instructions and layout](AGENTS.md) (Chinese)

```fish
make check
make build
python3 -B tests/pty_smoke.py ./bin/pi-task
```

State defaults to `~/.local/state/pi-task/`; `PI_TASK_HOME` may select another absolute path. To view remote progress, run the viewer on the same server, as the same user, with the same state directory.
