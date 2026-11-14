package outbox

type merge struct {
	outcome runOutcome
}

func (merge merge) apply(fresh Fields) (Fields, error) {
	state, err := Codec{}.Read(fresh)
	if err != nil {
		return Fields{}, err
	}
	state.Pending = merge.keepWaiting(merge.replaceAll(state.Pending))
	state.Dead, err = deadReplacing{}.added(state.Dead, merge.outcome.deadLetters)
	if err != nil {
		return Fields{}, err
	}
	return Codec{}.Write(state)
}

func (merge merge) replaceAll(pending []PendingEvent) []PendingEvent {
	replaced := []PendingEvent{}
	for _, event := range pending {
		replaced = append(replaced, merge.replacement(event))
	}
	return replaced
}

func (merge merge) replacement(event PendingEvent) PendingEvent {
	updated, found := merge.outcome.updated[event.Envelope.EventID]
	if !found {
		return event
	}
	return updated
}

func (merge merge) keepWaiting(pending []PendingEvent) []PendingEvent {
	kept := []PendingEvent{}
	for _, event := range pending {
		kept = append(kept, merge.ifWaiting(event)...)
	}
	return kept
}

func (merge) ifWaiting(event PendingEvent) []PendingEvent {
	if len(event.Waiting) == 0 {
		return []PendingEvent{}
	}
	return []PendingEvent{event}
}
