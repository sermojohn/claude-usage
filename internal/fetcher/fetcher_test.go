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
