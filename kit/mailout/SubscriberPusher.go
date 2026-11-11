package mailout

import (
	"net/http"
	"slices"

	"mesail.dev/packages/events"
	"mesail.dev/packages/outbox"
)

type SubscriberPusher struct {
	Client ServiceClient
}

func (SubscriberPusher) classSize() int { return 100 }

func (SubscriberPusher) successClass() int { return 2 }

func (SubscriberPusher) refusedClass() int { return 4 }

func (SubscriberPusher) worthRetrying() []int {
	return []int{http.StatusRequestTimeout, http.StatusTooManyRequests}
}

func (pusher SubscriberPusher) Push(subscriber outbox.Subscriber, envelope events.Envelope) error {
	status, err := pusher.Client.Post(subscriber.URL, envelope.DomainId, envelope)
	if err != nil {
		return outbox.DeliveryFailure{Subscriber: subscriber.Name, Detail: err.Error()}
	}
	return pusher.answer(subscriber, status)
}

func (pusher SubscriberPusher) answer(subscriber outbox.Subscriber, status int) error {
	if status/pusher.classSize() == pusher.successClass() {
		return nil
	}
	return outbox.DeliveryFailure{Subscriber: subscriber.Name, Status: status, Permanent: pusher.permanent(status), Detail: http.StatusText(status)}
}

func (pusher SubscriberPusher) permanent(status int) bool {
	refused := status/pusher.classSize() == pusher.refusedClass()
	return refused && !slices.Contains(pusher.worthRetrying(), status)
}
