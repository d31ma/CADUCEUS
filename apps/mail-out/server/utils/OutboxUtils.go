import "mesail.dev/packages/outbox"

type OutboxUtils struct{}

func (OutboxUtils) Fields(item OutboundItem) outbox.Fields {
	return outbox.Fields{PendingEvents: item.PendingEvents, DeadEvents: item.DeadEvents, OutboxPending: item.OutboxPending, OutboxDead: item.OutboxDead}
}

func (OutboxUtils) WithFields(item OutboundItem, fields outbox.Fields) OutboundItem {
	item.PendingEvents, item.DeadEvents = fields.PendingEvents, fields.DeadEvents
	item.OutboxPending, item.OutboxDead = fields.OutboxPending, fields.OutboxDead
	item.OutboxWaiting, item.OutboxHasDead = fields.OutboxPending > 0, fields.OutboxDead > 0
	return item
}

func (OutboxUtils) Patch(fields outbox.Fields) OutboxPatch {
	return OutboxPatch{PendingEvents: fields.PendingEvents, DeadEvents: fields.DeadEvents, OutboxPending: fields.OutboxPending, OutboxDead: fields.OutboxDead, OutboxWaiting: fields.OutboxPending > 0, OutboxHasDead: fields.OutboxDead > 0}
}

func (utils OutboxUtils) Documents(collection string, items []OutboundItem) []outbox.Document {
	return utils.documents(collection, items)
}

func (utils OutboxUtils) documents(collection string, items []OutboundItem) []outbox.Document {
	documents := []outbox.Document{}
	for _, item := range items {
		documents = append(documents, outbox.Document{Ref: outbox.Ref{Collection: collection, Id: item.Id}, Fields: utils.Fields(item)})
	}
	return documents
}

func (utils OutboxUtils) DeadLetterViews(letters []outbox.DeadLetter) []DeadLetterView {
	return utils.deadLetterViews(letters)
}

func (OutboxUtils) deadLetterViews(letters []outbox.DeadLetter) []DeadLetterView {
	views := []DeadLetterView{}
	for _, letter := range letters {
		envelope := letter.Envelope
		views = append(views, DeadLetterView{EventId: envelope.EventID, Type: envelope.Type, DomainId: envelope.DomainId, Subscriber: letter.Subscriber, Attempts: letter.Attempts, LastError: letter.LastError, DeadAt: letter.DeadAt})
	}
	return views
}
