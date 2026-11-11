package mailout

import (
	"encoding/json"
	"errors"
)

type documentDecoder[T any] struct{}

func (decoder documentDecoder[T]) decode(documents []json.RawMessage) ([]T, error) {
	values := []T{}
	failures := []error{}
	for _, document := range documents {
		value, err := decoder.decodeOne(document)
		values = append(values, value)
		failures = append(failures, err)
	}
	return values, errors.Join(failures...)
}

func (documentDecoder[T]) decodeOne(document json.RawMessage) (T, error) {
	var value T
	err := json.Unmarshal(document, &value)
	return value, err
}
