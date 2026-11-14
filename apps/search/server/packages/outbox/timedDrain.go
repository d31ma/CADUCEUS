package outbox

import (
	"slices"
	"time"
)

type timedDrain struct {
	pusher       Pusher
	subscribers  map[string]Subscriber
	now          time.Time
	pushTimeout  time.Duration
	drainTimeout time.Duration
}

func (drain timedDrain) answers(pending []PendingEvent) answeredPushes {
	deadline := time.Now().Add(drain.drainTimeout)
	lanes := drain.lanes(pending, deadline)
	messages := make(chan laneMessage, len(pending)*len(lanes)+len(lanes))
	drain.start(lanes, messages)
	return drain.collect(messages, len(lanes), deadline)
}

func (drain timedDrain) lanes(pending []PendingEvent, deadline time.Time) []subscriberLane {
	lanes := []subscriberLane{}
	for _, name := range drain.names(pending) {
		lanes = append(lanes, drain.lane(name, pending, deadline))
	}
	return lanes
}

func (drain timedDrain) names(pending []PendingEvent) []string {
	names := []string{}
	for _, event := range pending {
		names = append(names, event.Waiting...)
	}
	slices.Sort(names)
	return slices.Compact(names)
}

func (drain timedDrain) lane(name string, pending []PendingEvent, deadline time.Time) subscriberLane {
	return subscriberLane{
		subscriber:  Subscriber{Name: name, URL: drain.subscribers[name].URL},
		events:      drain.waitingFor(name, pending),
		pusher:      drain.pusher,
		now:         drain.now,
		pushTimeout: drain.pushTimeout,
		deadline:    deadline,
	}
}

func (drain timedDrain) waitingFor(name string, pending []PendingEvent) []PendingEvent {
	return slices.DeleteFunc(slices.Clone(pending), func(event PendingEvent) bool {
		return !slices.Contains(event.Waiting, name)
	})
}

func (drain timedDrain) start(lanes []subscriberLane, messages chan<- laneMessage) {
	for _, lane := range lanes {
		go lane.run(messages)
	}
}

func (drain timedDrain) collect(messages <-chan laneMessage, lanes int, deadline time.Time) answeredPushes {
	answers := newAnsweredPushes()
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	for finished := 0; finished < lanes; {
		select {
		case message := <-messages:
			finished += answers.take(message)
		case <-timer.C:
			return answers
		}
	}
	return answers
}
