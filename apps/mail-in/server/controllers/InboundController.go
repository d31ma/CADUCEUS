// @RequestRoute("/api/v1/mail-in/messages")
// @Allow(Service)
type InboundController struct {
	// @Needs
	messages *InboundService
}

// @PostRoute
// @ResponseStatus(201)
func (controller *InboundController) PostMessage(
	// @FromBody
	envelope InboundEnvelope,
) (InboundAccepted, error) {
	return controller.messages.Accept(envelope)
}

// @GetRoute("/{inbound_id}/original")
func (controller *InboundController) GetOriginal(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("inbound_id")
	inboundId string,
) (Download, error) {
	return controller.messages.Original(authorization, inboundId)
}

// @GetRoute("/{inbound_id}/content")
func (controller *InboundController) GetContent(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("inbound_id")
	inboundId string,
) (InboundContent, error) {
	return controller.messages.Content(authorization, inboundId)
}
