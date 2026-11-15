package mime

type TooLarge struct{}

func (TooLarge) Error() string {
	return "the message or its header block is larger than the size limit"
}

func (TooLarge) Number() int {
	return 34
}
