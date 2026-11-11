package adapters

import (
	"strings"
	"time"

	"mesail.dev/packages/events"
	"mesail.dev/packages/outbox"
)

type DomainPusher struct {
	tokens  ServiceAuthorizer
	timeout time.Duration
}

func NewDomainPusher(tokens ServiceAuthorizer, timeout time.Duration) DomainPusher {
	return DomainPusher{tokens: tokens, timeout: timeout}
}

func (pusher DomainPusher) Push(subscriber outbox.Subscriber, envelope events.Envelope) error {
	authorization, err := pusher.tokens.Authorization(envelope.DomainId)
	if err != nil {
		return err
	}
	token := outbox.StaticToken{Value: strings.TrimPrefix(authorization, pusher.bearer())}
	return outbox.NewHTTPPusher(token, pusher.timeout).Push(subscriber, envelope)
}

func (DomainPusher) bearer() string {
	return "Bearer "
}
