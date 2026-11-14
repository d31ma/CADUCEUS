package outbox

import "time"

type subscriberLane struct {
	subscriber  Subscriber
	events      []PendingEvent
	pusher      Pusher
	now         time.Time
	pushTimeout time.Duration
	deadline    time.Time
}

func (lane subscriberLane) run(messages chan<- laneMessage) {
	defer lane.finish(messages)
	going := true
	for index := 0; going && index < len(lane.events); index++ {
		going = lane.step(lane.events[index], messages)
	}
}

func (lane subscriberLane) finish(messages chan<- laneMessage) {
	messages <- laneMessage{finished: true}
}

func (lane subscriberLane) step(event PendingEvent, messages chan<- laneMessage) bool {
	attempt, answered := lane.pushInTime(event)
	if !answered {
		return false
	}
	messages <- laneMessage{eventId: event.Envelope.EventID, attempt: attempt}
	return attempt.kind() != attemptKind{}.retry()
}

func (lane subscriberLane) pushInTime(event PendingEvent) (pushAttempt, bool) {
	wait := min(lane.pushTimeout, time.Until(lane.deadline))
	if wait <= 0 || lane.notDue(event) {
		return pushAttempt{}, false
	}
	return lane.awaitAnswer(lane.started(event), wait)
}

func (lane subscriberLane) notDue(event PendingEvent) bool {
	nextAttempt, _ := time.Parse(time.RFC3339Nano, event.NextAttemptAt)
	return nextAttempt.After(lane.now)
}

func (lane subscriberLane) started(event PendingEvent) <-chan error {
	answer := make(chan error, 1)
	go func() {
		answer <- lane.pusher.Push(lane.subscriber, event.Envelope)
	}()
	return answer
}

func (lane subscriberLane) awaitAnswer(answer <-chan error, wait time.Duration) (pushAttempt, bool) {
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case err := <-answer:
		return pushAttempt{subscriber: lane.subscriber.Name, err: err}, true
	case <-timer.C:
		return pushAttempt{}, false
	}
}
