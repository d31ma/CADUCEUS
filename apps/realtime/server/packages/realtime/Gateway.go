package realtime

import (
	"encoding/json"
	"errors"
	"strconv"
	"time"
)

type Gateway struct {
	Sessions    Sessions
	Typing      Typing
	Presence    PresenceKeeper
	Connections ConnectionBook
	Pushes      ConnectionGateway
}

func (gateway Gateway) Connect(request GatewayConnect) error {
	return gateway.checked(request)
}

func (gateway Gateway) Message(request GatewayMessage) (GatewayReply, error) {
	return gateway.route(request, gateway.decoded(request.Body))
}

func (gateway Gateway) Disconnect(request GatewayDisconnect) error {
	removed, err := gateway.removed(request)
	if err != nil {
		return err
	}
	return gateway.disconnected(removed)
}

func (gateway Gateway) checked(request GatewayConnect) error {
	switch {
	case request.ConnectionId == "":
		return Invalid{Field: "connectionId", Reason: "a connection id is required"}
	case request.Kind != gateway.Sessions.socketKind() && request.Kind != gateway.Sessions.streamKind():
		return Invalid{Field: "kind", Reason: "the kind is socket or stream"}
	}
	return nil
}

func (Gateway) decoded(body string) ClientMessage {
	message := ClientMessage{}
	if json.Unmarshal([]byte(body), &message) != nil {
		return ClientMessage{}
	}
	return message
}

func (gateway Gateway) route(request GatewayMessage, message ClientMessage) (GatewayReply, error) {
	switch message.Type {
	case gateway.Sessions.authType(), gateway.Sessions.ticketType():
		reply, err := gateway.Sessions.Authenticate(request, message)
		return gateway.answered(request.ConnectionId, reply, err)
	case gateway.pingType():
		return GatewayReply{}, nil
	case gateway.typingType():
		return gateway.typing(request, message)
	case gateway.presenceType():
		return gateway.presence(request, message)
	}
	return GatewayReply{}, Invalid{Field: "type", Reason: "unknown message type"}
}

func (gateway Gateway) answered(connectionId string, reply GatewayReply, err error) (GatewayReply, error) {
	if err != nil {
		return gateway.refused(connectionId, err)
	}
	return gateway.accepted(connectionId, reply)
}

func (gateway Gateway) accepted(connectionId string, reply GatewayReply) (GatewayReply, error) {
	pushed := gateway.Pushes.Push(connectionId, Event{Type: "auth.accepted", State: reply.ExpiresAt})
	return reply, gateway.unlessGone(pushed)
}

func (gateway Gateway) typing(request GatewayMessage, message ClientMessage) (GatewayReply, error) {
	connection, err := gateway.connection(request)
	if err != nil {
		return gateway.refused(request.ConnectionId, err)
	}
	return GatewayReply{}, gateway.Typing.Typing(connection, message.ConversationId)
}

func (gateway Gateway) presence(request GatewayMessage, message ClientMessage) (GatewayReply, error) {
	connection, err := gateway.connection(request)
	if err != nil {
		return gateway.refused(request.ConnectionId, err)
	}
	presence := PresenceRequest{State: message.State, DoNotDisturbUntil: message.DoNotDisturbUntil}
	return GatewayReply{}, gateway.Presence.Set(connection.DomainId, connection.MemberId, presence)
}

func (gateway Gateway) connection(request GatewayMessage) (Connection, error) {
	err := gateway.signedIn(request.DomainId)
	if err != nil {
		return Connection{}, err
	}
	connection, err := gateway.Connections.Find(request.DomainId, request.ConnectionId)
	if err != nil {
		return Connection{}, gateway.Sessions.unknown(err)
	}
	return connection, gateway.current(connection)
}

func (Gateway) signedIn(domainId string) error {
	if domainId == "" {
		return Unauthenticated{}
	}
	return nil
}

func (gateway Gateway) current(connection Connection) error {
	expiresAt, _ := time.Parse(time.RFC3339, connection.ExpiresAt)
	if !gateway.Connections.Now().Before(expiresAt) {
		return Unauthenticated{}
	}
	return nil
}

func (gateway Gateway) refused(connectionId string, err error) (GatewayReply, error) {
	if !errors.As(err, &Unauthenticated{}) {
		return GatewayReply{}, err
	}
	return GatewayReply{}, gateway.close(connectionId)
}

func (gateway Gateway) close(connectionId string) error {
	pushed := gateway.Pushes.Push(connectionId, Event{Type: "auth.expired", State: strconv.Itoa(gateway.refusedCode())})
	closed := gateway.Pushes.Close(connectionId, gateway.refusedCode())
	return errors.Join(gateway.unlessGone(pushed), gateway.unlessGone(closed))
}

func (Gateway) unlessGone(err error) error {
	if errors.As(err, &Gone{}) {
		return nil
	}
	return err
}

func (gateway Gateway) removed(request GatewayDisconnect) (Connection, error) {
	if request.DomainId == "" {
		return Connection{}, nil
	}
	return gateway.Connections.Remove(request.DomainId, request.ConnectionId)
}

func (gateway Gateway) disconnected(removed Connection) error {
	if removed.MemberId == "" {
		return nil
	}
	return gateway.Presence.Disconnected(removed.DomainId, removed.MemberId)
}

func (Gateway) refusedCode() int {
	return 4401
}

func (Gateway) pingType() string {
	return "ping"
}

func (Gateway) typingType() string {
	return "typing"
}

func (Gateway) presenceType() string {
	return "presence"
}
