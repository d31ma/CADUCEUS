package core

type ConversationKinds struct{}

func (ConversationKinds) Channel() string {
	return "channel"
}

func (ConversationKinds) Direct() string {
	return "direct"
}
