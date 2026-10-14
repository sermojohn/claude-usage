# Journal: 2026-10-14-unexpected-output-error

<!-- fr:journal kind=repro scope=debug id=8db1dc98e01d created=2026-10-14T11:51:14+00:00 -->
### 8db1dc98e01d · repro · Unexpected claude output dumps raw text; polling never exits

Fake `claude` printing API-key style output (no "Current session"/"Current week" lines). Single run: prints `Error: no usage data found in output:` followed by the full raw output, exit 1. `--polling-interval-seconds 1`: prints the same multi-line dump on every tick forever; exit 0 on SIGTERM.
