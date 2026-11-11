import (
	"errors"

	"mesail.dev/packages/mailin"
	"mesail.dev/packages/richtext"
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

// @GetRoute("/{inbound_id}/original")
func (controller *InboundController) GetOriginal(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("inbound_id")
	inboundId string,
) (Download, error) {
	download, failed := controller.workflows.Messages().Original(authorization, inboundId)
	return Download{Url: download.Url, ExpiresAt: download.ExpiresAt}, controller.failure(failed)
}

// @GetRoute("/{inbound_id}/content")
func (controller *InboundController) GetContent(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("inbound_id")
	inboundId string,
) (InboundContent, error) {
	content, failed := controller.workflows.Messages().Content(authorization, inboundId)
	return controller.content(content), controller.failure(failed)
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

func (controller *InboundController) content(content mailin.InboundContent) InboundContent {
	return InboundContent{
		Text:        content.Text,
		BodyFormat:  content.BodyFormat,
		Blocks:      controller.blocks(content.Blocks),
		Truncated:   content.Truncated,
		Attachments: controller.attachments(content.Attachments),
	}
}

func (controller *InboundController) blocks(blocks []richtext.Block) []Block {
	converted := []Block{}
	for _, block := range blocks {
		converted = append(converted, controller.block(block))
	}
	return converted
}

func (controller *InboundController) block(block richtext.Block) Block {
	return Block{
		Kind:    block.Kind,
		Level:   block.Level,
		Ordered: block.Ordered,
		Spans:   controller.spans(block.Spans),
		Image:   controller.image(block.Image),
		Cells:   append([]string{}, block.Cells...),
	}
}

func (controller *InboundController) spans(spans []richtext.Span) []Span {
	converted := []Span{}
	for _, span := range spans {
		converted = append(converted, Span{Text: span.Text, Bold: span.Bold, Italic: span.Italic, Code: span.Code, Strike: span.Strike, Href: span.Href})
	}
	return converted
}

func (controller *InboundController) image(image *richtext.ImageRef) *ImageRef {
	if image == nil {
		return nil
	}
	return &ImageRef{Alt: image.Alt, AttachmentId: image.AttachmentId, RemoteUrl: image.RemoteUrl}
}

func (controller *InboundController) attachments(attachments []mailin.InboundAttachment) []InboundAttachment {
	converted := []InboundAttachment{}
	for _, attachment := range attachments {
		converted = append(converted, InboundAttachment{FileName: attachment.FileName, MediaType: attachment.MediaType, Size: attachment.Size, BlobKey: attachment.BlobKey, ContentId: attachment.ContentId})
	}
	return converted
}

func (controller *InboundController) failure(failed error) error {
	invalid := mailin.Invalid{}
	missing := mailin.Missing{}
	switch {
	case failed == nil:
		return nil
	case errors.As(failed, &invalid):
		return &InvalidEnvelope{Field: invalid.Field, Reason: invalid.Reason}
	case errors.As(failed, &missing):
		return &NotFound{Message: missing.Message}
	case errors.Is(failed, mailin.Unauthenticated{}):
		return &Unauthenticated{}
	}
	return &StorageUnavailable{Operation: failed.Error()}
}
