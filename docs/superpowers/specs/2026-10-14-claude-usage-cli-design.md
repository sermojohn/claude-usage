# claude-usage CLI Design

## Overview

A Go CLI tool that invokes `claude --print "/cost"` as a subprocess, parses its output, and displays current session and/or weekly usage with reset timestamps and time-until-reset countdowns. Supports one-shot and auto-polling modes with colored terminal or JSON output.

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--current-session` | bool | false | Show current session usage row |
| `--weekly-session` | bool | false | Show weekly usage row |
| `--polling-interval-seconds` | int | 0 | 0 = run once; >0 = poll every N seconds |
| `--json` | bool | false | Emit JSON instead of colored terminal output |

If neither `--current-session` nor `--weekly-session` is provided, both rows are shown.

## Project Layout

```
claude-usage/
├── main.go
├── go.mod / go.sum
├── cmd/
│   └── root.go              # cobra command, flag definitions, run loop
├── internal/
│   ├── fetcher/
│   │   └── fetcher.go       # exec claude --print "/cost", return raw string
│   ├── parser/
│   │   └── parser.go        # regex parse raw string → UsageData
│   └── display/
│       └── display.go       # render UsageData (terminal or JSON)
```

## Dependencies

- `github.com/spf13/cobra` — flag parsing and --help
- `github.com/charmbracelet/lipgloss` — styled terminal output

## Data Model

```go
type UsageEntry struct {
    Pct      int
    ResetsAt time.Time
    ResetsIn time.Duration   // computed: ResetsAt - FetchedAt
}

type UsageData struct {
    Session   *UsageEntry   // nil if not requested / absent in output
    Weekly    *UsageEntry
    FetchedAt time.Time
}
```

## Parsing

Input from `claude --print "/cost"`:

```
Current session: 50% used · resets Oct 14 at 1:10pm (Europe/Athens)
Current week (all models): 35% used · resets Oct 16 at 12pm (Europe/Athens)
```

Two regexes, one per line type. Timezone string (e.g. `Europe/Athens`) is extracted and passed to `time.LoadLocation` for correct `time.Time` parsing. `ResetsIn` is computed as `ResetsAt - time.Now()` at parse time.

## Display

### Colored terminal (default)

Two fixed-width rows, columns aligned with lipgloss:

```
Session   ████████░░  50%  resets in 2h 10m   (Oct 14  1:10pm)
Weekly    ███░░░░░░░  35%  resets in 1d 22h   (Oct 16 12:00pm)
```

Progress bar: 10 chars (`█` filled, `░` empty).  
Color thresholds: `< 50%` green · `50–80%` yellow · `> 80%` red.

### JSON (`--json`)

One JSON object per fetch, written to stdout:

```json
{
  "fetched_at": "2026-10-14T10:00:00+03:00",
  "session": { "pct": 50, "resets_at": "2026-10-14T13:10:00+03:00", "resets_in": "2h10m" },
  "weekly":  { "pct": 35, "resets_at": "2026-10-16T12:00:00+03:00", "resets_in": "46h0m" }
}
```

In polling mode with `--json`, no terminal clear — emits a newline-delimited JSON object each tick (pipe-friendly).

## Polling & Lifecycle

- `--polling-interval-seconds 0`: fetch once, print, exit 0.
- `--polling-interval-seconds N`: `time.Ticker` fires every N seconds; clears terminal (`\033[H\033[2J`) and redraws. Runs until `SIGINT`/`SIGTERM`.
- Signal handler registered on startup in polling mode; exits cleanly on Ctrl+C.

## Error Handling

- `claude` binary not in PATH → clear error message, exit 1.
- `claude --print "/cost"` exits non-zero → surface its stderr, exit 1.
- Output doesn't match expected patterns → print a one-line error only (no raw output), exit 1 — in polling mode too, since a format mismatch is not transient.
- Polling mode transient errors (e.g. `claude` exits non-zero) → print inline error line, retry on next tick (no crash).
