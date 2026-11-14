package outbox

import "mesail.dev/packages/events"

type RecordingPusher struct {
	attempted []Push
	pushed    []Push
	failures  map[string]error
}

func NewRecordingPusher() *RecordingPusher {
	return &RecordingPusher{attempted: []Push{}, pushed: []Push{}, failures: map[string]error{}}
}

func (pusher *RecordingPusher) Push(subscriber Subscriber, envelope events.Envelope) error {
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
	return append([]Push{}, pusher.pushed...)
}

func (pusher *RecordingPusher) Attempted() []Push {
	return append([]Push{}, pusher.attempted...)
}

func (pusher *RecordingPusher) PushedTo(subscriber string) []events.Envelope {
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
	pusher.failures[subscriber] = err
}

func (pusher *RecordingPusher) Recover(subscriber string) {
	delete(pusher.failures, subscriber)
}

func (*RecordingPusher) ifTo(push Push, subscriber string) []events.Envelope {
	if push.Subscriber != subscriber {
		return []events.Envelope{}
	}
	return []events.Envelope{push.Envelope}
}
