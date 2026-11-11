package mailin

import "mesail.dev/packages/events"

type Payloads struct{}

func (payloads Payloads) EmailReceived(stored InboundRaw, envelope Envelope) events.EmailReceived {
	return events.EmailReceived{
		InboundId:   stored.WireId(),
		Rcpts:       payloads.listOrEmpty(stored.Recipients),
		RawBlobKey:  stored.RawBlobKey,
		Headers:     payloads.headers(envelope.Headers),
		AuthResults: payloads.authResults(envelope.AuthResults),
	}
}

func (payloads Payloads) headers(headers Headers) events.EmailHeaders {
	return events.EmailHeaders{
		MessageId:  headers.MessageId,
		InReplyTo:  headers.InReplyTo,
		References: payloads.listOrEmpty(headers.References),
		Subject:    headers.Subject,
		From:       headers.From.Address,
		To:         payloads.addresses(headers.To),
		Cc:         payloads.addresses(headers.Cc),
	}
}

func (Payloads) authResults(results AuthResults) events.AuthResults {
	return events.AuthResults{Spf: results.Spf, Dkim: results.Dkim, Dmarc: results.Dmarc, Handoff: results.Handoff}
}

func (Payloads) addresses(mailboxes []Mailbox) []string {
	addresses := []string{}
	for _, mailbox := range mailboxes {
		addresses = append(addresses, mailbox.Address)
	}
	return addresses
}

func (Payloads) listOrEmpty(values []string) []string {
	return append([]string{}, values...)
}
