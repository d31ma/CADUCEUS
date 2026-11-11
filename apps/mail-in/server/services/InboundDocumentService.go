import "mesail.dev/packages/outbox"

// @Service
type InboundDocumentService struct{}

func (service *InboundDocumentService) New(domainId string, envelope InboundEnvelope, receivedAt string) InboundRaw {
	return InboundRaw{
		DomainId:      domainId,
		ReceivedAt:    receivedAt,
		EnvelopeFrom:  envelope.EnvelopeFrom,
		Rcpts:         envelope.Recipients,
		RawBlobKey:    envelope.RawBlobKey,
		Size:          envelope.Size,
		RemoteAddress: envelope.RemoteAddress,
		Helo:          envelope.Helo,
		Tls:           envelope.Tls,
		MessageId:     envelope.Headers.MessageId,
		Subject:       envelope.Headers.Subject,
		FromAddress:   envelope.Headers.From.Address,
		AuthResults:   envelope.AuthResults,
		State:         service.AcceptedState(),
		PendingEvents: []string{},
		DeadEvents:    []string{},
	}
}

func (service *InboundDocumentService) OutboxFields(stored InboundRaw) outbox.Fields {
	return outbox.Fields{PendingEvents: stored.PendingEvents, DeadEvents: stored.DeadEvents, OutboxPending: stored.OutboxPending, OutboxDead: stored.OutboxDead}
}

func (service *InboundDocumentService) EventPatch(fields outbox.Fields) InboundEventPatch {
	return InboundEventPatch{
		PendingEvents: fields.PendingEvents,
		DeadEvents:    fields.DeadEvents,
		OutboxPending: fields.OutboxPending,
		OutboxDead:    fields.OutboxDead,
		EventQueued:   true,
	}
}

func (service *InboundDocumentService) WireId(stored InboundRaw) string {
	return "in_" + stored.ID
}

func (service *InboundDocumentService) AcceptedState() string {
	return "accepted"
}
