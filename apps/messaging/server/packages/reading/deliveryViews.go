package reading

import (
	"mesail.dev/packages/core"
	"mesail.dev/packages/oidc"
)

type deliveryViews struct {
	readSequences map[string]int
}

func (deliveryViews) forParticipants(participants []core.Participant) deliveryViews {
	readSequences := map[string]int{}
	for _, participant := range participants {
		readSequences[participant.MemberId] = participant.ReadSequence
	}
	return deliveryViews{readSequences: readSequences}
}

func (deliveries deliveryViews) of(viewer oidc.Caller, message core.Message) ([]core.DeliveryRecord, error) {
	records, err := core.DeliveryCodec{}.Decode(message.Deliveries)
	authored := deliveries.authored(viewer, message)
	shown := []core.DeliveryRecord{}
	for _, record := range records {
		shown = append(shown, deliveries.ifVisible(deliveries.withReadStatus(record, message.Sequence), authored)...)
	}
	return shown, err
}

func (deliveryViews) authored(viewer oidc.Caller, message core.Message) bool {
	return message.AuthorMemberId != "" && message.AuthorMemberId == viewer.MemberId
}

func (deliveryViews) ifVisible(record core.DeliveryRecord, authored bool) []core.DeliveryRecord {
	if record.Role == (core.RecipientRoles{}).Bcc() && !authored {
		return nil
	}
	return []core.DeliveryRecord{record}
}

func (deliveries deliveryViews) withReadStatus(record core.DeliveryRecord, sequence int) core.DeliveryRecord {
	readSequence, joined := deliveries.readSequences[record.MemberId]
	if record.Channel == (core.DeliveryChannels{}).Chat() && joined && readSequence >= sequence {
		record.Status = core.DeliveryStatuses{}.Read()
	}
	return record
}
