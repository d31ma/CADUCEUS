package mailin

import (
	"encoding/json"
	"errors"

	"mesail.dev/packages/events"
)

type EventContent struct{}

func (content EventContent) From(inbound InboundContent) (events.EmailContent, error) {
	converted := events.EmailContent{}
	encoded, encodeFailure := json.Marshal(inbound)
	decodeFailure := json.Unmarshal(encoded, &converted)
	return converted, content.unreadable(errors.Join(encodeFailure, decodeFailure))
}

func (EventContent) unreadable(failure error) error {
	if failure == nil {
		return nil
	}
	return Unavailable{Operation: "event content"}
}
