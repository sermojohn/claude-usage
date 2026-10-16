package fetcher_test

import (
	"bufio"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"claude-usage/internal/fetcher"
)

// fakeClaude simulates a stream-json claude process. For every line it reads
// on stdin it calls respond, which writes reply lines to stdout.
type fakeClaude struct {
	requests []string
	stopped  bool
}

func (f *fakeClaude) start(respond func(n int, out io.Writer) bool) fetcher.StartFunc {
	return func() (*fetcher.Process, error) {
		inR, inW := io.Pipe()
		outR, outW := io.Pipe()
		go func() {
			defer outW.Close()
			r := bufio.NewReader(inR)
			for n := 1; ; n++ {
				line, err := r.ReadString('\n')
				if err != nil {
					return
				}
				f.requests = append(f.requests, line)
				if !respond(n, outW) {
					return
				}
			}
		}()
		return &fetcher.Process{
			Stdin:  inW,
			Stdout: outR,
			Stop: func() error {
				f.stopped = true
				inW.Close()
				inR.Close()
				outR.Close()
				return nil
			},
		}, nil
	}
}

func reply(out io.Writer, lines ...string) {
	for _, l := range lines {
		io.WriteString(out, l+"\n")
	}
}

const resultLine = `{"type":"result","is_error":false,"result":"Current session: 40% used"}`

func TestSession_SendsCostAndReturnsResultSkippingOtherEvents(t *testing.T) {
	f := &fakeClaude{}
	s := fetcher.NewSession(f.start(func(n int, out io.Writer) bool {
		reply(out,
			`{"type":"system","subtype":"init"}`,
			`{"type":"assistant","message":{}}`,
			resultLine)
		return true
	}), time.Second)
	defer s.Close()

	got, err := s.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Current session: 40% used" {
		t.Errorf("got %q", got)
	}
	want := `{"type":"user","message":{"role":"user","content":"/cost"}}` + "\n"
	if len(f.requests) != 1 || f.requests[0] != want {
		t.Errorf("requests = %q, want [%q]", f.requests, want)
	}
}

func TestSession_ReusesOneProcessAcrossFetches(t *testing.T) {
	f := &fakeClaude{}
	starts := 0
	start := f.start(func(n int, out io.Writer) bool { reply(out, resultLine); return true })
	s := fetcher.NewSession(func() (*fetcher.Process, error) { starts++; return start() }, time.Second)
	defer s.Close()

	for i := 0; i < 3; i++ {
		if _, err := s.Fetch(context.Background()); err != nil {
			t.Fatalf("fetch %d: %v", i, err)
		}
	}
	if starts != 1 {
		t.Errorf("started %d processes, want 1", starts)
	}
}

func TestSession_ErrorResult(t *testing.T) {
	f := &fakeClaude{}
	s := fetcher.NewSession(f.start(func(n int, out io.Writer) bool {
		reply(out, `{"type":"result","is_error":true,"result":"not logged in"}`)
		return true
	}), time.Second)
	defer s.Close()

	_, err := s.Fetch(context.Background())
	if err == nil || !strings.Contains(err.Error(), "not logged in") {
		t.Fatalf("got %v, want error containing %q", err, "not logged in")
	}
}

func TestSession_ProcessExitIsErrorAndNextFetchRestarts(t *testing.T) {
	f := &fakeClaude{}
	starts := 0
	start := f.start(func(n int, out io.Writer) bool {
		if starts == 1 {
			return false // first process dies without answering
		}
		reply(out, resultLine)
		return true
	})
	s := fetcher.NewSession(func() (*fetcher.Process, error) { starts++; return start() }, time.Second)
	defer s.Close()

	if _, err := s.Fetch(context.Background()); err == nil {
		t.Fatal("expected error when the process exits, got nil")
	}
	if !f.stopped {
		t.Error("dead process was not stopped")
	}
	if _, err := s.Fetch(context.Background()); err != nil {
		t.Fatalf("second fetch: %v", err)
	}
	if starts != 2 {
		t.Errorf("started %d processes, want 2", starts)
	}
}

func TestSession_TimeoutStopsProcess(t *testing.T) {
	f := &fakeClaude{}
	s := fetcher.NewSession(f.start(func(n int, out io.Writer) bool { return true }), 20*time.Millisecond)
	defer s.Close()

	_, err := s.Fetch(context.Background())
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("got %v, want timeout error", err)
	}
	if !f.stopped {
		t.Error("hung process was not stopped")
	}
}

func TestSession_ContextCancelStopsWaiting(t *testing.T) {
	f := &fakeClaude{}
	s := fetcher.NewSession(f.start(func(n int, out io.Writer) bool { return true }), time.Minute)
	defer s.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Fetch(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context.Canceled", err)
	}
}

func TestSession_StartError(t *testing.T) {
	s := fetcher.NewSession(func() (*fetcher.Process, error) {
		return nil, errors.New("claude binary not found in PATH")
	}, time.Second)
	defer s.Close()

	if _, err := s.Fetch(context.Background()); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSession_CloseStopsProcess(t *testing.T) {
	f := &fakeClaude{}
	s := fetcher.NewSession(f.start(func(n int, out io.Writer) bool { reply(out, resultLine); return true }), time.Second)
	if _, err := s.Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if !f.stopped {
		t.Error("Close did not stop the process")
	}
}

func TestSession_CloseWithoutFetchIsNoop(t *testing.T) {
	s := fetcher.NewSession(func() (*fetcher.Process, error) {
		t.Fatal("start must not be called")
		return nil, nil
	}, time.Second)
	s.Close()
}
