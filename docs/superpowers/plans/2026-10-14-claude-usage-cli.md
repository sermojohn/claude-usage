# claude-usage CLI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go CLI that runs `claude --print "/cost"` and displays current session and/or weekly usage with reset countdowns, supporting one-shot and auto-polling modes with colored terminal or JSON output.

**Architecture:** A cobra root command wires together three internal packages: `fetcher` (execs claude), `parser` (regex-extracts UsageData), and `display` (renders as lipgloss terminal or JSON). Polling is a `time.Ticker` loop in `cmd/root.go` with SIGINT/SIGTERM signal handling; each terminal tick clears and redraws; JSON ticks emit a newline-delimited object.

**Tech Stack:** Go 1.21+, `github.com/spf13/cobra`, `github.com/charmbracelet/lipgloss`

**Spec:** `docs/superpowers/specs/2026-10-14-claude-usage-cli-design.md`

**Status:** Complete

## Global Constraints

- Module name: `claude-usage`
- Go 1.21 minimum
- Only two external dependencies: `github.com/spf13/cobra`, `github.com/charmbracelet/lipgloss`
- Never read Claude internal files; all data comes exclusively from `claude --print "/cost"`
- `--current-session` and `--weekly-session` both default to shown when neither flag is provided

## Review Focus

- **Unicode middle dot (·):** The separator in the claude output is U+00B7, not an ASCII hyphen or dot; the regex must embed it literally or parsing silently fails for all lines.
- **Time formats with and without minutes:** "12pm" (no colon) and "1:10pm" (with colon) both appear; the parser must try both Go format strings or one case always errors.
- **Year rollover at year boundary:** The output has no year; a January reset parsed in December must add one year or `ResetsIn` is negative.
- **Transient polling errors must not crash:** When claude fails mid-poll (auth timeout, network error), the loop must print an inline error and continue to the next tick.
- **`--json` with polling must not clear terminal:** JSON polling emits one object per tick to stdout without the ANSI clear sequence; the clear must be gated on `!flagJSON`.

---

### Task 1: Project scaffold

**Files:**
- Create: `main.go`
- Create: `go.mod`
- Create: `cmd/root.go`

**Interfaces:**
- Produces: `cmd.Execute()` — called by main; `rootCmd` with all four flags registered; `run()` stub returning nil

- [x] **Step 1: Initialize go module**

```bash
go mod init claude-usage
```

Expected: `go.mod` created with `module claude-usage` and the current Go version.

- [x] **Step 2: Install dependencies**

```bash
go get github.com/spf13/cobra@latest
go get github.com/charmbracelet/lipgloss@latest
```

Expected: `go.sum` created; `go.mod` updated with two `require` entries.

- [x] **Step 3: Write `main.go`**

```go
package main

import "claude-usage/cmd"

func main() {
	cmd.Execute()
}
```

- [x] **Step 4: Write `cmd/root.go` scaffold**

```go
package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	flagCurrentSession      bool
	flagWeeklySession       bool
	flagPollingIntervalSecs int
	flagJSON                bool
)

var rootCmd = &cobra.Command{
	Use:   "claude-usage",
	Short: "Display Claude CLI session and weekly usage",
	RunE:  run,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolVar(&flagCurrentSession, "current-session", false, "Show current session usage")
	rootCmd.Flags().BoolVar(&flagWeeklySession, "weekly-session", false, "Show weekly usage")
	rootCmd.Flags().IntVar(&flagPollingIntervalSecs, "polling-interval-seconds", 0, "Poll every N seconds (0 = run once)")
	rootCmd.Flags().BoolVar(&flagJSON, "json", false, "Output JSON instead of colored terminal")
}

func run(cmd *cobra.Command, args []string) error {
	return nil
}
```

- [x] **Step 5: Verify build and flags**

```bash
go build -o claude-usage .
./claude-usage --help
```

Expected: `--help` output lists `--current-session`, `--weekly-session`, `--polling-interval-seconds`, and `--json`.

- [x] **Step 6: Commit**

```bash
git add main.go go.mod go.sum cmd/root.go
git commit -m "feat: scaffold cobra CLI with all flags"
```

---

### Task 2: Fetcher

**Files:**
- Create: `internal/fetcher/fetcher.go`
- Create: `internal/fetcher/fetcher_test.go`

**Interfaces:**
- Consumes: nothing
- Produces:
  - `type RunFunc func(name string, args ...string) (string, error)`
  - `func ExecRun(name string, args ...string) (string, error)` — production runner
  - `func Fetch(run RunFunc) (string, error)` — calls `run("claude", "--print", "/cost")`

