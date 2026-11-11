package core

type DeliveryChannels struct{}

func (DeliveryChannels) Chat() string {
	return "chat"
}

func (DeliveryChannels) Email() string {
	return "email"
}
