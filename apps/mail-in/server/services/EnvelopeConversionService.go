import (
	"mesail.dev/packages/mailin"
)

// @Service
type EnvelopeConversionService struct {
}

func (service *EnvelopeConversionService) Envelope(envelope InboundEnvelope) mailin.Envelope {
	return mailin.Envelope{
		EnvelopeFrom:  envelope.EnvelopeFrom,
		Recipients:    envelope.Recipients,
		RawBlobKey:    envelope.RawBlobKey,
		Size:          envelope.Size,
		RemoteAddress: envelope.RemoteAddress,
		Helo:          envelope.Helo,
		Tls:           envelope.Tls,
		Headers:       service.headers(envelope.Headers),
		AuthResults:   service.authResults(envelope.AuthResults),
	}
}

func (service *EnvelopeConversionService) headers(headers InboundHeaders) mailin.Headers {
	return mailin.Headers{
		MessageId:  headers.MessageId,
		InReplyTo:  headers.InReplyTo,
		References: headers.References,
		Subject:    headers.Subject,
		From:       service.mailbox(headers.From),
		To:         service.mailboxes(headers.To),
		Cc:         service.mailboxes(headers.Cc),
	}
}

func (service *EnvelopeConversionService) authResults(results AuthResults) mailin.AuthResults {
	return mailin.AuthResults{
		Spf:         results.Spf,
		Dkim:        results.Dkim,
		Dmarc:       results.Dmarc,
		DmarcPolicy: results.DmarcPolicy,
		Suspicious:  results.Suspicious,
		Handoff:     results.Handoff,
	}
}

func (service *EnvelopeConversionService) mailboxes(addresses []MailboxAddress) []mailin.Mailbox {
	mailboxes := []mailin.Mailbox{}
	for _, address := range addresses {
		mailboxes = append(mailboxes, service.mailbox(address))
	}
	return mailboxes
}

func (service *EnvelopeConversionService) mailbox(address MailboxAddress) mailin.Mailbox {
	return mailin.Mailbox{Name: address.Name, Address: address.Address}
}
