package transport

import (
	"time"

	"mesail.dev/edge/realtime/connections"
)

type GatewaySettings struct {
	BackendUrl     string
	BackendTimeout time.Duration
	Secret         string
	Origins        []string
	Session        connections.SessionSettings
}
