package connections

import (
	"context"
	"time"

	"mesail.dev/edge/realtime/backend"
)

type Relay struct {
	hub      *Hub
	backend  backend.Client
	settings SessionSettings
	log      EventLog
	ids      IdSource
}

func NewRelay(hub *Hub, client backend.Client, settings SessionSettings, log EventLog) Relay {
	return Relay{hub: hub, backend: client, settings: settings, log: log}
}

func (relay Relay) Open(kind string) (*Session, error) {
	session := NewSession(relay.ids.Next(), relay.settings)
	relay.hub.Add(session)
	relay.log.Opened(session.Id(), kind)
	request := backend.ConnectRequest{ConnectionId: session.Id(), Kind: kind}
	err := relay.backend.Connect(context.Background(), request)
	relay.log.BackendFailed(session.Id(), "connect", err)
	return session, err
}

func (relay Relay) Warm() {
	ready := false
	for attempt := 0; attempt < relay.settings.WarmAttempts && !ready; attempt++ {
		ready = relay.tryWarm(attempt)
	}
	relay.log.Warmed(ready)
}

func (relay Relay) tryWarm(attempt int) bool {
	time.Sleep(time.Duration(min(attempt, 1)) * relay.settings.WarmPause)
	request := backend.ConnectRequest{ConnectionId: relay.warmupId(), Kind: Kinds{}.Socket()}
	return relay.backend.Connect(context.Background(), request) == nil
}

func (Relay) warmupId() string {
	return "gateway-warmup"
}

func (relay Relay) Forward(session *Session, body string) {
	request := backend.MessageRequest{ConnectionId: session.Id(), DomainId: session.DomainId(), Body: body}
	reply, err := relay.backend.Message(context.Background(), request)
	relay.log.BackendFailed(session.Id(), "message", err)
	session.Accept(reply.DomainId, reply.ExpiresAt)
}

func (relay Relay) Finish(session *Session) {
	relay.hub.Remove(session)
	request := backend.DisconnectRequest{ConnectionId: session.Id(), DomainId: session.DomainId()}
	err := relay.backend.Disconnect(context.Background(), request)
	relay.log.BackendFailed(session.Id(), "disconnect", err)
	relay.log.Closed(session.Id(), session.CloseCode())
	relay.hub.Finished()
}