- [x] **Step 1: Write failing tests**

Create `internal/fetcher/fetcher_test.go`:

```go
package fetcher_test

import (
	"errors"
	"testing"

	"claude-usage/internal/fetcher"
)

func TestFetch_CallsClaudeWithCorrectArgs(t *testing.T) {
	var gotName string
	var gotArgs []string
	mock := func(name string, args ...string) (string, error) {
		gotName = name
		gotArgs = args
		return "output", nil
	}
	_, err := fetcher.Fetch(mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotName != "claude" {
		t.Errorf("got command %q, want %q", gotName, "claude")
	}
	if len(gotArgs) != 2 || gotArgs[0] != "--print" || gotArgs[1] != "/cost" {
		t.Errorf("got args %v, want [--print /cost]", gotArgs)
	}
}

func TestFetch_ReturnsOutput(t *testing.T) {
	const want = "Current session: 50% used\n"
	mock := func(name string, args ...string) (string, error) {
		return want, nil
	}
	got, err := fetcher.Fetch(mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFetch_PropagatesError(t *testing.T) {
	mock := func(name string, args ...string) (string, error) {
		return "", errors.New("exit status 1")
	}
	_, err := fetcher.Fetch(mock)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
```

- [x] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/fetcher/... -v
```

Expected: compile error — `fetcher` package does not exist yet.

- [x] **Step 3: Implement `internal/fetcher/fetcher.go`**

```go
package fetcher

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// RunFunc executes a named command and returns its stdout as a string.
type RunFunc func(name string, args ...string) (string, error)

// ExecRun is the production RunFunc using os/exec.
func ExecRun(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("claude exited %d: %s", exitErr.ExitCode(), strings.TrimSpace(string(exitErr.Stderr)))
		}
		var execErr *exec.Error
		if errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound) {
			return "", fmt.Errorf("claude binary not found in PATH")
		}
		return "", fmt.Errorf("failed to run claude: %w", err)
	}
	return string(out), nil
}

// Fetch invokes claude --print "/cost" and returns the raw output.
func Fetch(run RunFunc) (string, error) {
	return run("claude", "--print", "/cost")
}
```

- [x] **Step 4: Run tests to confirm they pass**

```bash
go test ./internal/fetcher/... -v
```

Expected: all 3 tests PASS.

- [x] **Step 5: Commit**

```bash
git add internal/fetcher/
git commit -m "feat: add fetcher package with injectable runner"
```

---

### Task 3: Parser

**Files:**
- Create: `internal/parser/parser.go`
- Create: `internal/parser/parser_test.go`

**Interfaces:**
- Consumes: raw string from `fetcher.Fetch`
- Produces:
  - `type UsageEntry struct { Pct int; ResetsAt time.Time; ResetsIn time.Duration }`
  - `type UsageData struct { Session *UsageEntry; Weekly *UsageEntry; FetchedAt time.Time }`
  - `func Parse(raw string, wantSession, wantWeekly bool, now time.Time) (UsageData, error)`

- [x] **Step 1: Write failing tests**

Create `internal/parser/parser_test.go`:

```go
package parser_test

import (
	"testing"
	"time"

	"claude-usage/internal/parser"
)

// refNow is a fixed reference time for deterministic year inference.
var refNow = time.Date(2026, 10, 14, 10, 0, 0, 0, time.UTC)

// sampleOutput mirrors the real claude --print "/cost" output.
const sampleOutput = `You are currently using your subscription to power your Claude Code usage

Current session: 50% used · resets Oct 14 at 1:10pm (Europe/Athens)
Current week (all models): 35% used · resets Oct 16 at 12pm (Europe/Athens)`

