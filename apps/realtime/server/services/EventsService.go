// @Service
type EventsService struct {
	// @Needs
	fanOutService *FanOutService
	// @Needs
	conversationMembersService *ConversationMembersService
}

func (service *EventsService) Apply(domainId string, eventType string, payload []byte) error {
	return nil
}
