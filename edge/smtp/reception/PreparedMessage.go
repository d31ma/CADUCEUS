package reception

type PreparedMessage struct {
	Transaction Transaction
	Verdict     AuthVerdict
	Headers     InboundHeaders
}

func (message PreparedMessage) StoredContent() []byte {
	return append([]byte(message.Verdict.HeaderField), message.Transaction.Raw...)
}

func (message PreparedMessage) Envelope(group DomainGroup, key string, size int) InboundEnvelope {
	return InboundEnvelope{
		EnvelopeFrom:  message.Transaction.EnvelopeFrom,
		Recipients:    group.Recipients,
		RawBlobKey:    key,
		Size:          int64(size),
		RemoteAddress: message.Transaction.RemoteAddress,
		Helo:          message.Transaction.Helo,
		Tls:           message.Transaction.Tls,
		Headers:       message.Headers,
		AuthResults:   message.Verdict.Results,
	}
}
