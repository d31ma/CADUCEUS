import (
	"cmp"
	"net/http"

	"mesail.dev/packages/core"
	"mesail.dev/packages/oidc"
)

// @RequestRoute("/api/v1/messaging/conversations")
// @SignedIn
type ConversationsController struct {
	// @Needs
	callers *CallerService
	// @Needs
	requests *RequestService
	// @Needs
	messaging *MessagingService
	// @Needs
	wire *WireService
}

// @GetRoute
func (controller *ConversationsController) GetConversations(request *http.Request) (ConversationPage, error) {
	caller, refused := controller.callers.ChatCaller(controller.requests.Authorization(request))
	page, invalid := controller.requests.Page(request)
	list, err := controller.conversations(caller, page, cmp.Or(refused, invalid))
	return controller.wire.ConversationPage(list), err
}

// @GetRoute("/{conversationId}/messages")
func (controller *ConversationsController) GetMessages(
	// @FromPath
	conversationId string,
	request *http.Request,
) (MessagePage, error) {
	caller, refused := controller.callers.ChatCaller(controller.requests.Authorization(request))
	query, invalid := controller.requests.MessageQuery(conversationId, request)
	list, err := controller.messages(caller, query, cmp.Or(refused, invalid))
	return controller.wire.MessagePage(list), err
}

// @PutRoute("/{conversationId}/read")
// @ResponseStatus(204)
func (controller *ConversationsController) PutRead(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath
	conversationId string,
	// @FromBody
	request ReadRequest,
) error {
	caller, refused := controller.callers.ChatCaller(authorization)
	return controller.read(caller, conversationId, request.MessageId, refused)
}

func (controller *ConversationsController) conversations(caller oidc.Caller, page core.PageRequest, failure error) (core.ConversationList, error) {
	if failure != nil {
		return core.ConversationList{}, failure
	}
	return controller.messaging.Conversations(caller, page)
}

func (controller *ConversationsController) messages(caller oidc.Caller, query MessageQuery, failure error) (core.MessageList, error) {
	if failure != nil {
		return core.MessageList{}, failure
	}
	return controller.messaging.Messages(caller, query)
}

func (controller *ConversationsController) read(caller oidc.Caller, conversationId string, messageId string, failure error) error {
	if failure != nil {
		return failure
	}
	return controller.messaging.MoveReadPosition(caller, conversationId, messageId)
}
