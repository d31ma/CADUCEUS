package core

type Conflict struct {
	Code        string
	Message     string
	ErrorNumber int
}

func (Conflict) MessageDeleted() *Conflict {
	return &Conflict{Code: "message_deleted", Message: "the message is deleted", ErrorNumber: 1545}
}

func (Conflict) ChannelArchived() *Conflict {
	return &Conflict{Code: "conflict", Message: "the channel is archived and read-only", ErrorNumber: 1546}
}

func (failure *Conflict) Error() string {
	return failure.Message
}

func (failure *Conflict) Number() int {
	return failure.ErrorNumber
}
