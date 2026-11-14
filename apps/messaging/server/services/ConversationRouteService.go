import (
	"cmp"
	"net/http"

	"mesail.dev/packages/core"
	"mesail.dev/packages/oidc"
)

// @Service
type ConversationRouteService struct {
	// @Needs
	callers *CallerService
	// @Needs
	requests *RequestService
	// @Needs
	messaging *MessagingService
	// @Needs
	wire *WireService
}

func (service *ConversationRouteService) List(request *http.Request) (ConversationPage, error) {
	caller, refused := service.callers.ChatCaller(service.requests.Authorization(request))
	page, invalid := service.requests.Page(request)
	list, err := service.conversations(caller, page, cmp.Or(refused, invalid))
	return service.wire.ConversationPage(list), err
}

func (service *ConversationRouteService) Messages(conversationId string, request *http.Request) (MessagePage, error) {
	caller, refused := service.callers.ChatCaller(service.requests.Authorization(request))
	query, invalid := service.requests.MessageQuery(conversationId, request)
	list, err := service.messages(caller, query, cmp.Or(refused, invalid))
	return service.wire.MessagePage(list), err
}

func (service *ConversationRouteService) MoveReadPosition(authorization string, conversationId string, request ReadRequest) error {
	caller, refused := service.callers.ChatCaller(authorization)
	return service.read(caller, conversationId, request.MessageId, refused)
}

func (service *ConversationRouteService) conversations(caller oidc.Caller, page core.PageRequest, failure error) (core.ConversationList, error) {
	if failure != nil {
		return core.ConversationList{}, failure
	}
	return service.messaging.Conversations(caller, page)
}

func (service *ConversationRouteService) messages(caller oidc.Caller, query MessageQuery, failure error) (core.MessageList, error) {
	if failure != nil {
		return core.MessageList{}, failure
	}
	return service.messaging.Messages(caller, query)
}

func (service *ConversationRouteService) read(caller oidc.Caller, conversationId string, messageId string, failure error) error {
	if failure != nil {
		return failure
	}
	return service.messaging.MoveReadPosition(caller, conversationId, messageId)
}
