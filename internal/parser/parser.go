package parser

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	// The · separator is U+00B7 (middle dot), embedded literally.
	sessionRe = regexp.MustCompile(`Current session: (\d+)% used · resets ([A-Za-z]+ \d+ at \d+(?::\d+)?(?:am|pm)) \(([^)]+)\)`)
	weeklyRe  = regexp.MustCompile(`Current week \(all models\): (\d+)% used · resets ([A-Za-z]+ \d+ at \d+(?::\d+)?(?:am|pm)) \(([^)]+)\)`)
)

// ErrUnexpectedOutput reports that claude --print "/cost" returned output
// without the expected usage lines. It is not transient, so callers stop
// rather than retry.
var ErrUnexpectedOutput = errors.New(`unexpected output from claude --print "/cost"`)

// UsageEntry holds parsed data for one usage period.
type UsageEntry struct {
	Pct      int
	ResetsAt time.Time
	ResetsIn time.Duration
}

// UsageData holds the parsed output of claude --print "/cost".
type UsageData struct {
	Session     *UsageEntry
	Weekly      *UsageEntry
	FetchedAt   time.Time
	WantSession bool
	WantWeekly  bool
}

// Parse extracts usage data from the raw output of claude --print "/cost".
// now is used to compute ResetsIn and to infer the reset year.
func Parse(raw string, wantSession, wantWeekly bool, now time.Time) (UsageData, error) {
	data := UsageData{FetchedAt: now, WantSession: wantSession, WantWeekly: wantWeekly}
	found := false

	if wantSession {
		if m := sessionRe.FindStringSubmatch(raw); m != nil {
			entry, err := parseEntry(m[1], m[2], m[3], now)
			if err != nil {
				return UsageData{}, fmt.Errorf("%w: session entry: %v", ErrUnexpectedOutput, err)
			}
			data.Session = entry
			found = true
		}
	}

	if wantWeekly {
		if m := weeklyRe.FindStringSubmatch(raw); m != nil {
			entry, err := parseEntry(m[1], m[2], m[3], now)
			if err != nil {
				return UsageData{}, fmt.Errorf("%w: weekly entry: %v", ErrUnexpectedOutput, err)
			}
			data.Weekly = entry
			found = true
		}
	}

	if !found {
		return UsageData{}, fmt.Errorf("%w: no usage data found", ErrUnexpectedOutput)
	}

	return data, nil
}

func parseEntry(pctStr, timeStr, tzName string, now time.Time) (*UsageEntry, error) {
	pct, err := strconv.Atoi(pctStr)
	if err != nil {
		return nil, fmt.Errorf("invalid percentage %q: %w", pctStr, err)
	}
	resetsAt, err := parseResetTime(timeStr, tzName, now)
	if err != nil {
		return nil, err
	}
	return &UsageEntry{
		Pct:      pct,
		ResetsAt: resetsAt,
		ResetsIn: resetsAt.Sub(now),
	}, nil
}

func parseResetTime(timeStr, tzName string, now time.Time) (time.Time, error) {
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		return time.Time{}, fmt.Errorf("unknown timezone %q: %w", tzName, err)
	}

	// Normalize lowercase "pm"/"am" to "PM"/"AM" for Go's time parser.
	normalized := timeStr[:len(timeStr)-2] + strings.ToUpper(timeStr[len(timeStr)-2:])
	withYear := fmt.Sprintf("%d %s", now.In(loc).Year(), normalized)

	for _, format := range []string{
		"2006 Jan 2 at 3:04PM",
		"2006 Jan 2 at 3PM",
	} {
		t, parseErr := time.ParseInLocation(format, withYear, loc)
		if parseErr == nil {
			// The output carries no year. A reset months in the past means it
			// falls in next year (December → January); one that passed only
			// moments ago is stale output and keeps the current year.
			if t.Before(now.AddDate(0, -6, 0)) {
				t = t.AddDate(1, 0, 0)
			}
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("cannot parse reset time %q with timezone %q", timeStr, tzName)
}
