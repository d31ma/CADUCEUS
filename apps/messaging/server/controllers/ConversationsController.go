import "net/http"

// @RequestRoute("/api/v1/messaging/conversations")
// @SignedIn
type ConversationsController struct {
	// @Needs
	conversations *ConversationRouteService
}

// @GetRoute
func (controller *ConversationsController) GetConversations(request *http.Request) (ConversationPage, error) {
	return controller.conversations.List(request)
}

// @GetRoute("/{conversation_id}/messages")
func (controller *ConversationsController) GetMessages(
	// @FromPath("conversation_id")
	conversationId string,
	request *http.Request,
) (MessagePage, error) {
	return controller.conversations.Messages(conversationId, request)
}

// @PutRoute("/{conversation_id}/read")
// @ResponseStatus(204)
func (controller *ConversationsController) PutRead(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("conversation_id")
	conversationId string,
	// @FromBody
	request ReadRequest,
) error {
	return controller.conversations.MoveReadPosition(authorization, conversationId, request)
}
