package outbox

type DeadLetters struct{}

func (letters DeadLetters) List(fields Fields) ([]DeadLetter, error) {
	state, err := Codec{}.Read(fields)
	return state.Dead, err
}

func (letters DeadLetters) Requeue(fields Fields, eventId string, subscriber string) (Fields, error) {
	state, err := Codec{}.Read(fields)
	if err != nil {
		return fields, err
	}
	position, err := letters.position(letters.positions(state.Dead), eventId, subscriber)
	if err != nil {
		return fields, err
	}
	letter := state.Dead[position]
	state.Dead = append(state.Dead[:position:position], state.Dead[position+1:]...)
	state.Pending = letters.requeued(state.Pending, letters.eventPositions(state.Pending), letter)
	return Codec{}.Write(state)
}

func (letters DeadLetters) positions(dead []DeadLetter) map[string]int {
	byKey := map[string]int{}
	for index, letter := range dead {
		byKey[letters.key(letter.Envelope.EventID, letter.Subscriber)] = index
	}
	return byKey
}

func (letters DeadLetters) position(byKey map[string]int, eventId string, subscriber string) (int, error) {
	position, found := byKey[letters.key(eventId, subscriber)]
	if !found {
		return 0, NotFound{Kind: letters.kind(), Key: eventId + " for " + subscriber}
	}
	return position, nil
}

func (DeadLetters) eventPositions(pending []PendingEvent) map[string]int {
	byEventId := map[string]int{}
	for index, event := range pending {
		byEventId[event.Envelope.EventID] = index
	}
	return byEventId
}

func (DeadLetters) requeued(pending []PendingEvent, byEventId map[string]int, letter DeadLetter) []PendingEvent {
	position, found := byEventId[letter.Envelope.EventID]
	if !found {
		return append(pending, PendingEvent{Envelope: letter.Envelope, Waiting: []string{letter.Subscriber}})
	}
	pending[position].Waiting = append(pending[position].Waiting, letter.Subscriber)
	return pending
}

func (DeadLetters) key(eventId string, subscriber string) string {
	return eventId + "\n" + subscriber
}

func (DeadLetters) kind() string {
	return "dead letter"
}
