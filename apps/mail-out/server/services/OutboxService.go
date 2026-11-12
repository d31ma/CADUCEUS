import (
	"mesail.dev/packages/mailout"
	"mesail.dev/packages/outbox"
)

// @Service
type OutboxService struct {
	// @Needs
	desks *DeskService
}

func (service *OutboxService) Relay() (RelayReport, error) {
	report, failed := service.desks.Desk().Relay()
	return service.report(report), failed
}

func (service *OutboxService) Pending() ([]PendingDocument, error) {
	documents, failed := service.desks.Desk().Pending()
	return service.pending(documents), failed
}

func (service *OutboxService) Dead() ([]DeadLetterView, error) {
	letters, failed := service.desks.Desk().DeadLetters()
	return service.dead(letters), failed
}

func (service *OutboxService) Requeue(request RequeueRequest) (RelayReport, error) {
	report, failed := service.desks.Desk().Requeue(mailout.RequeueRequest{DomainId: request.DomainId, ItemId: request.ItemId, EventId: request.EventId, Subscriber: request.Subscriber})
	return service.report(report), failed
}

func (service *OutboxService) report(report outbox.Report) RelayReport {
	return RelayReport{Documents: report.Documents, Delivered: report.Delivered, Retried: report.Retried, DeadLettered: report.DeadLettered}
}

func (service *OutboxService) pending(documents []outbox.Document) []PendingDocument {
	views := []PendingDocument{}
	for _, document := range documents {
		views = append(views, PendingDocument{Collection: document.Ref.Collection, Id: document.Ref.Id, OutboxPending: document.Fields.OutboxPending, OutboxDead: document.Fields.OutboxDead})
	}
	return views
}

func (service *OutboxService) dead(letters []outbox.DeadLetter) []DeadLetterView {
	views := []DeadLetterView{}
	for _, letter := range letters {
		envelope := letter.Envelope
		views = append(views, DeadLetterView{EventId: envelope.EventID, Type: envelope.Type, DomainId: envelope.DomainId, Subscriber: letter.Subscriber, Attempts: letter.Attempts, LastError: letter.LastError, DeadAt: letter.DeadAt})
	}
	return views
}