func TestParse_BothRows(t *testing.T) {
	data, err := parser.Parse(sampleOutput, true, true, refNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data.Session == nil {
		t.Fatal("Session is nil")
	}
	if data.Weekly == nil {
		t.Fatal("Weekly is nil")
	}
	if data.Session.Pct != 50 {
		t.Errorf("Session.Pct = %d, want 50", data.Session.Pct)
	}
	if data.Weekly.Pct != 35 {
		t.Errorf("Weekly.Pct = %d, want 35", data.Weekly.Pct)
	}
}

func TestParse_SessionOnly(t *testing.T) {
	data, err := parser.Parse(sampleOutput, true, false, refNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data.Session == nil {
		t.Fatal("Session is nil")
	}
	if data.Weekly != nil {
		t.Error("Weekly should be nil when wantWeekly=false")
	}
}

func TestParse_WeeklyOnly(t *testing.T) {
	data, err := parser.Parse(sampleOutput, false, true, refNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data.Session != nil {
		t.Error("Session should be nil when wantSession=false")
	}
	if data.Weekly == nil {
		t.Fatal("Weekly is nil")
	}
}

func TestParse_TimeWithoutMinutes(t *testing.T) {
	// "12pm" has no colon — parser must handle missing minutes.
	data, err := parser.Parse(sampleOutput, false, true, refNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data.Weekly.ResetsAt.Hour() != 12 {
		t.Errorf("Weekly.ResetsAt.Hour() = %d, want 12", data.Weekly.ResetsAt.Hour())
	}
	if data.Weekly.ResetsAt.Minute() != 0 {
		t.Errorf("Weekly.ResetsAt.Minute() = %d, want 0", data.Weekly.ResetsAt.Minute())
	}
}

func TestParse_TimeWithMinutes(t *testing.T) {
	// "1:10pm" includes minutes.
	data, err := parser.Parse(sampleOutput, true, false, refNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data.Session.ResetsAt.Minute() != 10 {
		t.Errorf("Session.ResetsAt.Minute() = %d, want 10", data.Session.ResetsAt.Minute())
	}
}

func TestParse_ResetsInIsPositive(t *testing.T) {
	data, err := parser.Parse(sampleOutput, true, true, refNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data.Session.ResetsIn <= 0 {
		t.Errorf("Session.ResetsIn = %v, want positive", data.Session.ResetsIn)
	}
	if data.Weekly.ResetsIn <= 0 {
		t.Errorf("Weekly.ResetsIn = %v, want positive", data.Weekly.ResetsIn)
	}
}

func TestParse_YearRollover(t *testing.T) {
	// now is late December; reset is Jan 1 next year.
	decNow := time.Date(2026, 12, 31, 23, 0, 0, 0, time.UTC)
	const janReset = `Current session: 10% used · resets Jan 1 at 12pm (UTC)`
	data, err := parser.Parse(janReset, true, false, decNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data.Session.ResetsAt.Year() != 2027 {
		t.Errorf("ResetsAt.Year() = %d, want 2027", data.Session.ResetsAt.Year())
	}
}

func TestParse_EmptyOutput_ReturnsError(t *testing.T) {
	_, err := parser.Parse("", true, true, refNow)
	if err == nil {
		t.Fatal("expected error for empty output, got nil")
	}
}

func TestParse_MalformedOutput_ReturnsError(t *testing.T) {
	_, err := parser.Parse("some unrelated text\nno usage data here", true, true, refNow)
	if err == nil {
		t.Fatal("expected error for malformed output, got nil")
	}
}
```

- [x] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/parser/... -v
```

Expected: compile error — `parser` package does not exist yet.

- [x] **Step 3: Implement `internal/parser/parser.go`**

```go
package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	// The · separator is U+00B7 (middle dot), embedded literally.
	sessionRe = regexp.MustCompile(`Current session: (\d+)% used · resets ([A-Za-z]+ \d+ at \d+(?::\d+)?(?:am|pm)) \(([^)]+)\)`)
	weeklyRe  = regexp.MustCompile(`Current week \(all models\): (\d+)% used · resets ([A-Za-z]+ \d+ at \d+(?::\d+)?(?:am|pm)) \(([^)]+)\)`)
)

// UsageEntry holds parsed data for one usage period.
type UsageEntry struct {
	Pct      int
	ResetsAt time.Time
	ResetsIn time.Duration
}

// UsageData holds the parsed output of claude --print "/cost".
type UsageData struct {
	Session   *UsageEntry
	Weekly    *UsageEntry
	FetchedAt time.Time
}

// Parse extracts usage data from the raw output of claude --print "/cost".
// now is used to compute ResetsIn and to infer the reset year.
func Parse(raw string, wantSession, wantWeekly bool, now time.Time) (UsageData, error) {
	data := UsageData{FetchedAt: now}
	found := false

	if wantSession {
		if m := sessionRe.FindStringSubmatch(raw); m != nil {
			entry, err := parseEntry(m[1], m[2], m[3], now)
			if err != nil {
				return UsageData{}, fmt.Errorf("parsing session entry: %w", err)
			}
			data.Session = entry
			found = true
		}
	}

	if wantWeekly {
		if m := weeklyRe.FindStringSubmatch(raw); m != nil {
			entry, err := parseEntry(m[1], m[2], m[3], now)
			if err != nil {
				return UsageData{}, fmt.Errorf("parsing weekly entry: %w", err)
			}
			data.Weekly = entry
			found = true
		}
	}

	if !found {
		return UsageData{}, fmt.Errorf("no usage data found in output:\n%s", raw)
	}

	return data, nil
}

func parseEntry(pctStr, timeStr, tzName string, now time.Time) (*UsageEntry, error) {
	pct, err := strconv.Atoi(pctStr)
	if err != nil {
		return nil, fmt.Errorf("invalid percentage %q: %w", pctStr, err)
	}
	resetsAt, err := parseResetTime(timeStr, tzName, now)
	if err != nil {
		return nil, err
	}
	return &UsageEntry{
		Pct:      pct,
		ResetsAt: resetsAt,
		ResetsIn: resetsAt.Sub(now),
	}, nil
}

func parseResetTime(timeStr, tzName string, now time.Time) (time.Time, error) {
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		return time.Time{}, fmt.Errorf("unknown timezone %q: %w", tzName, err)
	}

	// Normalize lowercase "pm"/"am" to "PM"/"AM" for Go's time parser.
	normalized := timeStr[:len(timeStr)-2] + strings.ToUpper(timeStr[len(timeStr)-2:])
	withYear := fmt.Sprintf("%d %s", now.Year(), normalized)

	for _, format := range []string{
		"2006 Jan 2 at 3:04PM",
		"2006 Jan 2 at 3PM",
	} {
		t, parseErr := time.ParseInLocation(format, withYear, loc)
		if parseErr == nil {
			if t.Before(now) {
				t = t.AddDate(1, 0, 0)
			}
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("cannot parse reset time %q with timezone %q", timeStr, tzName)
}
```

- [x] **Step 4: Run tests to confirm they pass**

```bash
go test ./internal/parser/... -v
```

Expected: all 8 tests PASS.

- [x] **Step 5: Commit**

```bash
git add internal/parser/
git commit -m "feat: add parser with regex extraction and timezone-aware time parsing"
```

---

### Task 4: Display

**Files:**
- Create: `internal/display/display.go`
- Create: `internal/display/display_test.go`

**Interfaces:**
- Consumes: `parser.UsageData`, `parser.UsageEntry`
- Produces: `func Render(data parser.UsageData, asJSON bool) (string, error)`

- [x] **Step 1: Write failing tests**

Create `internal/display/display_test.go`:

```go
package display_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"claude-usage/internal/display"
	"claude-usage/internal/parser"
)

var (
	athens, _ = time.LoadLocation("Europe/Athens")
	refNow    = time.Date(2026, 10, 14, 10, 0, 0, 0, athens)
	refData   = parser.UsageData{
		FetchedAt: refNow,
		Session: &parser.UsageEntry{
			Pct:      50,
			ResetsAt: time.Date(2026, 10, 14, 13, 10, 0, 0, athens),
			ResetsIn: 3*time.Hour + 10*time.Minute,
		},
		Weekly: &parser.UsageEntry{
			Pct:      35,
			ResetsAt: time.Date(2026, 10, 16, 12, 0, 0, 0, athens),
			ResetsIn: 46 * time.Hour,
		},
	}
)

func TestRender_JSON_IsValidJSON(t *testing.T) {
	out, err := display.Render(refData, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !json.Valid([]byte(out)) {
		t.Fatalf("output is not valid JSON: %s", out)
	}
}

func TestRender_JSON_Structure(t *testing.T) {
	out, err := display.Render(refData, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result struct {
		FetchedAt string `json:"fetched_at"`
		Session   struct {
			Pct      int    `json:"pct"`
			ResetsAt string `json:"resets_at"`
			ResetsIn string `json:"resets_in"`
		} `json:"session"`
		Weekly struct {
			Pct      int    `json:"pct"`
			ResetsAt string `json:"resets_at"`
			ResetsIn string `json:"resets_in"`
		} `json:"weekly"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal error: %v\noutput: %s", err, out)
	}
	if result.Session.Pct != 50 {
		t.Errorf("session.pct = %d, want 50", result.Session.Pct)
	}
	if result.Weekly.Pct != 35 {
		t.Errorf("weekly.pct = %d, want 35", result.Weekly.Pct)
	}
	if result.Session.ResetsIn != "3h 10m" {
		t.Errorf("session.resets_in = %q, want %q", result.Session.ResetsIn, "3h 10m")
	}
	if result.Weekly.ResetsIn != "1d 22h" {
		t.Errorf("weekly.resets_in = %q, want %q", result.Weekly.ResetsIn, "1d 22h")
	}
}

func TestRender_JSON_NilWeeklyOmitted(t *testing.T) {
	data := parser.UsageData{FetchedAt: refNow, Session: refData.Session}
	out, err := display.Render(data, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out, `"weekly"`) {
		t.Errorf("JSON must not contain 'weekly' key when Weekly is nil: %s", out)
	}
}

func TestRender_Terminal_ContainsPercentages(t *testing.T) {
	out, err := display.Render(refData, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "50%") {
		t.Errorf("output missing '50%%': %q", out)
	}
	if !strings.Contains(out, "35%") {
		t.Errorf("output missing '35%%': %q", out)
	}
}

func TestRender_Terminal_ContainsProgressBarChars(t *testing.T) {
	out, err := display.Render(refData, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "█") {
		t.Errorf("output missing filled bar char █: %q", out)
	}
	if !strings.Contains(out, "░") {
		t.Errorf("output missing empty bar char ░: %q", out)
	}
}

func TestRender_Terminal_ContainsResetsIn(t *testing.T) {
	out, err := display.Render(refData, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "resets in") {
		t.Errorf("output missing 'resets in': %q", out)
	}
}

func TestRender_Terminal_DaysDuration(t *testing.T) {
	data := parser.UsageData{
		FetchedAt: refNow,
		Weekly: &parser.UsageEntry{
			Pct:      10,
			ResetsAt: refNow.Add(46 * time.Hour),
			ResetsIn: 46 * time.Hour,
		},
	}
	out, err := display.Render(data, true) // JSON to check formatted duration
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "1d 22h") {
		t.Errorf("expected '1d 22h' in output, got: %s", out)
	}
}
```

- [x] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/display/... -v
```

Expected: compile error — `display` package does not exist yet.

- [x] **Step 3: Implement `internal/display/display.go`**

```go
package display

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"claude-usage/internal/parser"
)

var (
	labelStyle = lipgloss.NewStyle().Width(10).Bold(true)
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

func colorStyle(pct int) lipgloss.Style {
	switch {
	case pct >= 80:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("9"))  // red
	case pct >= 50:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("11")) // yellow
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // green
	}
}

func progressBar(pct int) string {
	filled := pct / 10
	return strings.Repeat("█", filled) + strings.Repeat("░", 10-filled)
}

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "now"
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh", days, hours)
	}
	return fmt.Sprintf("%dh %dm", hours, mins)
}

func renderRow(label string, entry *parser.UsageEntry) string {
	bar := colorStyle(entry.Pct).Render(progressBar(entry.Pct))
	pct := fmt.Sprintf("%d%%", entry.Pct)
	resetIn := formatDuration(entry.ResetsIn)
	resetAt := entry.ResetsAt.Format("Jan 2 3:04pm")
	return fmt.Sprintf("%s %s  %-5s  resets in %-8s  %s\n",
		labelStyle.Render(label),
		bar,
		pct,
		resetIn,
		dimStyle.Render("("+resetAt+")"),
	)
}

type jsonEntry struct {
	Pct      int    `json:"pct"`
	ResetsAt string `json:"resets_at"`
	ResetsIn string `json:"resets_in"`
}

type jsonOutput struct {
	FetchedAt string     `json:"fetched_at"`
	Session   *jsonEntry `json:"session,omitempty"`
	Weekly    *jsonEntry `json:"weekly,omitempty"`
}

func toJSONEntry(e *parser.UsageEntry) *jsonEntry {
	if e == nil {
		return nil
	}
	return &jsonEntry{
		Pct:      e.Pct,
		ResetsAt: e.ResetsAt.Format(time.RFC3339),
		ResetsIn: formatDuration(e.ResetsIn),
	}
}

// Render formats UsageData as a colored terminal string or a JSON object.
func Render(data parser.UsageData, asJSON bool) (string, error) {
	if asJSON {
		out := jsonOutput{
			FetchedAt: data.FetchedAt.Format(time.RFC3339),
			Session:   toJSONEntry(data.Session),
			Weekly:    toJSONEntry(data.Weekly),
		}
		b, err := json.Marshal(out)
		if err != nil {
			return "", fmt.Errorf("marshalling JSON: %w", err)
		}
		return string(b), nil
	}

	var sb strings.Builder
	if data.Session != nil {
		sb.WriteString(renderRow("Session", data.Session))
	}
	if data.Weekly != nil {
		sb.WriteString(renderRow("Weekly", data.Weekly))
	}
	return sb.String(), nil
}
```

- [x] **Step 4: Run tests to confirm they pass**

```bash
go test ./internal/display/... -v
```

Expected: all 7 tests PASS.

- [x] **Step 5: Commit**

```bash
git add internal/display/
git commit -m "feat: add display package with terminal and JSON rendering"
```

---

### Task 5: Wire up cmd/root.go and smoke test

**Files:**
- Modify: `cmd/root.go` (replace the `run` stub with the full implementation)

**Interfaces:**
- Consumes: `fetcher.Fetch`, `fetcher.ExecRun`, `parser.Parse`, `display.Render`
- Produces: working binary for all flags and polling modes

- [x] **Step 1: Replace the run stub in `cmd/root.go`**

The full `cmd/root.go` (replace the entire file):

```go
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"claude-usage/internal/display"
	"claude-usage/internal/fetcher"
	"claude-usage/internal/parser"
)

var (
	flagCurrentSession      bool
	flagWeeklySession       bool
	flagPollingIntervalSecs int
	flagJSON                bool
)

var rootCmd = &cobra.Command{
	Use:   "claude-usage",
	Short: "Display Claude CLI session and weekly usage",
	RunE:  run,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolVar(&flagCurrentSession, "current-session", false, "Show current session usage")
	rootCmd.Flags().BoolVar(&flagWeeklySession, "weekly-session", false, "Show weekly usage")
	rootCmd.Flags().IntVar(&flagPollingIntervalSecs, "polling-interval-seconds", 0, "Poll every N seconds (0 = run once)")
	rootCmd.Flags().BoolVar(&flagJSON, "json", false, "Output JSON instead of colored terminal")
}

func run(cmd *cobra.Command, args []string) error {
	showSession := flagCurrentSession
	showWeekly := flagWeeklySession
	if !showSession && !showWeekly {
		showSession = true
		showWeekly = true
	}

	doFetch := func() error {
		raw, err := fetcher.Fetch(fetcher.ExecRun)
		if err != nil {
			return err
		}
		now := time.Now()
		data, err := parser.Parse(raw, showSession, showWeekly, now)
		if err != nil {
			return err
		}
		out, err := display.Render(data, flagJSON)
		if err != nil {
			return err
		}
		fmt.Print(out)
		return nil
	}

	if flagPollingIntervalSecs == 0 {
		return doFetch()
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	clearScreen()
	if err := doFetch(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	}

	ticker := time.NewTicker(time.Duration(flagPollingIntervalSecs) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !flagJSON {
				clearScreen()
			}
			if err := doFetch(); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func clearScreen() {
	fmt.Print("\033[H\033[2J")
}
```

- [x] **Step 2: Build**

```bash
go build -o claude-usage .
```

Expected: compiles with no errors.

- [x] **Step 3: Run all tests**

```bash
go test ./... -v
```

Expected: all tests in all packages PASS.

- [x] **Step 4: Smoke test — both rows (default)**

```bash
./claude-usage
```

Expected: two rows with progress bars, percentages, and reset countdowns for Session and Weekly.

- [x] **Step 5: Smoke test — session only**

```bash
./claude-usage --current-session
```

Expected: only the Session row.

- [x] **Step 6: Smoke test — weekly only**

```bash
./claude-usage --weekly-session
```

Expected: only the Weekly row.

- [x] **Step 7: Smoke test — JSON output**

```bash
./claude-usage --json
```

Expected: valid JSON printed with `fetched_at`, `session`, and `weekly` keys.

- [x] **Step 8: Smoke test — polling mode**

```bash
./claude-usage --polling-interval-seconds 5
```

Expected: display redraws in place every 5 seconds. Press Ctrl+C — exits cleanly with no error.

- [x] **Step 9: Smoke test — JSON polling (no clear)**

```bash
./claude-usage --json --polling-interval-seconds 3
```

Expected: a new JSON object printed every 3 seconds without clearing the terminal. Ctrl+C exits cleanly.

- [x] **Step 10: Final commit**

```bash
git add cmd/root.go
git commit -m "feat: wire up run loop with polling and signal handling"
```
