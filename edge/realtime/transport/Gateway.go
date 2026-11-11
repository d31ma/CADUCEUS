package transport

import (
	"context"
	"net/http"

	"mesail.dev/edge/realtime/backend"
	"mesail.dev/edge/realtime/connections"
)

type Gateway struct {
	hub        *connections.Hub
	public     http.Handler
	management http.Handler
}

func NewGateway(settings GatewaySettings, log connections.EventLog) Gateway {
	hub := connections.NewHub()
	client := backend.NewRetryingClient(settings.BackendUrl, settings.Secret, settings.BackendTimeout, settings.BackendRetry)
	relay := connections.NewRelay(hub, client, settings.Session, log)
	public := http.NewServeMux()
	socket := NewSocketHandler(relay, settings.Session)
	public.Handle("GET /api/v1/realtime/socket", NewOriginPolicy(settings.Origins, socket, log))
	public.Handle("GET /api/v1/realtime/stream", NewStreamHandler(relay, settings.Session))
	public.Handle("GET /api/v1/realtime/gateway-health", HealthHandler{})
	management := NewSecretGuard(settings.Secret, NewManagementHandler(hub).Routes())
	return Gateway{hub: hub, public: public, management: management}
}

func (gateway Gateway) Public() http.Handler {
	return gateway.public
}

func (gateway Gateway) Management() http.Handler {
	return gateway.management
}

func (gateway Gateway) Close(ctx context.Context) error {
	return gateway.hub.CloseAll(ctx, connections.CloseCodes{}.GoingAway())
}
