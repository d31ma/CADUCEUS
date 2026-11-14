package core

import "strconv"

type Unavailable struct {
	Message     string
	Cause       error
	ErrorNumber int
}

func (Unavailable) Store(cause error) *Unavailable {
	return &Unavailable{Message: "the message store is not available", Cause: cause, ErrorNumber: 1551}
}

func (Unavailable) Directory(cause error) *Unavailable {
	return &Unavailable{Message: "the directory is not available", Cause: cause, ErrorNumber: 1552}
}

func (Unavailable) AttachmentsService(cause error) *Unavailable {
	return &Unavailable{Message: "the attachments service is not available", Cause: cause, ErrorNumber: 1553}
}

func (Unavailable) Renderer(cause error) *Unavailable {
	return &Unavailable{Message: "the text renderer is not available", Cause: cause, ErrorNumber: 1554}
}

func (Unavailable) InboundStorage(cause error) *Unavailable {
	return &Unavailable{Message: "the messaging storage is not available", Cause: cause, ErrorNumber: 1555}
}

func (Unavailable) NotStoredYet(what string) *Unavailable {
	return &Unavailable{Message: what + " is not stored yet", ErrorNumber: 1556}
}

func (Unavailable) NoServiceToken(cause error) *Unavailable {
	return &Unavailable{Message: "no service token for the domain", Cause: cause, ErrorNumber: 1557}
}

func (Unavailable) ServiceAnswered(status int) *Unavailable {
	return &Unavailable{Message: "the service answered " + strconv.Itoa(status), ErrorNumber: 1558}
}

func (Unavailable) ServiceDown(cause error) *Unavailable {
	return &Unavailable{Message: "the service is not available", Cause: cause, ErrorNumber: 1559}
}

func (Unavailable) NoResolveRoute() *Unavailable {
	return &Unavailable{Message: "the directory does not know the resolve route", ErrorNumber: 1560}
}

func (Unavailable) RelayAnswered(status int) *Unavailable {
	return &Unavailable{Message: "the outbox relay route answered " + strconv.Itoa(status), ErrorNumber: 1561}
}

func (Unavailable) OutboxSettings(cause error) *Unavailable {
	return &Unavailable{Message: "the outbox settings are missing", Cause: cause, ErrorNumber: 1562}
}

func (failure *Unavailable) Error() string {
	return failure.Message
}

func (failure *Unavailable) Unwrap() error {
	return failure.Cause
}

func (failure *Unavailable) Number() int {
	return failure.ErrorNumber
}
