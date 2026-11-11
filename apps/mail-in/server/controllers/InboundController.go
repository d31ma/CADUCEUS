// @RequestRoute("/api/v1/mail-in/messages")
// @Allow(Service)
type InboundController struct {
	// @Needs
	inboundService *InboundService
}

// @PostRoute
// @ResponseStatus(201)
func (controller *InboundController) PostMessage(
	// @FromBody
	envelope InboundEnvelope,
) (InboundAccepted, error) {
	return controller.inboundService.Accept(envelope)
}
