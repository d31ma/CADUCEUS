package devicebox

import "mesail.dev/packages/outbox"

type Unavailable struct {
	Message     string
	ErrorNumber int
	Cause       error
}

func (Unavailable) DirectoryDown(cause error) *Unavailable {
	return &Unavailable{Message: "The device list is not available.", ErrorNumber: 6019, Cause: cause}
}

func (Unavailable) DirectoryNotWired() *Unavailable {
	return &Unavailable{Message: "The device list is not connected yet.", ErrorNumber: 6020}
}

func (Unavailable) StoreDown(cause error) *Unavailable {
	return &Unavailable{Message: "The mailbox store is not available.", ErrorNumber: 6021, Cause: cause}
}

func (Unavailable) StoreUnreadable(cause error) *Unavailable {
	return &Unavailable{Message: "The mailbox store gave an answer that cannot be read.", ErrorNumber: 6022, Cause: cause}
}

func (Unavailable) OutboxSettingsMissing(cause error) *Unavailable {
	return &Unavailable{Message: "The outbox settings are missing.", ErrorNumber: 6023, Cause: cause}
}

func (Unavailable) OutboxFull(full outbox.OutboxFull) *Unavailable {
	return &Unavailable{Message: "Too many events are waiting. Try again later.", ErrorNumber: full.Number(), Cause: full}
}

func (Unavailable) OutboxUnreadable(cause error) *Unavailable {
	return &Unavailable{Message: "The outbox fields cannot be read.", ErrorNumber: 6030, Cause: cause}
}

func (Unavailable) DirectoryUnreadable(cause error) *Unavailable {
	return &Unavailable{Message: "The device list gave an answer that cannot be read.", ErrorNumber: 6031, Cause: cause}
}

func (failure *Unavailable) Error() string {
	return failure.Message
}

func (failure *Unavailable) Number() int {
	return failure.ErrorNumber
}

func (failure *Unavailable) Unwrap() error {
	return failure.Cause
}
