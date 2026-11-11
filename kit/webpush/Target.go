package webpush

type Target interface {
	Send(subscription Subscription, message Message) (string, error)
}
