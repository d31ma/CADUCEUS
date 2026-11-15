package mime

type TooManyParts struct{}

func (TooManyParts) Error() string {
	return "the message has more parts than the part limit"
}

func (TooManyParts) Number() int {
	return 36
}
