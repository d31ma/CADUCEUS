package outbox

import "mesail.dev/packages/events"

type Published struct{}

func (published Published) Staged(fields Fields, eventType string) ([]events.Envelope, error) {
	state, err := Codec{}.Read(fields)
	if err != nil {
		return nil, err
	}
	return published.ofType(state.Pending, eventType), nil
}

func (published Published) One(fields Fields, eventType string, payload any) (events.Envelope, error) {
	staged, err := published.Staged(fields, eventType)
	if err != nil {
		return events.Envelope{}, err
	}
	envelope, err := published.single(staged, eventType)
	if err != nil {
		return events.Envelope{}, err
	}
	return envelope, Codec{}.strict(string(envelope.Payload), payload)
}

func (published Published) ofType(pending []PendingEvent, eventType string) []events.Envelope {
	envelopes := []events.Envelope{}
	for _, event := range pending {
		envelopes = append(envelopes, published.ifType(event, eventType)...)
	}
	return envelopes
}

func (Published) ifType(event PendingEvent, eventType string) []events.Envelope {
	if event.Envelope.Type != eventType {
		return []events.Envelope{}
	}
	return []events.Envelope{event.Envelope}
}

func (Published) single(staged []events.Envelope, eventType string) (events.Envelope, error) {
	if len(staged) != 1 {
		return events.Envelope{}, NotPublished{Type: eventType, Count: len(staged)}
	}
	return staged[0], nil
}
