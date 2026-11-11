package core

import (
	"encoding/json"
	"errors"
)

type DeliveryCodec struct{}

func (codec DeliveryCodec) Encode(records []DeliveryRecord) []string {
	encoded := []string{}
	for _, record := range records {
		text, _ := json.Marshal(record)
		encoded = append(encoded, string(text))
	}
	return encoded
}

func (codec DeliveryCodec) Decode(encoded []string) ([]DeliveryRecord, error) {
	records := []DeliveryRecord{}
	failures := []error{}
	for _, text := range encoded {
		record := DeliveryRecord{}
		failures = append(failures, json.Unmarshal([]byte(text), &record))
		records = append(records, record)
	}
	return records, errors.Join(failures...)
}
