package transport

import (
	"time"

	"mesail.dev/edge/realtime/backend"
	"mesail.dev/edge/realtime/connections"
)

type GatewaySettings struct {
	BackendUrl     string
	BackendTimeout time.Duration
	BackendRetry   backend.RetryPolicy
	Secret         string
	Origins        []string
	Session        connections.SessionSettings
}
