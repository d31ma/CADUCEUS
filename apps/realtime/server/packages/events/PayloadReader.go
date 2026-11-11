package events

import (
	"bytes"
	"encoding/json"
)

type PayloadReader struct{}

func (reader PayloadReader) Read(envelope Envelope, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(envelope.Payload))
	decoder.DisallowUnknownFields()
	err := decoder.Decode(target)
	if err != nil {
		return err
	}
	return reader.requireEnd(decoder)
}

func (PayloadReader) requireEnd(decoder *json.Decoder) error {
	if decoder.More() {
		return InvalidEnvelope{Reason: "payload holds data after its JSON value"}
	}
	return nil
}
