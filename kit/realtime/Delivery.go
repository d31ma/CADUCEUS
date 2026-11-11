package realtime

type Delivery struct {
	Event     Event
	MemberIds []string
	ToDomain  bool
}
