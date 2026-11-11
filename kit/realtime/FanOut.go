package realtime

type FanOut struct {
	Connections ConnectionBook
	Gateway     ConnectionGateway
}

func (fanOut FanOut) ToMembers(domainId string, memberIds []string, event Event) error {
	return nil
}

func (fanOut FanOut) ToMembersExcept(domainId string, memberIds []string, exceptMemberId string, event Event) error {
	return nil
}

func (fanOut FanOut) ToDomain(domainId string, event Event) error {
	return nil
}

func (fanOut FanOut) ToConnection(connectionId string, event Event) error {
	return nil
}
