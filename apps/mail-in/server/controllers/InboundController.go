import (
	"errors"

	"mesail.dev/packages/mailin"
)

// @RequestRoute("/api/v1/mail-in/messages")
// @Allow(Service)
type InboundController struct {
	// @Needs
	workflows *WorkflowService
}

// @PostRoute
// @ResponseStatus(201)
func (controller *InboundController) PostMessage(
	// @FromBody
	envelope InboundEnvelope,
) (InboundAccepted, error) {
	inboundId, failed := controller.workflows.Inbox().Accept(controller.envelope(envelope))
	return InboundAccepted{InboundId: inboundId}, controller.failure(failed)
}

func (controller *InboundController) envelope(envelope InboundEnvelope) mailin.Envelope {
	return mailin.Envelope{
		EnvelopeFrom:  envelope.EnvelopeFrom,
		Recipients:    envelope.Recipients,
		RawBlobKey:    envelope.RawBlobKey,
		Size:          envelope.Size,
		RemoteAddress: envelope.RemoteAddress,
		Helo:          envelope.Helo,
		Tls:           envelope.Tls,
		Headers:       controller.headers(envelope.Headers),
		AuthResults:   controller.authResults(envelope.AuthResults),
	}
}

func (controller *InboundController) headers(headers InboundHeaders) mailin.Headers {
	return mailin.Headers{
		MessageId:  headers.MessageId,
		InReplyTo:  headers.InReplyTo,
		References: headers.References,
		Subject:    headers.Subject,
		From:       controller.mailbox(headers.From),
		To:         controller.mailboxes(headers.To),
		Cc:         controller.mailboxes(headers.Cc),
	}
}

func (controller *InboundController) authResults(results AuthResults) mailin.AuthResults {
	return mailin.AuthResults{
		Spf:         results.Spf,
		Dkim:        results.Dkim,
		Dmarc:       results.Dmarc,
		DmarcPolicy: results.DmarcPolicy,
		Suspicious:  results.Suspicious,
		Handoff:     results.Handoff,
	}
}

func (controller *InboundController) mailboxes(addresses []MailboxAddress) []mailin.Mailbox {
	mailboxes := []mailin.Mailbox{}
	for _, address := range addresses {
		mailboxes = append(mailboxes, controller.mailbox(address))
	}
	return mailboxes
}

func (controller *InboundController) mailbox(address MailboxAddress) mailin.Mailbox {
	return mailin.Mailbox{Name: address.Name, Address: address.Address}
}

func (controller *InboundController) failure(failed error) error {
	invalid := mailin.Invalid{}
	switch {
	case failed == nil:
		return nil
	case errors.As(failed, &invalid):
		return &InvalidEnvelope{Field: invalid.Field, Reason: invalid.Reason}
	}
	return &StorageUnavailable{Operation: failed.Error()}
}
