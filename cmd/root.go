package cmd

import (
	"context"
	"fmt"
	"io"
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
	Use:          "claude-usage",
	Short:        "Display Claude CLI session and weekly usage",
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE:         run,
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
	if flagPollingIntervalSecs < 0 {
		return fmt.Errorf("--polling-interval-seconds must be >= 0, got %d", flagPollingIntervalSecs)
	}

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
		data, err := parser.Parse(raw, showSession, showWeekly, time.Now())
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

	ticker := time.NewTicker(time.Duration(flagPollingIntervalSecs) * time.Second)
	defer ticker.Stop()

	poll(ctx, ticker.C, doFetch, clearScreen, !flagJSON, os.Stderr)
	return nil
}

// poll runs tick once immediately and again on every value from ticks until
// ctx is done. Errors are reported inline to stderr and never stop the loop.
// The screen is cleared before each tick only when clearEnabled is set, so
// JSON polling stays a clean newline-delimited stream.
func poll(ctx context.Context, ticks <-chan time.Time, tick func() error, clear func(), clearEnabled bool, stderr io.Writer) {
	once := func() {
		if clearEnabled {
			clear()
		}
		if err := tick(); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
		}
	}

	once()
	for {
		select {
		case <-ticks:
			once()
		case <-ctx.Done():
			return
		}
	}
}

func clearScreen() {
	fmt.Print("\033[H\033[2J")
}
