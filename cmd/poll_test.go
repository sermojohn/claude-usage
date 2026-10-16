package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"claude-usage/internal/parser"
)

func runPoll(t *testing.T, ticks int, clearEnabled bool, tick func() error) (clears int, stderr string) {
	t.Helper()
	ch := make(chan time.Time)
	ctx, cancel := context.WithCancel(context.Background())
	var errBuf bytes.Buffer
	done := make(chan struct{})
	go func() {
		if err := poll(ctx, ch, tick, func() { clears++ }, clearEnabled, &errBuf); err != nil {
			t.Errorf("poll returned %v, want nil", err)
		}
		close(done)
	}()
	for i := 0; i < ticks; i++ {
		ch <- time.Now()
	}
	cancel()
	<-done
	return clears, errBuf.String()
}

func TestPoll_RunsImmediatelyAndOnEveryTick(t *testing.T) {
	calls := 0
	runPoll(t, 3, true, func() error { calls++; return nil })
	if calls != 4 {
		t.Errorf("tick called %d times, want 4 (initial + 3 ticks)", calls)
	}
}

func TestPoll_TransientErrorDoesNotStopLoop(t *testing.T) {
	calls := 0
	_, stderr := runPoll(t, 2, true, func() error {
		calls++
		if calls == 2 {
			return errors.New("claude exited 1: auth timeout")
		}
		return nil
	})
	if calls != 3 {
		t.Errorf("tick called %d times, want 3", calls)
	}
	if !strings.Contains(stderr, "auth timeout") {
		t.Errorf("stderr = %q, want inline error", stderr)
	}
}

func TestPoll_ClearsScreenInTerminalMode(t *testing.T) {
	clears, _ := runPoll(t, 2, true, func() error { return nil })
	if clears != 3 {
		t.Errorf("cleared %d times, want 3", clears)
	}
}

func TestPoll_NeverClearsInJSONMode(t *testing.T) {
	clears, _ := runPoll(t, 2, false, func() error { return nil })
	if clears != 0 {
		t.Errorf("cleared %d times in JSON mode, want 0", clears)
	}
}

func TestPoll_UnexpectedOutputStopsWithError(t *testing.T) {
	calls := 0
	var errBuf bytes.Buffer
	err := poll(context.Background(), make(chan time.Time), func() error {
		calls++
		return fmt.Errorf("%w: no usage lines", parser.ErrUnexpectedOutput)
	}, func() {}, true, &errBuf)
	if !errors.Is(err, parser.ErrUnexpectedOutput) {
		t.Fatalf("poll returned %v, want ErrUnexpectedOutput", err)
	}
	if calls != 1 {
		t.Errorf("tick called %d times, want 1", calls)
	}
	if errBuf.Len() != 0 {
		t.Errorf("stderr = %q, want nothing printed inline", errBuf.String())
	}
}

func TestPoll_ErrorAfterCancelIsNotPrinted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var errBuf bytes.Buffer
	err := poll(ctx, make(chan time.Time), func() error {
		cancel()
		return context.Canceled
	}, func() {}, true, &errBuf)
	if err != nil {
		t.Fatalf("poll returned %v, want nil", err)
	}
	if errBuf.Len() != 0 {
		t.Errorf("stderr = %q, want nothing printed after cancel", errBuf.String())
	}
}
