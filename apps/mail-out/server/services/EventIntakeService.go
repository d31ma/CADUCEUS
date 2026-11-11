import (
	"errors"
	"slices"
	"strings"
	"time"

	"mesail.dev/packages/events"
)

// @Service
type EventIntakeService struct {
	// @Needs
	outbound *OutboundStoreDelegate
	// @Needs
	relay *OutboxRelayService
	// @Needs
	settings *SettingsDelegate
	// @Needs
	sources *MessageSourceDelegate
}

func (EventIntakeService) missing() int { return -1 }

func (intake *EventIntakeService) Accept(envelope events.Envelope, authorize func(string) (string, error)) error {
	request := events.EmailSendRequested{}
	readErr := events.PayloadReader{}.Read(envelope, &request)
	checkErr := intake.refuse(intake.invalidField(envelope, request, readErr == nil))
	_, known, findErr := intake.outbound.FindBy(envelope.DomainId, "idempotencyKey", request.IdempotencyKey)
	item := intake.item(envelope, request, time.Now())
	change, changeErr := ChangeUtils{}.Envelope(item)
	saved, queueErr := intake.queue(item, change, known, errors.Join(checkErr, findErr, changeErr))
	return errors.Join(queueErr, intake.announce(saved, authorize))
}

func (intake *EventIntakeService) invalidField(envelope events.Envelope, request events.EmailSendRequested, decoded bool) string {
	addresses := AddressUtils{}
	names := []string{"producer", "payload", "messageId", "idempotencyKey", "rcpt", "from"}
	valid := []bool{
		intake.sources.Knows(envelope.Producer),
		decoded,
		request.MessageId != "",
		request.IdempotencyKey != "",
		addresses.Plain(request.Rcpt) != "",
		addresses.Domain(addresses.Plain(request.From)) == strings.ToLower(envelope.DomainId),
	}
	return intake.fieldAt(names, slices.Index(valid, false))
}

func (intake *EventIntakeService) fieldAt(names []string, index int) string {
	if index == intake.missing() {
		return ""
	}
	return names[index]
}

func (EventIntakeService) refuse(field string) error {
	if field != "" {
		return events.InvalidEnvelope{Reason: "the email.send.requested field " + field + " is missing or not valid"}
	}
	return nil
}

func (intake *EventIntakeService) item(envelope events.Envelope, request events.EmailSendRequested, now time.Time) OutboundItem {
	addresses := AddressUtils{}
	sender := addresses.Mailbox(request.From)
	return OutboundItem{
		DomainId: strings.ToLower(envelope.DomainId), EventId: envelope.EventID, Producer: envelope.Producer, MessageId: request.MessageId,
		From: sender.Address, FromName: sender.Name, Rcpt: addresses.Plain(request.Rcpt), Role: request.Role,
		To: request.To, Cc: request.Cc, Subject: request.Subject, InReplyTo: request.InReplyTo, References: request.References,
		AttachmentIds: request.AttachmentIds, IdempotencyKey: request.IdempotencyKey,
		QueuedAt: TimeUtils{}.Format(now), NextAttemptAt: TimeUtils{}.Format(now),
		ExpiresAt: TimeUtils{}.Format(now.Add(intake.settings.MaxAge())), State: OutboundStateUtils{}.Queued(),
	}
}

func (intake *EventIntakeService) queue(item OutboundItem, change events.Envelope, known bool, failure error) (OutboundItem, error) {
	if known || failure != nil {
		return OutboundItem{}, failure
	}
	return intake.outbound.Insert(item, change)
}

func (intake *EventIntakeService) announce(item OutboundItem, authorize func(string) (string, error)) error {
	if item.Id == "" {
		return nil
	}
	_, err := intake.relay.RelayDocument(StorageUtils{}.QueueRef(item), item.DomainId, authorize)
	return err
}
