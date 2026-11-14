package outbox

type pendingSplit struct {
	kept    []PendingEvent
	matched []PendingEvent
}

func (split pendingSplit) with(event PendingEvent, found bool) pendingSplit {
	if found {
		split.matched = append(split.matched, event)
		return split
	}
	split.kept = append(split.kept, event)
	return split
}

func (split pendingSplit) matchedWaiting() []string {
	waiting := []string{}
	for _, event := range split.matched {
		waiting = append(waiting, event.Waiting...)
	}
	return waiting
}
