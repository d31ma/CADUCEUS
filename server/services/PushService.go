// PushService sends Web Push notifications for new and unsnoozed mail and
// answers the VAPID public key browsers subscribe with.
// @Service
type PushService struct {
	// @Needs
	pushSubscriptionsHelper *PushSubscriptionsHelper
	// @Needs
	usersHelper *UsersHelper
	// @Needs
	emailsHelper *EmailsHelper
}

// GetVapidPublicKey answers the configured key, or a throwaway one.
func (service *PushService) GetVapidPublicKey() string {
	return service.notices().VapidPublicKey()
}

// NotifyNewEmail notifies every device of the mail's owner (inbound
// delivery calls it); it reports whether the work is finished.
func (service *PushService) NotifyNewEmail(email map[string]any) bool {
	return service.notices().NewEmail(email)
}

// NotifyProviderNewEmail notifies of new mail in a connected provider
// account (the provider notification tick calls it).
func (service *PushService) NotifyProviderNewEmail(notification map[string]any) bool {
	return service.notices().ProviderNewEmail(notification)
}

// NotifyUnsnoozed notifies that a snoozed conversation returned (the snooze
// worker calls it).
func (service *PushService) NotifyUnsnoozed(email map[string]any) bool {
	return service.notices().Unsnoozed(email)
}

func (service *PushService) notices() PushNotices {
	return PushNotices{Users: service.usersHelper, Subscriptions: service.pushSubscriptionsHelper, Emails: service.emailsHelper}
}
