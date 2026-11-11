package mailout

import (
	"crypto/rand"
	"time"

	"mesail.dev/packages/events"
)

type ChangeEvents struct {
	Now func() time.Time
}

func (changes ChangeEvents) Envelope(item Item) (events.Envelope, error) {
	change := events.EmailDeliveryChanged{MessageId: item.MessageId, Rcpt: item.Rcpt, Status: States{}.EventStatus(item.State), Detail: item.Detail}
	factory := events.NewEnvelopeFactory(events.Services{}.MailOut(), changes.Now, rand.Reader)
	return factory.New(events.Types{}.EmailDeliveryChanged(), item.DomainId, change)
}
