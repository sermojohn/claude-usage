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

func TestRender_Terminal_OverLimitDoesNotPanic(t *testing.T) {
	data := parser.UsageData{
		FetchedAt: refNow,
		Session: &parser.UsageEntry{
			Pct:      120,
			ResetsAt: refNow.Add(time.Hour),
			ResetsIn: time.Hour,
		},
	}
	out, err := display.Render(data, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "120%") {
		t.Errorf("output missing '120%%': %q", out)
	}
}
