package mailout

import (
	"errors"
	"time"

	"mesail.dev/packages/outbox"
)

type Relayer struct {
	Store    OutboxStore
	Settings outbox.Settings
	Client   ServiceClient
	Now      func() time.Time
}

func (relayer Relayer) RunOnce(limit int) (outbox.Report, error) {
	return relayer.relay().RunOnce(limit)
}

func (relayer Relayer) RelayDocument(ref outbox.Ref, domainId string) (outbox.Report, error) {
	_, err := relayer.Client.Authorize(domainId)
	if err != nil {
		return outbox.Report{}, err
	}
	return relayer.relay().RelayDocument(ref)
}

func (relayer Relayer) Pending(limit int) ([]outbox.Document, error) {
	return relayer.Store.Pending(limit)
}

func (relayer Relayer) DeadLetters(limit int) ([]outbox.DeadLetter, error) {
	documents, err := relayer.Store.WithDeadLetters(limit)
	letters, listErr := relayer.letters(documents)
	return letters, errors.Join(err, listErr)
}

func (relayer Relayer) Requeue(request RequeueRequest) (outbox.Report, error) {
	ref := Queue{}.Ref(Item{DomainId: request.DomainId, Id: request.ItemId})
	fields, err := relayer.Store.Load(ref)
	if err != nil {
		return outbox.Report{}, err
	}
	requeued, err := outbox.DeadLetters{}.Requeue(fields, request.EventId, request.Subscriber)
	if err != nil {
		return outbox.Report{}, err
	}
	err = relayer.Store.Save(ref, requeued)
	if err != nil {
		return outbox.Report{}, err
	}
	return relayer.RelayDocument(ref, request.DomainId)
}

func (relayer Relayer) relay() outbox.Relay {
	return outbox.NewRelay(relayer.Store, SubscriberPusher{Client: relayer.Client}, relayer.Settings, relayer.Now)
}

func (Relayer) letters(documents []outbox.Document) ([]outbox.DeadLetter, error) {
	letters := []outbox.DeadLetter{}
	failures := []error{}
	for _, document := range documents {
		listed, err := outbox.DeadLetters{}.List(document.Fields)
		letters, failures = append(letters, listed...), append(failures, err)
	}
	return letters, errors.Join(failures...)
}
