package mailout

type HandOff struct{}

func (HandOff) Envelope(item Item) InboundEnvelope {
	ids, addresses := MessageIds{}, Addresses{}
	headers := InboundHeaders{
		MessageId: ids.Header(item), InReplyTo: ids.Bare(item.InReplyTo), References: ids.BareAll(item.References), Subject: item.Subject,
		From: MailboxAddress{Name: item.FromName, Address: item.From}, To: addresses.Mailboxes(item.To), Cc: addresses.Mailboxes(item.Cc),
	}
	results := AuthResults{DmarcPolicy: "none", Handoff: true}
	return InboundEnvelope{EnvelopeFrom: item.From, Recipients: []string{item.Rcpt}, RawBlobKey: item.RawBlobKey, Size: item.Size, RemoteAddress: "internal", Helo: "mail-out", Tls: true, Headers: headers, AuthResults: results}
}
