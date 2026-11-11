package mailout

import (
	"bytes"
	"io"
	"time"

	"mesail.dev/packages/blobs"
	"mesail.dev/packages/compose"
	"mesail.dev/packages/dkim"
)

type Builder struct {
	Directory Directory
	Sources   Sources
	Blobs     blobs.Store
	Keys      dkim.KeyStore
}

func (builder Builder) Prepare(item Item, now time.Time) (Prepared, error) {
	sender, found, err := builder.Directory.Domain(item.DomainId, item.DomainId)
	if err != nil {
		return Prepared{Item: item}, err
	}
	source, err := builder.Sources.Source(item)
	if err != nil {
		return Prepared{Item: item}, err
	}
	refusal := builder.refusal(item, sender, found, source)
	if refusal != "" {
		return Prepared{Item: item, Refusal: refusal}, nil
	}
	return builder.build(item, sender.DkimSelector, source, now)
}

func (Builder) refusal(item Item, sender Domain, found bool, source MessageSource) string {
	switch {
	case !found:
		return "the sender domain is not hosted"
	case sender.State != "active":
		return "the sender domain is not active"
	case source.DomainId != item.DomainId:
		return "the message source belongs to another domain"
	default:
		return ""
	}
}

func (builder Builder) build(item Item, selector string, source MessageSource, now time.Time) (Prepared, error) {
	attachments, err := builder.attachments(item.DomainId, source.AttachmentIds)
	if err != nil {
		return Prepared{Item: item}, err
	}
	key, err := builder.Keys.Key(item.DomainId, selector)
	if err != nil {
		return Prepared{Item: item}, err
	}
	raw, err := compose.Composer{}.Compose(builder.message(item, source.Body, attachments, now))
	if err != nil {
		return Prepared{Item: item}, err
	}
	signed, err := dkim.Signer{}.Sign(dkim.SignRequest{Message: raw, Key: key, Time: now})
	if err != nil {
		return Prepared{Item: item}, err
	}
	blobKey := blobs.Key{DomainId: item.DomainId, Name: "outbound-" + item.Id}
	item.RawBlobKey, item.Size = blobKey.Path(), int64(len(signed))
	return Prepared{Item: item, Raw: signed}, builder.Blobs.Write(blobKey, bytes.NewReader(signed), item.Size)
}

func (builder Builder) attachments(domainId string, attachmentIds []string) ([]compose.Attachment, error) {
	attachments := []compose.Attachment{}
	for _, attachmentId := range attachmentIds {
		reader, err := builder.Blobs.Read(blobs.Key{DomainId: domainId, Name: "attachment-" + attachmentId})
		if err != nil {
			return nil, err
		}
		content, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			return nil, err
		}
		attachments = append(attachments, compose.Attachment{FileName: attachmentId, ContentType: "application/octet-stream", Content: content})
	}
	return attachments, nil
}

func (builder Builder) message(item Item, body string, attachments []compose.Attachment, now time.Time) compose.Message {
	ids := MessageIds{}
	return compose.Message{
		From: compose.Address{Name: item.FromName, Email: item.From}, To: builder.visibleTo(item), Cc: builder.addresses(item.Cc),
		Subject: item.Subject, MessageId: ids.Header(item), InReplyTo: ids.Bare(item.InReplyTo), References: ids.BareAll(item.References),
		Date: now, TextBody: body, Attachments: attachments,
	}
}

func (builder Builder) visibleTo(item Item) []compose.Address {
	if len(item.To)+len(item.Cc) == 0 {
		return []compose.Address{{Name: item.FromName, Email: item.From}}
	}
	return builder.addresses(item.To)
}

func (Builder) addresses(texts []string) []compose.Address {
	addresses := []compose.Address{}
	for _, mailbox := range (Addresses{}).Mailboxes(texts) {
		addresses = append(addresses, compose.Address{Name: mailbox.Name, Email: mailbox.Address})
	}
	return addresses
}
