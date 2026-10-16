package cmd

import "testing"

func TestFlagShorthands(t *testing.T) {
	want := map[string]string{
		"current-session":          "c",
		"weekly-session":           "w",
		"polling-interval-seconds": "p",
		"json":                     "j",
	}
	for name, short := range want {
		f := rootCmd.Flags().Lookup(name)
		if f == nil {
			t.Fatalf("flag --%s not defined", name)
		}
		if f.Shorthand != short {
			t.Errorf("--%s shorthand = %q, want %q", name, f.Shorthand, short)
		}
	}
}

func TestShorthandsParse(t *testing.T) {
	t.Cleanup(func() {
		flagCurrentSession, flagWeeklySession, flagPollingIntervalSecs, flagJSON = false, false, 0, false
	})
	if err := rootCmd.ParseFlags([]string{"-cwj", "-p", "30"}); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	if !flagCurrentSession || !flagWeeklySession || !flagJSON || flagPollingIntervalSecs != 30 {
		t.Errorf("got current=%v weekly=%v json=%v poll=%d", flagCurrentSession, flagWeeklySession, flagJSON, flagPollingIntervalSecs)
	}
}
