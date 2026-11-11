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
	return service.edit(messageId, body)
}

func (service *ConversationRequestService) Delete(messageId string) error {
	return service.delete(messageId)
}

func (service *ConversationRequestService) SetReaction(messageId string, name string, isMine bool) error {
	return service.changeReaction(service.reactionChange(isMine), messageId, name)
}

func (service *ConversationRequestService) MarkRead(conversationId string, messageId string) error {
	return service.readMarker(messageId)(conversationId, messageId)
}

func (service *ConversationRequestService) Original(messageId string) (MessageDownload, error) {
	return service.original(messageId)
}

func (service *ConversationRequestService) DomainId() string {
	return service.token.DomainId()
}

func (service *ConversationRequestService) MyAddress() string {
	return service.token.MyAddress()
}

func (service *ConversationRequestService) page(conversationId string, cursor string) (MessagePage, error) {
	page := MessagePage{}
	failure := service.token.WithRenewal(func(authorization string) error {
		answer, failure := service.messaging.Messages(conversationId, authorization, cursor, service.pageSize())
		page = answer
		return failure
	})
	return page, failure
}

func (service *ConversationRequestService) edit(messageId string, body string) (Message, error) {
	message := Message{}
	failure := service.token.WithRenewal(func(authorization string) error {
		answer, failure := BehaviorsAnswer[Message](service.messaging.Edit(messageId, authorization, MessageEditRequest{Body: body}))
		message = answer
		return failure
	})
	return message, failure
}

func (service *ConversationRequestService) delete(messageId string) error {
	return service.token.WithRenewal(func(authorization string) error {
		_, failure := service.messaging.Delete(messageId, authorization)
		return failure
	})
}

func (service *ConversationRequestService) changeReaction(change func(BehaviorValue, BehaviorValue, BehaviorValue) (BehaviorValue, error), messageId string, name string) error {
	return service.token.WithRenewal(func(authorization string) error {
		_, failure := change(messageId, name, authorization)
		return failure
	})
}

func (service *ConversationRequestService) original(messageId string) (MessageDownload, error) {
	download := MessageDownload{}
	failure := service.token.WithRenewal(func(authorization string) error {
		answer, failure := service.messaging.Original(messageId, authorization)
		download = answer
		return failure
	})
	return download, failure
}

func (service *ConversationRequestService) reactionChange(isMine bool) func(BehaviorValue, BehaviorValue, BehaviorValue) (BehaviorValue, error) {
	if isMine {
		return service.messaging.RemoveReaction
	}
	return service.messaging.AddReaction
}

func (service *ConversationRequestService) readMarker(messageId string) func(string, string) error {
	if messageId == "" {
		return service.skipRead
	}
	return service.markRead
}

func (service *ConversationRequestService) markRead(conversationId string, messageId string) error {
	return service.token.WithRenewal(func(authorization string) error {
		_, failure := service.messaging.MarkRead(conversationId, authorization, ConversationReadRequest{MessageId: messageId})
		return failure
	})
}

func (service *ConversationRequestService) skipRead(conversationId string, messageId string) error {
	return nil
}

func (service *ConversationRequestService) pageSize() int {
	return 50
}
