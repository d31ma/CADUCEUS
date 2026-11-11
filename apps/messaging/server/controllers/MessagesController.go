import (
	"mesail.dev/packages/core"
	"mesail.dev/packages/oidc"
)

// @RequestRoute("/api/v1/messaging/messages")
// @SignedIn
type MessagesController struct {
	// @Needs
	callers *CallerService
	// @Needs
	messaging *MessagingService
	// @Needs
	wire *WireService
}

// @PostRoute
// @ResponseStatus(201)
func (controller *MessagesController) PostMessage(
	// @FromHeader("authorization")
	authorization string,
	// @FromBody
	request SendRequest,
) (SendResult, error) {
	caller, refused := controller.callers.ChatCaller(authorization)
	outcome, err := controller.send(caller, authorization, controller.wire.SendRequest(request), refused)
	return controller.wire.SendResult(outcome), err
}

// @GetRoute("/{message_id}")
func (controller *MessagesController) GetMessage(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("message_id")
	messageId string,
) (Message, error) {
	caller, refused := controller.callers.ChatCaller(authorization)
	view, err := controller.message(caller, messageId, refused)
	return controller.wire.Message(view), err
}

// @PatchRoute("/{message_id}")
func (controller *MessagesController) PatchMessage(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("message_id")
	messageId string,
	// @FromBody
	request EditRequest,
) (Message, error) {
	caller, refused := controller.callers.ChatCaller(authorization)
	view, err := controller.edit(caller, messageId, request.Body, refused)
	return controller.wire.Message(view), err
}

// @DeleteRoute("/{message_id}")
// @ResponseStatus(204)
func (controller *MessagesController) DeleteMessage(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("message_id")
	messageId string,
) error {
	caller, refused := controller.callers.ChatCaller(authorization)
	return controller.deleteMessage(caller, messageId, refused)
}

// @PutRoute("/{message_id}/reactions/{reaction}")
// @ResponseStatus(204)
func (controller *MessagesController) PutReaction(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("message_id")
	messageId string,
	// @FromPath
	reaction string,
) error {
	caller, refused := controller.callers.ChatCaller(authorization)
	return controller.addReaction(caller, messageId, reaction, refused)
}

// @DeleteRoute("/{message_id}/reactions/{reaction}")
// @ResponseStatus(204)
func (controller *MessagesController) DeleteReaction(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("message_id")
	messageId string,
	// @FromPath
	reaction string,
) error {
	caller, refused := controller.callers.ChatCaller(authorization)
	return controller.removeReaction(caller, messageId, reaction, refused)
}

// @GetRoute("/{message_id}/original")
func (controller *MessagesController) GetOriginal(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("message_id")
	messageId string,
) (Download, error) {
	caller, refused := controller.callers.ChatCaller(authorization)
	download, err := controller.original(caller, messageId, refused)
	return controller.wire.Download(download), err
}

func (controller *MessagesController) send(caller oidc.Caller, authorization string, request core.SendRequest, failure error) (core.SendOutcome, error) {
	if failure != nil {
		return core.SendOutcome{}, failure
	}
	return controller.messaging.Send(caller, authorization, request)
}

func (controller *MessagesController) message(caller oidc.Caller, messageId string, failure error) (core.MessageView, error) {
	if failure != nil {
		return core.MessageView{}, failure
	}
	return controller.messaging.Message(caller, messageId)
}

func (controller *MessagesController) edit(caller oidc.Caller, messageId string, body string, failure error) (core.MessageView, error) {
	if failure != nil {
		return core.MessageView{}, failure
	}
	return controller.messaging.Edit(caller, messageId, body)
}

func (controller *MessagesController) deleteMessage(caller oidc.Caller, messageId string, failure error) error {
	if failure != nil {
		return failure
	}
	return controller.messaging.Delete(caller, messageId)
}

func (controller *MessagesController) addReaction(caller oidc.Caller, messageId string, reaction string, failure error) error {
	if failure != nil {
		return failure
	}
	return controller.messaging.AddReaction(caller, messageId, reaction)
}

func (controller *MessagesController) removeReaction(caller oidc.Caller, messageId string, reaction string, failure error) error {
	if failure != nil {
		return failure
	}
	return controller.messaging.RemoveReaction(caller, messageId, reaction)
}

func (controller *MessagesController) original(caller oidc.Caller, messageId string, failure error) (core.Download, error) {
	if failure != nil {
		return core.Download{}, failure
	}
	return controller.messaging.Original(caller, messageId)
}
