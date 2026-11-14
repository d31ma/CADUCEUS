package outbox

import (
	"encoding/json"
	"strings"
)

type Codec struct{}

func (codec Codec) Read(fields Fields) (OutboxState, error) {
	pending, err := codec.DecodePending(fields.PendingEvents)
	if err != nil {
		return OutboxState{}, err
	}
	dead, err := codec.DecodeDead(fields.DeadEvents)
	return OutboxState{Pending: pending, Dead: dead}, err
}

func (codec Codec) Write(state OutboxState) (Fields, error) {
	pending, err := codec.EncodePending(state.Pending)
	if err != nil {
		return Fields{}, err
	}
	dead, err := codec.EncodeDead(state.Dead)
	return Fields{PendingEvents: pending, DeadEvents: dead, OutboxPending: len(pending), OutboxDead: len(dead), OutboxWaiting: len(pending) > 0, OutboxHasDead: len(dead) > 0}, err
}

func (codec Codec) EncodePending(pending []PendingEvent) ([]string, error) {
	return codec.split(json.Marshal(pending))
}

func (codec Codec) EncodeDead(dead []DeadLetter) ([]string, error) {
	return codec.split(json.Marshal(dead))
}

func (codec Codec) DecodePending(texts []string) ([]PendingEvent, error) {
	return codec.decodeEachPending(texts)
}

func (codec Codec) DecodeDead(texts []string) ([]DeadLetter, error) {
	return codec.decodeEachDead(texts)
}

func (codec Codec) split(list []byte, err error) ([]string, error) {
	if err != nil {
		return nil, err
	}
	parts := []json.RawMessage{}
	err = json.Unmarshal(list, &parts)
	return codec.texts(parts), err
}

func (Codec) texts(parts []json.RawMessage) []string {
	texts := []string{}
	for _, part := range parts {
		texts = append(texts, string(part))
	}
	return texts
}

func (codec Codec) decodeEachPending(texts []string) ([]PendingEvent, error) {
	pending := []PendingEvent{}
	for _, text := range texts {
		event := PendingEvent{}
		err := codec.strict(text, &event)
		if err != nil {
			return nil, err
		}
		pending = append(pending, event)
	}
	return pending, nil
}

func (codec Codec) decodeEachDead(texts []string) ([]DeadLetter, error) {
	dead := []DeadLetter{}
	for _, text := range texts {
		letter := DeadLetter{}
		err := codec.strict(text, &letter)
		if err != nil {
			return nil, err
		}
		dead = append(dead, letter)
	}
	return dead, nil
}

func (Codec) strict(text string, target any) error {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}
