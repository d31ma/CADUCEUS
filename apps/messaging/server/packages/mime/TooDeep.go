package mime

type TooDeep struct{}

func (TooDeep) Error() string {
	return "the message has more nested parts than the depth limit"
}
