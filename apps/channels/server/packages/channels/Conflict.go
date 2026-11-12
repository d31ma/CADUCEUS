package channels

type Conflict struct {
	Message     string
	ErrorNumber int
}

func (Conflict) NameUsed(name string) *Conflict {
	return &Conflict{Message: "the channel name " + name + " is already used in this domain", ErrorNumber: 2006}
}

func (Conflict) Archived() *Conflict {
	return &Conflict{Message: "the channel is archived and read-only", ErrorNumber: 2015}
}

func (failure *Conflict) Error() string {
	return failure.Message
}

func (failure *Conflict) Number() int {
	return failure.ErrorNumber
}
