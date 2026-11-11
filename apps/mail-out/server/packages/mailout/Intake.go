package mailout

import (
	"slices"
	"strings"
	"time"

	"mesail.dev/packages/events"
)

type Intake struct {
	Queue    Queue
	Relayer  Relayer
	Sources  Sources
	Settings Settings
	Changes  ChangeEvents
	Now      func() time.Time
}

func (intake Intake) Handle(envelope events.Envelope) error {
	request := events.EmailSendRequested{}
	readErr := events.PayloadReader{}.Read(envelope, &request)
	field := intake.invalidField(envelope, request, readErr == nil)
	if field != "" {
		return events.InvalidEnvelope{Reason: "the email.send.requested field " + field + " is missing or not valid"}
	}
	_, known, err := intake.Queue.FindBy(envelope.DomainId, "idempotencyKey", request.IdempotencyKey)
	if err != nil || known {
		return err
	}
	item := intake.item(envelope, request)
	change, err := intake.Changes.Envelope(item)
	if err != nil {
		return err
	}
	saved, err := intake.Queue.Insert(item, change)
	if err != nil {
		return err
	}
	_, err = intake.Relayer.RelayDocument(intake.Queue.Ref(saved), saved.DomainId)
	return err
}

func (intake Intake) invalidField(envelope events.Envelope, request events.EmailSendRequested, decoded bool) string {
	addresses := Addresses{}
	names := []string{"producer", "payload", "messageId", "idempotencyKey", "rcpt", "from"}
	valid := []bool{
		intake.Sources.Knows(envelope.Producer),
		decoded,
		request.MessageId != "",
		request.IdempotencyKey != "",
		addresses.Mailbox(request.Rcpt).Address != "",
		addresses.Domain(addresses.Mailbox(request.From).Address) == strings.ToLower(envelope.DomainId),
	}
	index := slices.Index(valid, false)
	if index < 0 {
		return ""
	}
	return names[index]
}

func (intake Intake) item(envelope events.Envelope, request events.EmailSendRequested) Item {
	now := intake.Now()
	addresses := Addresses{}
	sender := addresses.Mailbox(request.From)
	return Item{
		DomainId: strings.ToLower(envelope.DomainId), EventId: envelope.EventID, Producer: envelope.Producer, MessageId: request.MessageId,
		From: sender.Address, FromName: sender.Name, Rcpt: addresses.Mailbox(request.Rcpt).Address, Role: request.Role,
		To: request.To, Cc: request.Cc, Subject: request.Subject, InReplyTo: request.InReplyTo, References: request.References,
		AttachmentIds: request.AttachmentIds, IdempotencyKey: request.IdempotencyKey,
		QueuedAt: Clock{}.Format(now), NextAttemptAt: Clock{}.Format(now), ExpiresAt: Clock{}.Format(now.Add(intake.Settings.MaxAge())), State: States{}.Queued(),
	}
}
