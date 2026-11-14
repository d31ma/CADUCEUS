// @RequestRoute("/api/v1/messaging/messages")
// @SignedIn
type MessagesController struct {
	// @Needs
	messages *MessageRouteService
}

// @PostRoute
// @ResponseStatus(201)
func (controller *MessagesController) PostMessage(
	// @FromHeader("authorization")
	authorization string,
	// @FromBody
	request SendRequest,
) (SendResult, error) {
	return controller.messages.Send(authorization, request)
}

// @GetRoute("/{message_id}")
func (controller *MessagesController) GetMessage(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("message_id")
	messageId string,
) (Message, error) {
	return controller.messages.Message(authorization, messageId)
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
	return controller.messages.Edit(authorization, messageId, request)
}

// @DeleteRoute("/{message_id}")
// @ResponseStatus(204)
func (controller *MessagesController) DeleteMessage(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("message_id")
	messageId string,
) error {
	return controller.messages.Delete(authorization, messageId)
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
	return controller.messages.AddReaction(authorization, messageId, reaction)
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
	return controller.messages.RemoveReaction(authorization, messageId, reaction)
}

// @GetRoute("/{message_id}/original")
func (controller *MessagesController) GetOriginal(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("message_id")
	messageId string,
) (Download, error) {
	return controller.messages.Original(authorization, messageId)
}
