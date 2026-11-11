import (
	"crypto/rand"
	"time"

	"mesail.dev/packages/events"
)

type ChangeUtils struct{}

func (ChangeUtils) Envelope(item OutboundItem) (events.Envelope, error) {
	change := events.EmailDeliveryChanged{MessageId: item.MessageId, Rcpt: item.Rcpt, Status: EventStatusUtils{}.ForState(item.State), Detail: item.Detail}
	factory := events.NewEnvelopeFactory(events.Services{}.MailOut(), time.Now, rand.Reader)
	return factory.New(events.Types{}.EmailDeliveryChanged(), item.DomainId, change)
}
