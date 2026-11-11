type HandOffUtils struct{}

func (HandOffUtils) remoteAddress() string { return "internal" }

func (HandOffUtils) helo() string { return "mail-out" }

func (HandOffUtils) dmarcPolicy() string { return "none" }

func (utils HandOffUtils) Envelope(item OutboundItem) InboundEnvelope {
	ids := MessageIdUtils{}
	headers := InboundHeaders{
		MessageId: ids.Header(item), InReplyTo: ids.Bare(item.InReplyTo), References: ids.BareAll(item.References), Subject: item.Subject,
		From: MailboxAddress{Name: item.FromName, Address: item.From}, To: utils.mailboxes(item.To), Cc: utils.mailboxes(item.Cc),
	}
	results := AuthResults{DmarcPolicy: utils.dmarcPolicy(), Handoff: true}
	return InboundEnvelope{EnvelopeFrom: item.From, Recipients: []string{item.Rcpt}, RawBlobKey: item.RawBlobKey, Size: item.Size, RemoteAddress: utils.remoteAddress(), Helo: utils.helo(), Tls: true, Headers: headers, AuthResults: results}
}

func (HandOffUtils) mailboxes(texts []string) []MailboxAddress {
	mailboxes := []MailboxAddress{}
	for _, text := range texts {
		mailboxes = append(mailboxes, AddressUtils{}.Mailbox(text))
	}
	return mailboxes
}
