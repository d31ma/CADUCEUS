package realtime

type Typing struct {
	Members MemberBook
	FanOut  FanOut
}

func (typing Typing) Typing(connection Connection, conversationId string) error {
	return nil
}
