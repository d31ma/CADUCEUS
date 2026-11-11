// @Service
type TypingService struct {
	// @Needs
	conversationMembersService *ConversationMembersService
	// @Needs
	fanOutService *FanOutService
}

func (service *TypingService) Typing(connection Connection, conversationId string) error {
	return nil
}
