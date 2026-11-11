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
	documents := outboxDocumentsForTest{{}, {}}
	calls := 0
	clock := time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)
	desk := OutboxDesk{now: func() time.Time { calls++; return clock.Add(time.Duration(calls) * 25 * time.Second) }}
	report, err := desk.relayEach(documents.refs())
	wantNoError(t, err)
	wantEqual(t, report.Documents, 0)
	wantEqual(t, report.More, true)
}
