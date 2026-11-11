import (
	"os"

	"mesail.dev/packages/mailout"
	"mesail.dev/packages/outbox"
)

// @RequestRoute("/api/v1/mail-out/outbox")
// @Allow(Service)
type OutboxController struct {
	// @Needs
	dataSource DataSource
}

// @PostRoute("/relay")
func (controller *OutboxController) PostRelay() (RelayReport, error) {
	report, failed := controller.desk().Relay()
	return controller.report(report), failed
}

// @GetRoute("/pending")
func (controller *OutboxController) GetPending() ([]PendingDocument, error) {
	documents, failed := controller.desk().Pending()
	return controller.pending(documents), failed
}

// @GetRoute("/dead")
func (controller *OutboxController) GetDead() ([]DeadLetterView, error) {
	letters, failed := controller.desk().DeadLetters()
	return controller.dead(letters), failed
}

// @PostRoute("/dead/requeue")
func (controller *OutboxController) PostRequeue(
	// @FromBody
	request RequeueRequest,
) (RelayReport, error) {
	report, failed := controller.desk().Requeue(mailout.RequeueRequest{DomainId: request.DomainId, ItemId: request.ItemId, EventId: request.EventId, Subscriber: request.Subscriber})
	return controller.report(report), failed
}

func (controller *OutboxController) desk() mailout.Desk {
	return mailout.DeskFactory{Environment: os.Getenv}.Desk(controller.dataSource)
}

func (OutboxController) report(report outbox.Report) RelayReport {
	return RelayReport{Documents: report.Documents, Delivered: report.Delivered, Retried: report.Retried, DeadLettered: report.DeadLettered}
}

func (OutboxController) pending(documents []outbox.Document) []PendingDocument {
	views := []PendingDocument{}
	for _, document := range documents {
		views = append(views, PendingDocument{Collection: document.Ref.Collection, Id: document.Ref.Id, OutboxPending: document.Fields.OutboxPending, OutboxDead: document.Fields.OutboxDead})
	}
	return views
}

func (OutboxController) dead(letters []outbox.DeadLetter) []DeadLetterView {
	views := []DeadLetterView{}
	for _, letter := range letters {
		envelope := letter.Envelope
		views = append(views, DeadLetterView{EventId: envelope.EventID, Type: envelope.Type, DomainId: envelope.DomainId, Subscriber: letter.Subscriber, Attempts: letter.Attempts, LastError: letter.LastError, DeadAt: letter.DeadAt})
	}
	return views
}
