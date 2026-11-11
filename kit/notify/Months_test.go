package notify

import (
	"testing"
	"time"
)

func TestMonthsInMiddleOfYear(t *testing.T) {
	expectSame(t, Months{}.Recent(time.Date(2026, 11, 30, 23, 0, 0, 0, time.UTC)), []string{"202611", "202610"})
}

func TestMonthsInJanuaryUseLastYear(t *testing.T) {
	expectSame(t, Months{}.Recent(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)), []string{"202701", "202612"})
}

func TestMonthsUseUtc(t *testing.T) {
	expectSame(t, Months{}.Recent(time.Date(2026, 12, 1, 0, 30, 0, 0, time.FixedZone("plus", 3600))), []string{"202611", "202610"})
}
