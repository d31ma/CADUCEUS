package core

type MessageView struct {
	Message     Message
	Blocks      []Block
	Reactions   map[string]int
	MyReactions []string
	Deliveries  []DeliveryRecord
}
