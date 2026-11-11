import "mesail.dev/packages/events"

// @Service
type EventPayloadService struct {
}

func (service *EventPayloadService) EmailReceived(inboundId string, stored InboundRaw, envelope InboundEnvelope) events.EmailReceived {
	return events.EmailReceived{
		InboundId:   inboundId,
		Rcpts:       service.listOrEmpty(stored.Rcpts),
		RawBlobKey:  stored.RawBlobKey,
		Headers:     service.headers(envelope.Headers),
		AuthResults: service.authResults(envelope.AuthResults),
	}
}

func (service *EventPayloadService) headers(headers InboundHeaders) events.EmailHeaders {
	return events.EmailHeaders{
		MessageId:  headers.MessageId,
		InReplyTo:  headers.InReplyTo,
		References: service.listOrEmpty(headers.References),
		Subject:    headers.Subject,
		From:       headers.From.Address,
		To:         service.addresses(headers.To),
		Cc:         service.addresses(headers.Cc),
	}
}

func (service *EventPayloadService) authResults(results AuthResults) events.AuthResults {
	return events.AuthResults{Spf: results.Spf, Dkim: results.Dkim, Dmarc: results.Dmarc, Handoff: results.Handoff}
}

func (service *EventPayloadService) addresses(mailboxes []MailboxAddress) []string {
	addresses := []string{}
	for _, mailbox := range mailboxes {
		addresses = append(addresses, mailbox.Address)
	}
	return addresses
}

func (service *EventPayloadService) listOrEmpty(values []string) []string {
	return append([]string{}, values...)
}
