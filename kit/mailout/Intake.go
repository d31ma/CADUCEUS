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

func (Intake) keyField() string { return "idempotencyKey" }

func (intake Intake) Handle(envelope events.Envelope) error {
	request := events.EmailSendRequested{}
	readErr := events.PayloadReader{}.Read(envelope, &request)
	err := intake.validate(envelope, request, readErr == nil)
	if err != nil {
		return err
	}
	_, known, err := intake.Queue.FindBy(envelope.DomainId, intake.keyField(), request.IdempotencyKey)
	if err != nil {
		return err
	}
	return intake.enqueueUnlessKnown(known, envelope, request)
}

func (intake Intake) validate(envelope events.Envelope, request events.EmailSendRequested, decoded bool) error {
	index := slices.Index(intake.checks(envelope, request, decoded), false)
	if index < 0 {
		return nil
	}
	return events.InvalidEnvelope{Reason: "the email.send.requested field " + intake.fieldNames()[index] + " is missing or not valid"}
}

func (intake Intake) fieldNames() []string {
	return []string{"producer", "payload", "messageId", intake.keyField(), "rcpt", "from"}
}

func (intake Intake) checks(envelope events.Envelope, request events.EmailSendRequested, decoded bool) []bool {
	addresses := Addresses{}
	return []bool{
		intake.Sources.Knows(envelope.Producer),
		decoded,
		request.MessageId != "",
		request.IdempotencyKey != "",
		addresses.Mailbox(request.Rcpt).Address != "",
		addresses.Domain(addresses.Mailbox(request.From).Address) == strings.ToLower(envelope.DomainId),
	}
}

func (intake Intake) enqueueUnlessKnown(known bool, envelope events.Envelope, request events.EmailSendRequested) error {
	if known {
		return nil
	}
	return intake.enqueue(envelope, request)
}

func (intake Intake) enqueue(envelope events.Envelope, request events.EmailSendRequested) error {
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
