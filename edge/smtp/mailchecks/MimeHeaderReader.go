package mailchecks

import (
	"mesail.dev/edge/smtp/reception"
	"mesail.dev/packages/mime"
)

type MimeHeaderReader struct {
	parser HeaderParser
}

func NewMimeHeaderReader(parser HeaderParser) MimeHeaderReader {
	return MimeHeaderReader{parser: parser}
}

func (reader MimeHeaderReader) Headers(raw []byte) (reception.InboundHeaders, error) {
	headers, err := reader.parser.Headers(raw)
	if err != nil {
		return reception.InboundHeaders{}, err
	}
	return reception.InboundHeaders{
		MessageId:  headers.MessageId,
		InReplyTo:  headers.InReplyTo,
		References: append([]string{}, headers.References...),
		Subject:    headers.Subject,
		From:       reader.mailbox(headers.From),
		To:         reader.mailboxes(headers.To),
		Cc:         reader.mailboxes(headers.Cc),
	}, nil
}

func (MimeHeaderReader) mailbox(mailbox mime.Mailbox) reception.MailboxAddress {
	return reception.MailboxAddress{Name: mailbox.Name, Address: mailbox.Address}
}

func (reader MimeHeaderReader) mailboxes(mailboxes []mime.Mailbox) []reception.MailboxAddress {
	converted := make([]reception.MailboxAddress, 0, len(mailboxes))
	for _, mailbox := range mailboxes {
		converted = append(converted, reader.mailbox(mailbox))
	}
	return converted
}
