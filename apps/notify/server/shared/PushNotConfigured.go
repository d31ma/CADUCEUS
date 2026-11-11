type PushNotConfigured struct{}

func (failure *PushNotConfigured) Error() string {
	return "web push keys are not configured"
}
