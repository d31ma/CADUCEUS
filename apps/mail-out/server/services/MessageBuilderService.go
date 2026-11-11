import (
	"errors"
	"time"

	"mesail.dev/packages/compose"
	"mesail.dev/packages/dkim"
)

// @Service
type MessageBuilderService struct {
	// @Needs
	sources *MessageSourceDelegate
	// @Needs
	attachments *AttachmentSourceDelegate
	// @Needs
	blobs *BlobStoreDelegate
	// @Needs
	keys *DkimKeyDelegate
	// @Needs
	directory *DirectoryDelegate
}

func (MessageBuilderService) folder() string { return "/outbound-" }

func (MessageBuilderService) active() string { return "active" }

func (builder *MessageBuilderService) Prepare(item OutboundItem, now time.Time, authorize func(string) (string, error)) (Prepared, error) {
	sender, found, lookupErr := builder.directory.Domain(item.DomainId, item.DomainId, authorize)
	source, sourceErr := builder.sources.Source(item, authorize)
	prepared := Prepared{Item: item, Selector: sender.DkimSelector, Source: source, Refusal: builder.refusal(item, sender, found, source)}
	return builder.build(prepared, errors.Join(lookupErr, sourceErr), now)
}

func (builder *MessageBuilderService) refusal(item OutboundItem, sender Domain, found bool, source MessageSource) string {
	switch {
	case !found:
		return "the sender domain is not hosted"
	case sender.State != builder.active():
		return "the sender domain is not active"
	case source.DomainId != item.DomainId:
		return "the message source belongs to another domain"
	default:
		return ""
	}
}

func (builder *MessageBuilderService) build(prepared Prepared, failure error, now time.Time) (Prepared, error) {
	if failure != nil || prepared.Refusal != "" {
		return prepared, failure
	}
	return builder.compose(prepared, now)
}

func (builder *MessageBuilderService) compose(prepared Prepared, now time.Time) (Prepared, error) {
	item := prepared.Item
	attachments, attachmentErr := builder.attachments.Attachments(item.DomainId, prepared.Source.AttachmentIds)
	key, keyErr := builder.keys.Key(item.DomainId, prepared.Selector)
	raw, composeErr := compose.Composer{}.Compose(builder.message(item, prepared.Source.Body, attachments, now))
	signed, signErr := builder.sign(raw, key, now, errors.Join(composeErr, attachmentErr, keyErr))
	item.RawBlobKey = item.DomainId + builder.folder() + item.Id
	item.Size = int64(len(signed))
	prepared.Item, prepared.Raw = item, string(signed)
	return prepared, builder.store(item, signed, signErr)
}

func (builder *MessageBuilderService) message(item OutboundItem, body string, attachments []compose.Attachment, now time.Time) compose.Message {
	return compose.Message{
		From: compose.Address{Name: item.FromName, Email: item.From}, To: builder.visibleTo(item), Cc: builder.addresses(item.Cc),
		Subject: item.Subject, MessageId: MessageIdUtils{}.Header(item), InReplyTo: MessageIdUtils{}.Bare(item.InReplyTo), References: MessageIdUtils{}.BareAll(item.References),
		Date: now, TextBody: body, Attachments: attachments,
	}
}

func (builder *MessageBuilderService) visibleTo(item OutboundItem) []compose.Address {
	if len(item.To)+len(item.Cc) == 0 {
		return []compose.Address{{Name: item.FromName, Email: item.From}}
	}
	return builder.addresses(item.To)
}

func (MessageBuilderService) addresses(texts []string) []compose.Address {
	addresses := []compose.Address{}
	for _, text := range texts {
		mailbox := AddressUtils{}.Mailbox(text)
		addresses = append(addresses, compose.Address{Name: mailbox.Name, Email: mailbox.Address})
	}
	return addresses
}

func (MessageBuilderService) sign(raw []byte, key dkim.Key, now time.Time, failure error) ([]byte, error) {
	if failure != nil {
		return nil, failure
	}
	return dkim.Signer{}.Sign(dkim.SignRequest{Message: raw, Key: key, Time: now})
}

func (builder *MessageBuilderService) store(item OutboundItem, signed []byte, failure error) error {
	if failure != nil {
		return failure
	}
	return builder.blobs.Put(item.DomainId, item.RawBlobKey, signed)
}
