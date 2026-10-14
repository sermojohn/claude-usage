package cmd

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func runPoll(t *testing.T, ticks int, clearEnabled bool, tick func() error) (clears int, stderr string) {
	t.Helper()
	ch := make(chan time.Time)
	ctx, cancel := context.WithCancel(context.Background())
	var errBuf bytes.Buffer
	done := make(chan struct{})
	go func() {
		poll(ctx, ch, tick, func() { clears++ }, clearEnabled, &errBuf)
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
