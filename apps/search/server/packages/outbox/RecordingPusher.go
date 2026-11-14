package outbox

import (
	"sync"

	"mesail.dev/packages/events"
)

type RecordingPusher struct {
	mutex     sync.Mutex
	attempted []Push
	pushed    []Push
	failures  map[string]error
}

func NewRecordingPusher() *RecordingPusher {
	return &RecordingPusher{attempted: []Push{}, pushed: []Push{}, failures: map[string]error{}}
}

func (pusher *RecordingPusher) Push(subscriber Subscriber, envelope events.Envelope) error {
	pusher.mutex.Lock()
	defer pusher.mutex.Unlock()
	push := Push{Subscriber: subscriber.Name, Envelope: envelope}
	pusher.attempted = append(pusher.attempted, push)
	err := pusher.failures[subscriber.Name]
	if err != nil {
		return err
	}
	pusher.pushed = append(pusher.pushed, push)
	return nil
}

func (pusher *RecordingPusher) Pushed() []Push {
	pusher.mutex.Lock()
	defer pusher.mutex.Unlock()
	return append([]Push{}, pusher.pushed...)
}

func (pusher *RecordingPusher) Attempted() []Push {
	pusher.mutex.Lock()
	defer pusher.mutex.Unlock()
	return append([]Push{}, pusher.attempted...)
}

func (pusher *RecordingPusher) PushedTo(subscriber string) []events.Envelope {
	pusher.mutex.Lock()
	defer pusher.mutex.Unlock()
	return pusher.envelopesTo(subscriber)
}

func (pusher *RecordingPusher) envelopesTo(subscriber string) []events.Envelope {
	envelopes := []events.Envelope{}
	for _, push := range pusher.pushed {
		envelopes = append(envelopes, pusher.ifTo(push, subscriber)...)
	}
	return envelopes
}

func (pusher *RecordingPusher) FailFor(subscriber string, err error) {
	pusher.mutex.Lock()
	defer pusher.mutex.Unlock()
	pusher.failures[subscriber] = err
}

func (pusher *RecordingPusher) Recover(subscriber string) {
	pusher.mutex.Lock()
	defer pusher.mutex.Unlock()
	delete(pusher.failures, subscriber)
}

func (*RecordingPusher) ifTo(push Push, subscriber string) []events.Envelope {
	if push.Subscriber != subscriber {
		return []events.Envelope{}
	}
	return []events.Envelope{push.Envelope}
}
