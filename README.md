# claude-usage

A small Go CLI that shows your Claude CLI usage: the current session and the
weekly limit, with progress bars, reset times and a countdown.

It runs `claude --print "/cost"` under the hood, so the [Claude CLI](https://claude.com/claude-code)
must be installed and logged in.

## Install

Requires Go 1.21+.

```sh
git clone git@github.com:sermojohn/claude-usage.git
cd claude-usage
go build -o claude-usage .
```

Move the `claude-usage` binary somewhere on your `PATH` if you want to run it from anywhere.

## Usage

```sh
claude-usage [flags]
```

| Flag | Description |
|------|-------------|
| `--current-session` | Show only the current session row |
| `--weekly-session` | Show only the weekly row |
| `--polling-interval-seconds N` | Refresh every N seconds (default `0` = run once) |
| `--json` | Print JSON instead of colored output |

With neither `--current-session` nor `--weekly-session`, both rows are shown.

## Examples

Show both rows once:

```sh
$ claude-usage
Session    █████░░░░░  50%    resets in 2h 10m    (Oct 14 1:10pm)
Weekly     ███░░░░░░░  35%    resets in 1d 22h    (Oct 16 12:00pm)
```

Bars are green below 50%, yellow from 50% and red from 80%.

Show only the weekly limit:

```sh
claude-usage --weekly-session
```

Keep a live view that refreshes every 60 seconds (stop with Ctrl+C):

```sh
claude-usage --polling-interval-seconds 60
```

Get JSON output, for example for a script or status bar:

```sh
$ claude-usage --json
{"fetched_at":"2026-10-14T11:00:00+03:00","session":{"pct":50,"resets_at":"2026-10-14T13:10:00+03:00","resets_in":"2h 10m"},"weekly":{"pct":35,"resets_at":"2026-10-16T12:00:00+03:00","resets_in":"1d 22h"}}

$ claude-usage --json --current-session | jq .session.pct
50
```

With `--json` and polling together, each refresh prints one JSON line
(newline-delimited), so the output can be piped into other tools:

```sh
claude-usage --json --polling-interval-seconds 30 | while read -r line; do
  echo "$line" | jq -r '"session \(.session.pct)%  weekly \(.weekly.pct)%"'
done
```

## Errors

Errors go to stderr. A one-shot run exits with code `1` on failure. In polling
mode, temporary errors are printed and retried on the next refresh. If
`claude` returns output the tool can't parse, polling stops and the tool
exits with code `1`.

## Development

```sh
go test ./...
```

## License

MIT, see [LICENSE](LICENSE).
