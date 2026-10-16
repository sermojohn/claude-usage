package fetcher

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

// costRequest is one stream-json user turn asking for /cost.
const costRequest = `{"type":"user","message":{"role":"user","content":"/cost"}}` + "\n"

// Process is a running claude process speaking stream-json on stdin/stdout.
type Process struct {
	Stdin  io.WriteCloser
	Stdout io.Reader
	// Stop ends the process and releases its resources.
	Stop func() error
}

// StartFunc launches a new stream-json claude process.
type StartFunc func() (*Process, error)

// ExecStart is the production StartFunc: one long-lived claude process that
// reads user turns from stdin and writes events to stdout.
func ExecStart() (*Process, error) {
	cmd := exec.Command("claude", "--print",
		"--input-format", "stream-json",
		"--output-format", "stream-json",
		"--verbose")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, fmt.Errorf("claude binary not found in PATH")
		}
		return nil, fmt.Errorf("failed to start claude: %w", err)
	}
	stop := func() error {
		stdin.Close()
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case err := <-done:
			if err != nil && stderr.Len() > 0 {
				return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
			}
			return err
		case <-time.After(2 * time.Second):
			cmd.Process.Kill()
			return <-done
		}
	}
	return &Process{Stdin: stdin, Stdout: stdout, Stop: stop}, nil
}

// Session keeps one claude process open and asks it for /cost on every
// Fetch, so polling pays claude's startup cost once instead of per refresh.
// A process that exits, hangs or fails is stopped, and the next Fetch starts
// a fresh one. Fetch must not be called concurrently.
type Session struct {
	start   StartFunc
	timeout time.Duration

	proc    *Process
	results chan resultOrErr
	done    chan struct{} // closed when proc is stopped
}

type resultOrErr struct {
	text string
	err  error
}

// NewSession returns a Session that starts processes with start and gives
// each /cost turn up to timeout to answer.
func NewSession(start StartFunc, timeout time.Duration) *Session {
	return &Session{start: start, timeout: timeout}
}

// Fetch asks the running process for /cost, starting one if needed, and
// returns the result text.
func (s *Session) Fetch(ctx context.Context) (string, error) {
	if s.proc == nil {
		proc, err := s.start()
		if err != nil {
			return "", err
		}
		s.proc = proc
		s.results = make(chan resultOrErr)
		s.done = make(chan struct{})
		go readResults(proc.Stdout, s.results, s.done)
	}

	if _, err := io.WriteString(s.proc.Stdin, costRequest); err != nil {
		return "", s.fail(fmt.Errorf("claude session: write failed: %w", err))
	}

	timer := time.NewTimer(s.timeout)
	defer timer.Stop()
	select {
	case r := <-s.results:
		if r.err != nil {
			return "", s.fail(r.err)
		}
		return r.text, nil
	case <-timer.C:
		return "", s.fail(fmt.Errorf("claude session: timed out after %s waiting for /cost", s.timeout))
	case <-ctx.Done():
		return "", s.fail(ctx.Err())
	}
}

// Close stops the running process, if any.
func (s *Session) Close() error {
	if s.proc == nil {
		return nil
	}
	close(s.done)
	err := s.proc.Stop()
	s.proc = nil
	return err
}

// fail stops the current process so the next Fetch starts a new one, and
// returns err, with the process's exit error attached when there is one.
func (s *Session) fail(err error) error {
	if stopErr := s.Close(); stopErr != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("%w (%v)", err, stopErr)
	}
	return err
}

// readResults sends one value to results per result event read from r, and
// a final error when r ends. It returns as soon as done is closed.
func readResults(r io.Reader, results chan<- resultOrErr, done <-chan struct{}) {
	send := func(v resultOrErr) bool {
		select {
		case results <- v:
			return true
		case <-done:
			return false
		}
	}
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			var ev struct {
				Type    string `json:"type"`
				IsError bool   `json:"is_error"`
				Result  string `json:"result"`
			}
			if json.Unmarshal(line, &ev) == nil && ev.Type == "result" {
				r := resultOrErr{text: ev.Result}
				if ev.IsError {
					r = resultOrErr{err: fmt.Errorf("claude session: %s", strings.TrimSpace(ev.Result))}
				}
				if !send(r) {
					return
				}
			}
		}
		if err != nil {
			send(resultOrErr{err: errors.New("claude session exited")})
			return
		}
	}
}
