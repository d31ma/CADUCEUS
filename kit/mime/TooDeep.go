package mime

type TooDeep struct{}

func (TooDeep) Error() string {
	return "the message has more nested parts than the depth limit"
}

func (TooDeep) Number() int {
	return 35
}
