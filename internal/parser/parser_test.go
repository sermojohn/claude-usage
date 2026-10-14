package parser_test

import (
	"errors"
	"strings"
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

func TestParse_RecentlyPassedResetKeepsYear(t *testing.T) {
	// The reset passed a few minutes ago (stale output); it must not jump a year ahead.
	now := time.Date(2026, 10, 14, 13, 15, 0, 0, time.UTC)
	const stale = `Current session: 10% used · resets Oct 14 at 1:10pm (UTC)`
	data, err := parser.Parse(stale, true, false, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data.Session.ResetsAt.Year() != 2026 {
		t.Errorf("ResetsAt.Year() = %d, want 2026", data.Session.ResetsAt.Year())
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

func TestParse_UnexpectedOutput_ShortSentinelError(t *testing.T) {
	raw := "You are using an API key\nTotal cost: $0.12"
	_, err := parser.Parse(raw, true, true, refNow)
	if !errors.Is(err, parser.ErrUnexpectedOutput) {
		t.Fatalf("err = %v, want ErrUnexpectedOutput", err)
	}
	if strings.Contains(err.Error(), "Total cost") || strings.Contains(err.Error(), "\n") {
		t.Errorf("error message %q must not include the raw output", err.Error())
	}
}

func TestParse_UnparseableResetTime_IsUnexpectedOutput(t *testing.T) {
	raw := "Current session: 50% used · resets Oct 14 at 1:10pm (Mars/Olympus)"
	_, err := parser.Parse(raw, true, false, refNow)
	if !errors.Is(err, parser.ErrUnexpectedOutput) {
		t.Fatalf("err = %v, want ErrUnexpectedOutput", err)
	}
}
