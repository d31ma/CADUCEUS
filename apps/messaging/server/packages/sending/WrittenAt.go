package sending

import (
	"time"

	"mesail.dev/packages/core"
)

type WrittenAt struct{}

func (written WrittenAt) check(text string) []error {
	_, err := written.parsed(text)
	if text == "" || err == nil {
		return nil
	}
	return []error{core.InvalidRequest{}.WrittenAtFormat()}
}

func (written WrittenAt) kept(text string, now time.Time) string {
	at, err := written.parsed(text)
	if err != nil || !at.Before(now) || at.Before(now.Add(-written.longest())) {
		return ""
	}
	return at.UTC().Format(time.RFC3339)
}

func (WrittenAt) parsed(text string) (time.Time, error) {
	return time.Parse(time.RFC3339, text)
}

func (WrittenAt) longest() time.Duration {
	return 30 * 24 * time.Hour
}
