package mailin

import (
	"time"

	"mesail.dev/packages/outbox"
)

type Inbox struct {
	Storage  Storage
	Registry CollectionRegistry
	Stager   EventStager
	Outbox   OutboxDesk
	HandOff  HandOff
	Now      func() time.Time
}

func (inbox Inbox) Accept(envelope Envelope) (string, error) {
	domainId, err := EnvelopeRules{}.Check(envelope)
	if err != nil {
		return "", err
	}
	envelope, err = inbox.HandOff.Received(envelope, domainId)
	if err != nil {
		return "", err
	}
	collection, err := inbox.Collection(domainId)
	if err != nil {
		return "", err
	}
	stored, err := inbox.findOrInsert(collection, domainId, envelope)
	if err != nil {
		return "", err
	}
	return stored.WireId(), inbox.queueEvent(collection, stored, envelope)
}

func (inbox Inbox) Collection(domainId string) (string, error) {
	collection := InboundBook{}.Collection(domainId)
	err := inbox.Storage.Ensure(collection)
	if err != nil {
		return "", err
	}
	return collection, inbox.Registry.Register(collection)
}

func (inbox Inbox) FindByBlobKey(collection string, key string) ([]InboundRaw, error) {
	found := []InboundRaw{}
	err := inbox.Storage.All(collection, map[string]any{"rawBlobKey": key}, &found)
	return found, err
}

func (Inbox) AcceptedState() string {
	return "accepted"
}

func (inbox Inbox) findOrInsert(collection string, domainId string, envelope Envelope) (InboundRaw, error) {
	matches, err := inbox.FindByBlobKey(collection, envelope.RawBlobKey)
	if err != nil {
		return InboundRaw{}, err
	}
	if len(matches) > 0 {
		return matches[0], nil
	}
	record := inbox.record(domainId, envelope)
	record.ID, err = inbox.Storage.Insert(collection, record)
	return record, err
}

func (inbox Inbox) queueEvent(collection string, stored InboundRaw, envelope Envelope) error {
	if stored.EventQueued {
		return nil
	}
	patch, err := inbox.Stager.Stage(stored, envelope)
	if err != nil {
		return err
	}
	err = inbox.Storage.Replace(collection, stored.ID, patch)
	if err != nil {
		return err
	}
	inbox.Outbox.Drain(outbox.Ref{Collection: collection, Id: stored.ID})
	return nil
}

func (inbox Inbox) record(domainId string, envelope Envelope) InboundRaw {
	return InboundRaw{
		DomainId:      domainId,
		ReceivedAt:    inbox.Now().UTC().Format(time.RFC3339Nano),
		EnvelopeFrom:  envelope.EnvelopeFrom,
		Recipients:    envelope.Recipients,
		RawBlobKey:    envelope.RawBlobKey,
		Size:          envelope.Size,
		RemoteAddress: envelope.RemoteAddress,
		Helo:          envelope.Helo,
		Tls:           envelope.Tls,
		MessageId:     envelope.Headers.MessageId,
		Subject:       envelope.Headers.Subject,
		FromAddress:   envelope.Headers.From.Address,
		AuthResults:   envelope.AuthResults,
		State:         inbox.AcceptedState(),
		Fields:        outbox.Fields{PendingEvents: []string{}, DeadEvents: []string{}},
	}
}
