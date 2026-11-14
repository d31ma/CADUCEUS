type PushNotConfigured struct {
	ErrorCase
}

func (PushNotConfigured) NoKeys() *PushNotConfigured {
	return &PushNotConfigured{ErrorCase{4516, "unavailable", "", "Web push is not configured on this server."}}
}
