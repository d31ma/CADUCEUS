type Unavailable struct {
	ErrorCase
	Cause error
}

func (failure *Unavailable) Unwrap() error {
	return failure.Cause
}

func (Unavailable) Dns(cause error) *Unavailable {
	return &Unavailable{ErrorCase{1013, "unavailable", "", "the DNS lookup failed"}, cause}
}

func (Unavailable) IdentityServer(cause error) *Unavailable {
	return &Unavailable{ErrorCase{1033, "unavailable", "", "the identity server is not available"}, cause}
}

func (Unavailable) OutboxSettings(cause error) *Unavailable {
	return &Unavailable{ErrorCase{1044, "unavailable", "", "the outbox is not configured"}, cause}
}

func (Unavailable) Store() *Unavailable {
	return &Unavailable{ErrorCase{1048, "unavailable", "", "the directory store is not available"}, nil}
}
