package notify

import (
	"testing"
	"time"
)

func TestSummaryClockIsZeroWhenOff(t *testing.T) {
	expectSame(t, SummaryClock{}.DueAt(time.Unix(1000, 0), 0), int64(0))
}

func TestSummaryClockAddsDelay(t *testing.T) {
	expectSame(t, SummaryClock{}.DueAt(time.Unix(1000, 0), 15), int64(1900))
}
