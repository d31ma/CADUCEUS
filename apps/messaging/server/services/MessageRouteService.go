import (
	"mesail.dev/packages/core"
	"mesail.dev/packages/oidc"
)

// @Service
type MessageRouteService struct {
	// @Needs
	callers *CallerService
	// @Needs
	messaging *MessagingService
	// @Needs
	wire *WireService
}

func (service *MessageRouteService) Send(authorization string, request SendRequest) (SendResult, error) {
	caller, refused := service.callers.ChatCaller(authorization)
	outcome, err := service.send(caller, authorization, service.wire.SendRequest(request), refused)
	return service.wire.SendResult(outcome), err
}

func (service *MessageRouteService) Message(authorization string, messageId string) (Message, error) {
	caller, refused := service.callers.ChatCaller(authorization)
	view, err := service.message(caller, messageId, refused)
	return service.wire.Message(view), err
}

func (service *MessageRouteService) Edit(authorization string, messageId string, request EditRequest) (Message, error) {
	caller, refused := service.callers.ChatCaller(authorization)
	view, err := service.edit(caller, messageId, request, refused)
	return service.wire.Message(view), err
}

func (service *MessageRouteService) Delete(authorization string, messageId string) error {
	caller, refused := service.callers.ChatCaller(authorization)
	return service.deleteMessage(caller, messageId, refused)
}

func (service *MessageRouteService) AddReaction(authorization string, messageId string, reaction string) error {
	caller, refused := service.callers.ChatCaller(authorization)
	return service.addReaction(caller, messageId, reaction, refused)
}

func (service *MessageRouteService) RemoveReaction(authorization string, messageId string, reaction string) error {
	caller, refused := service.callers.ChatCaller(authorization)
	return service.removeReaction(caller, messageId, reaction, refused)
}

func (service *MessageRouteService) Original(authorization string, messageId string) (Download, error) {
	caller, refused := service.callers.ChatCaller(authorization)
	download, err := service.original(caller, messageId, refused)
	return service.wire.Download(download), err
}

func (service *MessageRouteService) send(caller oidc.Caller, authorization string, request core.SendRequest, failure error) (core.SendOutcome, error) {
	if failure != nil {
		return core.SendOutcome{}, failure
	}
	return service.messaging.Send(caller, authorization, request)
}

func (service *MessageRouteService) message(caller oidc.Caller, messageId string, failure error) (core.MessageView, error) {
	if failure != nil {
		return core.MessageView{}, failure
	}
	return service.messaging.Message(caller, messageId)
}

func (service *MessageRouteService) edit(caller oidc.Caller, messageId string, request EditRequest, failure error) (core.MessageView, error) {
	if failure != nil {
		return core.MessageView{}, failure
	}
	return service.messaging.Edit(caller, messageId, request.Body, request.IfVersion)
}

func (service *MessageRouteService) deleteMessage(caller oidc.Caller, messageId string, failure error) error {
	if failure != nil {
		return failure
	}
	return service.messaging.Delete(caller, messageId)
}

func (service *MessageRouteService) addReaction(caller oidc.Caller, messageId string, reaction string, failure error) error {
	if failure != nil {
		return failure
	}
	return service.messaging.AddReaction(caller, messageId, reaction)
}

func (service *MessageRouteService) removeReaction(caller oidc.Caller, messageId string, reaction string, failure error) error {
	if failure != nil {
		return failure
	}
	return service.messaging.RemoveReaction(caller, messageId, reaction)
}

func (service *MessageRouteService) original(caller oidc.Caller, messageId string, failure error) (core.Download, error) {
	if failure != nil {
		return core.Download{}, failure
	}
	return service.messaging.Original(caller, messageId)
}
