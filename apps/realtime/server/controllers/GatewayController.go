// @RequestRoute("/api/v1/realtime/gateway")
// @Allow(Gateway)
type GatewayController struct {
	// @Needs
	gateway *GatewayService
}

// @PostRoute("/connect")
// @ResponseStatus(204)
func (controller *GatewayController) PostConnect(
	// @FromBody
	request GatewayConnect,
) error {
	return controller.gateway.Connect(request)
}

// @PostRoute("/message")
func (controller *GatewayController) PostMessage(
	// @FromBody
	request GatewayMessage,
) (GatewayReply, error) {
	return controller.gateway.Message(request)
}

// @PostRoute("/disconnect")
// @ResponseStatus(204)
func (controller *GatewayController) PostDisconnect(
	// @FromBody
	request GatewayDisconnect,
) error {
	return controller.gateway.Disconnect(request)
}
