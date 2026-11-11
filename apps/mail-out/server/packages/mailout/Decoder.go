package mailout

import (
	"encoding/json"
	"errors"
)

type Decoder struct{}

func (Decoder) Items(documents []json.RawMessage) ([]Item, error) {
	items := []Item{}
	var err error
	for _, document := range documents {
		item := Item{}
		err = errors.Join(err, json.Unmarshal(document, &item))
		items = append(items, item)
	}
	return items, err
}

func (Decoder) Attempts(documents []json.RawMessage) ([]DeliveryAttempt, error) {
	attempts := []DeliveryAttempt{}
	var err error
	for _, document := range documents {
		attempt := DeliveryAttempt{}
		err = errors.Join(err, json.Unmarshal(document, &attempt))
		attempts = append(attempts, attempt)
	}
	return attempts, err
}
