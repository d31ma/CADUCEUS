package channels

type Unavailable struct {
	Message     string
	Cause       error
	ErrorNumber int
}

func (Unavailable) DirectoryDown(message string, cause error) *Unavailable {
	return &Unavailable{Message: message, Cause: cause, ErrorNumber: 2019}
}

func (Unavailable) StoreDown(cause error) *Unavailable {
	return &Unavailable{Message: "the channel store is not available", Cause: cause, ErrorNumber: 2020}
}

func (Unavailable) StoreUnreadable(cause error) *Unavailable {
	return &Unavailable{Message: "the channel store answered something unreadable", Cause: cause, ErrorNumber: 2021}
}

func (Unavailable) OutboxSettingsMissing(cause error) *Unavailable {
	return &Unavailable{Message: "the outbox settings are missing", Cause: cause, ErrorNumber: 2022}
}

func (Unavailable) OutboxUnreadable(cause error) *Unavailable {
	return &Unavailable{Message: "the outbox document cannot be read", Cause: cause, ErrorNumber: 2023}
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
