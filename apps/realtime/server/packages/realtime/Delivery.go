package realtime

type Delivery struct {
	Event                 Event
	MemberIds             []string
	ToDomain              bool
	ChannelConversationId string
	PersonId              string
	DeviceIds             []string
}
