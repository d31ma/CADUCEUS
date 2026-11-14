package outbox

type directPushes struct {
	pusher      Pusher
	subscribers map[string]Subscriber
}

func (pushes directPushes) push(event PendingEvent, name string) pushAttempt {
	subscriber := Subscriber{Name: name, URL: pushes.subscribers[name].URL}
	return pushAttempt{subscriber: name, err: pushes.pusher.Push(subscriber, event.Envelope)}
}
