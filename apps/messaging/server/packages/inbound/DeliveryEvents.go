package inbound

import (
	"slices"
	"strings"

	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
)

type DeliveryEvents struct {
	messages core.MessageStore
}

func NewDeliveryEvents(messages core.MessageStore) DeliveryEvents {
	return DeliveryEvents{messages: messages}
}

func (deliveryEvents DeliveryEvents) Changed(envelope events.Envelope) error {
	payload := events.EmailDeliveryChanged{}
	err := events.PayloadReader{}.Read(envelope, &payload)
	if err != nil {
		return err
	}
	month, storedId, err := core.MessageIds{}.Parse(payload.MessageId)
	if err != nil {
		return err
	}
	message, _, err := deliveryEvents.messages.Find(envelope.DomainId, month, storedId)
	if err != nil {
		return Unavailability{}.Of(err)
	}
	records, err := core.DeliveryCodec{}.Decode(message.Deliveries)
	if err != nil {
		return err
	}
	before := core.DeliveryCodec{}.Encode(records)
	after := core.DeliveryCodec{}.Encode(deliveryEvents.applied(records, payload))
	err = deliveryEvents.saveIfChanged(message, before, after)
	if err != nil {
		return Unavailability{}.Of(err)
	}
	return nil
}

func (deliveryEvents DeliveryEvents) applied(records []core.DeliveryRecord, payload events.EmailDeliveryChanged) []core.DeliveryRecord {
	updated := []core.DeliveryRecord{}
	for _, record := range records {
		updated = append(updated, deliveryEvents.updated(record, payload, deliveryEvents.matches(record, payload)))
	}
	return updated
}

func (DeliveryEvents) matches(record core.DeliveryRecord, payload events.EmailDeliveryChanged) bool {
	isEmail := record.Channel == (core.DeliveryChannels{}).Email()
	return isEmail && strings.EqualFold(record.Recipient, payload.Rcpt) && DeliveryRanks{}.Allows(record.Status, payload.Status)
}

func (DeliveryEvents) updated(record core.DeliveryRecord, payload events.EmailDeliveryChanged, matches bool) core.DeliveryRecord {
	if !matches {
		return record
	}
	record.Status = payload.Status
	record.Detail = payload.Detail
	return record
}

func (deliveryEvents DeliveryEvents) saveIfChanged(message core.Message, before []string, after []string) error {
	if slices.Equal(before, after) {
		return nil
	}
	message.Deliveries = after
	return deliveryEvents.messages.Save(message)
}
