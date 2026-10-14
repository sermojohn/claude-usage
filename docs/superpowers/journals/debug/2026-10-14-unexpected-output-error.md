# Journal: 2026-10-14-unexpected-output-error

<!-- fr:journal kind=repro scope=debug id=8db1dc98e01d created=2026-10-14T11:51:14+00:00 -->
### 8db1dc98e01d · repro · Unexpected claude output dumps raw text; polling never exits

Fake `claude` printing API-key style output (no "Current session"/"Current week" lines). Single run: prints `Error: no usage data found in output:` followed by the full raw output, exit 1. `--polling-interval-seconds 1`: prints the same multi-line dump on every tick forever; exit 0 on SIGTERM.

<!-- fr:journal kind=root-cause scope=debug id=c06b1add20e9 created=2026-10-14T11:51:17+00:00 -->
### c06b1add20e9 · root-cause · Parser embeds raw output in error; poll treats every error as transient

`internal/parser/parser.go` returns `fmt.Errorf("no usage data found in output:\n%s", raw)` (per spec line 105, now reversed by operator). `cmd/root.go` `poll` reports every tick error inline and never stops, so a persistent format mismatch loops forever instead of exiting 1.
