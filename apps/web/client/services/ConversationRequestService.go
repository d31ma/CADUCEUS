// @Service
type ConversationRequestService struct {
	// @Needs
	messaging *MessagingClient
	// @Needs
	token *ConversationTokenService
}

func (service *ConversationRequestService) FirstPage(conversationId string) (MessagePage, error) {
	return service.page(conversationId, "")
}

func (service *ConversationRequestService) OlderPage(conversationId string, cursor string) (MessagePage, error) {
	return service.page(conversationId, cursor)
}

func (service *ConversationRequestService) Edit(messageId string, body string) (Message, error) {
	authorization, failure := service.token.Authorization()
	if failure != nil {
		return Message{}, failure
	}
	return BehaviorsAnswer[Message](service.messaging.Edit(messageId, authorization, MessageEditRequest{Body: body}))
}

func (service *ConversationRequestService) Delete(messageId string) error {
	authorization, failure := service.token.Authorization()
	if failure != nil {
		return failure
	}
	_, failure = service.messaging.Delete(messageId, authorization)
	return failure
}

func (service *ConversationRequestService) SetReaction(messageId string, name string, isMine bool) error {
	authorization, failure := service.token.Authorization()
	if failure != nil {
		return failure
	}
	_, failure = service.reactionChange(isMine)(messageId, name, authorization)
	return failure
}

func (service *ConversationRequestService) MarkRead(conversationId string, messageId string) error {
	return service.markReadWhenAny(conversationId, messageId)
}

func (service *ConversationRequestService) Original(messageId string) (MessageDownload, error) {
	authorization, failure := service.token.Authorization()
	if failure != nil {
		return MessageDownload{}, failure
	}
	return service.messaging.Original(messageId, authorization)
}

func (service *ConversationRequestService) DomainId() string {
	return service.token.DomainId()
}

func (service *ConversationRequestService) MyAddress() string {
	return service.token.MyAddress()
}

func (service *ConversationRequestService) page(conversationId string, cursor string) (MessagePage, error) {
	authorization, failure := service.token.Authorization()
	if failure != nil {
		return MessagePage{}, failure
	}
	return service.messaging.Messages(conversationId, authorization, cursor, service.pageSize())
}

func (service *ConversationRequestService) reactionChange(isMine bool) func(BehaviorValue, BehaviorValue, BehaviorValue) (BehaviorValue, error) {
	if isMine {
		return service.messaging.RemoveReaction
	}
	return service.messaging.AddReaction
}

func (service *ConversationRequestService) markReadWhenAny(conversationId string, messageId string) error {
	if messageId == "" {
		return nil
	}
	return service.markRead(conversationId, messageId)
}

func (service *ConversationRequestService) markRead(conversationId string, messageId string) error {
	authorization, failure := service.token.Authorization()
	if failure != nil {
		return failure
	}
	_, failure = service.messaging.MarkRead(conversationId, authorization, ConversationReadRequest{MessageId: messageId})
	return failure
}

func (service *ConversationRequestService) pageSize() int {
	return 50
}
