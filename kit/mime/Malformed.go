package mime

type Malformed struct{}

func (Malformed) Error() string {
	return "the message header block cannot be read"
}

func (Malformed) Number() int {
	return 33
}
