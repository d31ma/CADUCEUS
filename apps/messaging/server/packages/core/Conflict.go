package core

type Conflict struct {
	Message     string
	ErrorNumber int
}

func (Conflict) MessageDeleted() *Conflict {
	return &Conflict{Message: "the message is deleted", ErrorNumber: 1545}
}

func (Conflict) ChannelArchived() *Conflict {
	return &Conflict{Message: "the channel is archived and read-only", ErrorNumber: 1546}
}

func (failure *Conflict) Error() string {
	return failure.Message
}

func (failure *Conflict) Number() int {
	return failure.ErrorNumber
}
