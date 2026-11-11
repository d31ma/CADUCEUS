package realtime

type Gateway struct {
	Sessions    Sessions
	Typing      Typing
	Presence    PresenceKeeper
	Connections ConnectionBook
	Pushes      ConnectionGateway
}

func (gateway Gateway) Connect(request GatewayConnect) error {
	return nil
}

func (gateway Gateway) Message(request GatewayMessage) (GatewayReply, error) {
	return GatewayReply{}, nil
}

func (gateway Gateway) Disconnect(request GatewayDisconnect) error {
	return nil
}
