package mailout

import (
	"errors"
	"strings"
	"time"

	"mesail.dev/packages/outbox"
)

type Relayer struct {
	Store     OutboxStore
	Settings  outbox.Settings
	Authorize func(realm string) (string, error)
	Timeout   time.Duration
	Now       func() time.Time
}

func (relayer Relayer) RunOnce(limit int) (outbox.Report, error) {
	documents, err := relayer.Store.Pending(limit)
	if err != nil {
		return outbox.Report{}, err
	}
	total := outbox.Report{}
	for _, document := range documents {
		owner, ownerErr := relayer.Store.Owner(document.Ref)
		report, relayErr := relayer.RelayDocument(document.Ref, owner)
		total = outbox.Report{Documents: total.Documents + report.Documents, Delivered: total.Delivered + report.Delivered, Retried: total.Retried + report.Retried, DeadLettered: total.DeadLettered + report.DeadLettered}
		err = errors.Join(err, ownerErr, relayErr)
	}
	return total, err
}

func (relayer Relayer) RelayDocument(ref outbox.Ref, domainId string) (outbox.Report, error) {
	authorization, err := relayer.Authorize(domainId)
	if err != nil {
		return outbox.Report{}, err
	}
	tokens := outbox.StaticToken{Value: strings.TrimPrefix(authorization, "Bearer ")}
	pusher := outbox.NewHTTPPusher(tokens, relayer.Timeout)
	return outbox.NewRelay(relayer.Store, pusher, relayer.Settings, relayer.Now).RelayDocument(ref)
}

func (relayer Relayer) Pending(limit int) ([]outbox.Document, error) {
	return relayer.Store.Pending(limit)
}

func (relayer Relayer) DeadLetters(limit int) ([]outbox.DeadLetter, error) {
	documents, err := relayer.Store.WithDeadLetters(limit)
	letters := []outbox.DeadLetter{}
	for _, document := range documents {
		listed, listErr := outbox.DeadLetters{}.List(document.Fields)
		letters, err = append(letters, listed...), errors.Join(err, listErr)
	}
	return letters, err
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
