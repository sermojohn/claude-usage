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
