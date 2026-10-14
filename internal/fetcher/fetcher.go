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
			return "", fmt.Errorf("%s exited %d: %s", name, exitErr.ExitCode(), strings.TrimSpace(string(exitErr.Stderr)))
		}
		if errors.Is(err, exec.ErrNotFound) {
			return "", fmt.Errorf("%s binary not found in PATH", name)
		}
		return "", fmt.Errorf("failed to run %s: %w", name, err)
	}
	return string(out), nil
}

// Fetch invokes claude --print "/cost" and returns the raw output.
func Fetch(run RunFunc) (string, error) {
	return run("claude", "--print", "/cost")
}
