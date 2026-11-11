package reception

type Refused struct{}

func (Refused) Error() string {
	return "the message failed DMARC and the sender domain asks to reject it"
}
