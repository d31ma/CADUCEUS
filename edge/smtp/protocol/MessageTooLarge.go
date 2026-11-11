package protocol

type MessageTooLarge struct{}

func (MessageTooLarge) Error() string {
	return "the message is larger than the size limit"
}
