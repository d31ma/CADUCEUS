package outbox

import (
	"cmp"
	"maps"
	"slices"
	"strings"
	"time"
)

type DueOrder struct{}

func (order DueOrder) Due(documents []Document, now time.Time) []Document {
	candidates := order.candidates(documents)
	due := order.dueOnly(candidates, now)
	slices.SortStableFunc(due, order.compare)
	return order.documentsOf(due)
}

func (order DueOrder) candidates(documents []Document) []dueDocument {
	candidates := []dueDocument{}
	for _, document := range documents {
		candidates = append(candidates, order.candidate(document))
	}
	return candidates
}

func (order DueOrder) candidate(document Document) dueDocument {
	state, err := Codec{}.Read(document.Fields)
	times := slices.Collect(maps.Values(order.firstTimes(state.Pending)))
	return dueDocument{document: document, times: times, readable: err == nil}
}

func (DueOrder) dueOnly(candidates []dueDocument, now time.Time) []dueDocument {
	return slices.DeleteFunc(candidates, func(candidate dueDocument) bool { return !candidate.due(now) })
}

func (order DueOrder) firstTimes(pending []PendingEvent) map[string]time.Time {
	times := map[string]time.Time{}
	for _, event := range pending {
		order.addFirst(times, event)
	}
	return times
}

func (order DueOrder) addFirst(times map[string]time.Time, event PendingEvent) {
	nextAttempt, _ := time.Parse(time.RFC3339Nano, event.NextAttemptAt)
	for _, name := range event.Waiting {
		order.setOnce(times, name, nextAttempt)
	}
}

func (DueOrder) setOnce(times map[string]time.Time, name string, moment time.Time) {
	_, known := times[name]
	if !known {
		times[name] = moment
	}
}

func (DueOrder) compare(first dueDocument, second dueDocument) int {
	return cmp.Or(
		cmp.Compare(first.rank(), second.rank()),
		first.earliest().Compare(second.earliest()),
		strings.Compare(first.document.Ref.Collection, second.document.Ref.Collection),
		strings.Compare(first.document.Ref.Id, second.document.Ref.Id),
	)
}

func (DueOrder) documentsOf(due []dueDocument) []Document {
	documents := []Document{}
	for _, candidate := range due {
		documents = append(documents, candidate.document)
	}
	return documents
}
