package mailin

type Invalid struct {
	Field       string
	Reason      string
	ErrorNumber int
}

func (invalid Invalid) RecipientCount() Invalid {
	return Invalid{Field: invalid.recipientsField(), Reason: "between 1 and 100 recipients are required", ErrorNumber: 2501}
}

func (invalid Invalid) RecipientsInManyDomains() Invalid {
	return Invalid{Field: invalid.recipientsField(), Reason: "all recipients must be in one domain", ErrorNumber: 2502}
}

func (invalid Invalid) RecipientDomain() Invalid {
	return Invalid{Field: invalid.recipientsField(), Reason: "the recipient domain is not valid", ErrorNumber: 2503}
}

func (Invalid) HandOffSender() Invalid {
	return Invalid{Field: "headers.from", Reason: "a hand-off must come from an address of the sender domain", ErrorNumber: 2504}
}

func (invalid Invalid) BlobKeyOwner() Invalid {
	return Invalid{Field: invalid.rawBlobKeyField(), Reason: "the raw blob key must be a blob of the domain that owns it", ErrorNumber: 2505}
}

func (Invalid) Size() Invalid {
	return Invalid{Field: "size", Reason: "the size must be between 1 byte and 64 MiB", ErrorNumber: 2506}
}

func (Invalid) SenderDomainNotHosted() Invalid {
	return Invalid{Field: "envelopeFrom", Reason: "the sender domain is not hosted or not active", ErrorNumber: 2507}
}

func (invalid Invalid) NoRawMessage() Invalid {
	return Invalid{Field: invalid.rawBlobKeyField(), Reason: "no raw message is stored under this key", ErrorNumber: 2508}
}

func (invalid Invalid) Error() string {
	return invalid.Field + ": " + invalid.Reason
}

func (invalid Invalid) Number() int {
	return invalid.ErrorNumber
}

func (Invalid) recipientsField() string {
	return "recipients"
}

func (Invalid) rawBlobKeyField() string {
	return "rawBlobKey"
}
