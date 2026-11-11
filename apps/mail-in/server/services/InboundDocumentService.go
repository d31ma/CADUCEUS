import "encoding/json"

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
	}
}

func (service *InboundDocumentService) EventPatch(stored InboundRaw, envelope InboundEnvelope, occurredAt string) (InboundEventPatch, error) {
	encoded, err := json.Marshal(service.event(stored, envelope, occurredAt))
	return InboundEventPatch{PendingEvents: []string{string(encoded)}, EventQueued: true}, err
}

func (service *InboundDocumentService) WireId(stored InboundRaw) string {
	return "in_" + stored.ID
}

func (service *InboundDocumentService) AcceptedState() string {
	return "accepted"
}

func (service *InboundDocumentService) event(stored InboundRaw, envelope InboundEnvelope, occurredAt string) PendingEvent {
	return PendingEvent{
		EventId:    "evt_" + service.WireId(stored) + "_received",
		Type:       "email.received",
		DomainId:   stored.DomainId,
		Producer:   "mail-in",
		OccurredAt: occurredAt,
		Payload:    service.payload(stored, envelope),
	}
}

func (service *InboundDocumentService) payload(stored InboundRaw, envelope InboundEnvelope) EmailReceivedPayload {
	return EmailReceivedPayload{
		InboundId:   service.WireId(stored),
		Rcpts:       stored.Rcpts,
		RawBlobKey:  stored.RawBlobKey,
		Headers:     envelope.Headers,
		AuthResults: stored.AuthResults,
	}
}
