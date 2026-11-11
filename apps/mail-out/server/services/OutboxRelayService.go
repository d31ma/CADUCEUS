import (
	"errors"
	"strings"
	"time"

	"mesail.dev/packages/outbox"
)

// @Service
type OutboxRelayService struct {
	// @Needs
	store *OutboxStoreDelegate
	// @Needs
	settings *SettingsDelegate
}

func (OutboxRelayService) bearer() string { return "Bearer " }

func (relay *OutboxRelayService) RunOnce(limit int, authorize func(string) (string, error)) (RelayReport, error) {
	documents, err := relay.store.Pending(limit)
	report, relayErr := relay.relayEach(documents, authorize)
	return report, errors.Join(err, relayErr)
}

func (relay *OutboxRelayService) RelayDocument(ref outbox.Ref, domainId string, authorize func(string) (string, error)) (RelayReport, error) {
	authorization, tokenErr := authorize(domainId)
	settings, settingsErr := relay.settings.Outbox()
	report, runErr := relay.relayOne(ref, settings, authorization, errors.Join(tokenErr, settingsErr))
	return relay.report(report), runErr
}

func (relay *OutboxRelayService) Pending(limit int) ([]PendingDocument, error) {
	documents, err := relay.store.Pending(limit)
	return relay.views(documents), err
}

func (relay *OutboxRelayService) DeadLetters(limit int) ([]DeadLetterView, error) {
	documents, err := relay.store.WithDeadLetters(limit)
	letters, listErr := relay.letters(documents)
	return letters, errors.Join(err, listErr)
}

func (relay *OutboxRelayService) Requeue(request RequeueRequest, authorize func(string) (string, error)) (RelayReport, error) {
	ref := StorageUtils{}.QueueRef(OutboundItem{DomainId: request.DomainId, Id: request.ItemId})
	fields, err := relay.store.Load(ref)
	requeued, requeueErr := outbox.DeadLetters{}.Requeue(fields, request.EventId, request.Subscriber)
	saveErr := relay.save(ref, requeued, errors.Join(err, requeueErr))
	report, relayErr := relay.RelayDocument(ref, request.DomainId, authorize)
	return report, errors.Join(saveErr, relayErr)
}

func (relay *OutboxRelayService) relayEach(documents []outbox.Document, authorize func(string) (string, error)) (RelayReport, error) {
	total := RelayReport{}
	var err error
	for _, document := range documents {
		owner, ownerErr := relay.store.Owner(document.Ref)
		report, relayErr := relay.RelayDocument(document.Ref, owner, authorize)
		total = RelayReport{Documents: total.Documents + report.Documents, Delivered: total.Delivered + report.Delivered, Retried: total.Retried + report.Retried, DeadLettered: total.DeadLettered + report.DeadLettered}
		err = errors.Join(err, ownerErr, relayErr)
	}
	return total, err
}

func (relay *OutboxRelayService) relayOne(ref outbox.Ref, settings outbox.Settings, authorization string, failure error) (outbox.Report, error) {
	if failure != nil {
		return outbox.Report{}, failure
	}
	tokens := outbox.StaticToken{Value: strings.TrimPrefix(authorization, relay.bearer())}
	pusher := outbox.NewHTTPPusher(tokens, relay.settings.Timeout())
	return outbox.NewRelay(relay.store, pusher, settings, time.Now).RelayDocument(ref)
}

func (OutboxRelayService) report(report outbox.Report) RelayReport {
	return RelayReport{Documents: report.Documents, Delivered: report.Delivered, Retried: report.Retried, DeadLettered: report.DeadLettered}
}

func (OutboxRelayService) views(documents []outbox.Document) []PendingDocument {
	views := []PendingDocument{}
	for _, document := range documents {
		views = append(views, PendingDocument{Collection: document.Ref.Collection, Id: document.Ref.Id, OutboxPending: document.Fields.OutboxPending, OutboxDead: document.Fields.OutboxDead})
	}
	return views
}

func (OutboxRelayService) letters(documents []outbox.Document) ([]DeadLetterView, error) {
	views := []DeadLetterView{}
	var err error
	for _, document := range documents {
		listed, listErr := outbox.DeadLetters{}.List(document.Fields)
		views, err = append(views, OutboxUtils{}.DeadLetterViews(listed)...), errors.Join(err, listErr)
	}
	return views, err
}

func (relay *OutboxRelayService) save(ref outbox.Ref, fields outbox.Fields, failure error) error {
	if failure != nil {
		return failure
	}
	return relay.store.Save(ref, fields)
}
