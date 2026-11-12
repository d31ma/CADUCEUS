package inbound

import (
	"encoding/json"
	"errors"

	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
)

type ReceivedContent struct{}

func (ReceivedContent) From(content events.EmailContent) (core.InboundContent, error) {
	converted := core.InboundContent{}
	encoded, encodeFailure := json.Marshal(content)
	decodeFailure := json.Unmarshal(encoded, &converted)
	return converted, errors.Join(encodeFailure, decodeFailure)
}
