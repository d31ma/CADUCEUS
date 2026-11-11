package realtime

type Sessions struct {
	Tokens      TokenChecker
	Tickets     TicketBook
	Connections ConnectionBook
	Domains     DomainBook
	Presence    PresenceKeeper
}

func (sessions Sessions) Authenticate(request GatewayMessage, message ClientMessage) (GatewayReply, error) {
	return GatewayReply{}, Unauthenticated{}
}
