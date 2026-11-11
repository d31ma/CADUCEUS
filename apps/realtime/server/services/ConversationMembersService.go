// @Service
type ConversationMembersService struct {
	// @Needs
	dataSource DataSource
}

func (service *ConversationMembersService) Members(domainId string, conversationId string) ([]string, error) {
	return []string{}, nil
}

func (service *ConversationMembersService) Replace(domainId string, conversationId string, memberIds []string) error {
	return nil
}

func (service *ConversationMembersService) Add(domainId string, conversationId string, memberId string) error {
	return nil
}

func (service *ConversationMembersService) Remove(domainId string, conversationId string, memberId string) error {
	return nil
}
