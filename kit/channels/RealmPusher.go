package channels

import (
	"strings"
	"time"

	"mesail.dev/packages/events"
	"mesail.dev/packages/oidc"
	"mesail.dev/packages/outbox"
)

type RealmPusher struct {
	Tokens  oidc.ServiceTokens
	Timeout time.Duration
}

func (pusher RealmPusher) Push(subscriber outbox.Subscriber, envelope events.Envelope) error {
	authorization, err := pusher.Tokens.Authorization(envelope.DomainId)
	if err != nil {
		return err
	}
	token := strings.TrimPrefix(authorization, "Bearer ")
	return outbox.NewHTTPPusher(outbox.StaticToken{Value: token}, pusher.Timeout).Push(subscriber, envelope)
}
