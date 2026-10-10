// PushService sends Web Push notifications to the devices a user subscribed.
// (Only the provider new-mail notification is here; hosted mail and
// unsnoozed conversations use the same policy.)
// @Service
type PushService struct {
	// @Needs
	pushSubscriptionsHelper *PushSubscriptionsHelper
	// @Needs
	usersHelper *UsersHelper
}

// NotifyProviderNewEmail delivers one scheduler-claimed provider
// notification and answers true when its durable outbox item is terminal.
func (service *PushService) NotifyProviderNewEmail(notification map[string]any) bool {
	return ProviderPush{Users: service.usersHelper, Subscriptions: service.pushSubscriptionsHelper}.Notify(notification)
}
