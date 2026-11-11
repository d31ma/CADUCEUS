package webpush

type NotConfigured struct{}

func (NotConfigured) Error() string {
	return "Web Push VAPID keys are not configured"
}
