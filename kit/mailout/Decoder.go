package mailout

import "encoding/json"

type Decoder struct{}

func (Decoder) Items(documents []json.RawMessage) ([]Item, error) {
	return documentDecoder[Item]{}.decode(documents)
}

func (Decoder) Attempts(documents []json.RawMessage) ([]DeliveryAttempt, error) {
	return documentDecoder[DeliveryAttempt]{}.decode(documents)
}
