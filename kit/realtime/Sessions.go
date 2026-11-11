package realtime

import "errors"

type Sessions struct {
	Tokens      TokenChecker
	Tickets     TicketBook
	Connections ConnectionBook
	Domains     DomainBook
	Presence    PresenceKeeper
}

func (sessions Sessions) Authenticate(request GatewayMessage, message ClientMessage) (GatewayReply, error) {
	grant, kind, err := sessions.grant(message)
	if err != nil {
		return GatewayReply{}, err
	}
	err = sessions.record(request, grant, kind)
	if err != nil {
		return GatewayReply{}, err
	}
	return GatewayReply{DomainId: grant.DomainId, ExpiresAt: grant.ExpiresAt}, nil
}

func (sessions Sessions) grant(message ClientMessage) (Grant, string, error) {
	switch message.Type {
	case sessions.authType():
		grant, err := sessions.Tokens.Grant(message.Token, sessions.Connections.Now())
		return grant, sessions.socketKind(), err
	case sessions.ticketType():
		grant, err := sessions.Tickets.Redeem(message.Ticket)
		return grant, sessions.streamKind(), err
	}
	return Grant{}, "", Unauthenticated{}
}

func (sessions Sessions) record(request GatewayMessage, grant Grant, kind string) error {
	if request.DomainId == "" {
		return sessions.open(request, grant, kind)
	}
	return sessions.renew(request, grant)
}

func (sessions Sessions) open(request GatewayMessage, grant Grant, kind string) error {
	err := sessions.Domains.Remember(grant.DomainId)
	if err != nil {
		return err
	}
	connection := Connection{ConnectionId: request.ConnectionId, DomainId: grant.DomainId, MemberId: grant.MemberId, Kind: kind, ExpiresAt: grant.ExpiresAt}
	err = sessions.Connections.Add(connection)
	if err != nil {
		return err
	}
	return sessions.Presence.Connected(grant.DomainId, grant.MemberId)
}

func (sessions Sessions) renew(request GatewayMessage, grant Grant) error {
	connection, err := sessions.Connections.Find(request.DomainId, request.ConnectionId)
	if err != nil {
		return sessions.unknown(err)
	}
	err = sessions.sameOwner(connection, grant)
	if err != nil {
		return err
	}
	return sessions.Connections.Renew(request.DomainId, request.ConnectionId, grant.ExpiresAt)
}

func (Sessions) unknown(err error) error {
	if errors.As(err, &Missing{}) {
		return Unauthenticated{}
	}
	return err
}

func (Sessions) sameOwner(connection Connection, grant Grant) error {
	if connection.DomainId != grant.DomainId || connection.MemberId != grant.MemberId {
		return Unauthenticated{}
	}
	return nil
}

func (Sessions) authType() string {
	return "auth"
}

func (Sessions) ticketType() string {
	return "ticket"
}

func (Sessions) socketKind() string {
	return "socket"
}

func (Sessions) streamKind() string {
	return "stream"
}
