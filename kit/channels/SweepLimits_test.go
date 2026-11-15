package channels

import (
	"strconv"
	"testing"
	"time"
)

func TestASweepStopsAfterTwentyFiveDocumentsAndSaysThereIsMore(t *testing.T) {
	w := newWorld(t)
	w.configure(`{"yon":{"outbox":{"producer":"channels","subscribers":[{"name":"search","url":"http://127.0.0.1:1/s"}]}}}`)
	w.pusher.FailFor("search", storageDown{})
	for index := 0; index < 15; index++ {
		w.create("Bearer ana", "c"+strconv.Itoa(index), "public")
	}
	w.pusher.Recover("search")
	w.clock = w.clock.Add(time.Hour)
	first, err := w.desk.RunOnce()
	wantNoError(t, err)
	wantEqual(t, first.Documents, 25)
	wantEqual(t, first.More, true)
	second, err := w.desk.RunOnce()
	wantNoError(t, err)
	wantEqual(t, second.Documents, 5)
	wantEqual(t, second.More, false)
}

func TestASweepStopsWhenItsTimeIsUsedUp(t *testing.T) {
	w := newWorld(t)
	w.configure(`{"yon":{"outbox":{"producer":"channels","subscribers":[{"name":"search","url":"http://127.0.0.1:1/s"}]}}}`)
	w.pusher.FailFor("search", storageDown{})
	w.create("Bearer ana", "c0", "public")
	w.create("Bearer ana", "c1", "public")
	w.pusher.Recover("search")
	w.desk.now = steppingClock(w.clock.Add(time.Hour), 3*time.Second)
	report, err := w.desk.RunOnce()
	wantNoError(t, err)
	wantEqual(t, report.Documents, 0)
	wantEqual(t, report.More, true)
}

func TestASweepHasTwoSecondsToRelay(t *testing.T) {
	wantEqual(t, OutboxDesk{}.timeBudget(), 2*time.Second)
}

func steppingClock(start time.Time, step time.Duration) func() time.Time {
	calls := 0
	return func() time.Time {
		calls++
		return start.Add(time.Duration(calls) * step)
	}
}
