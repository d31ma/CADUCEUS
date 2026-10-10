import "log"

// PushNotices decides whether a delivered or unsnoozed message would reach
// the owner's devices by Web Push, as the former PushService.notify did:
// push is opt-in (WEB_PUSH_DISABLED off and VAPID keys configured) and
// needs a known user with subscriptions. Notifications owns Web Push
// delivery itself (VAPID signing and payload encryption); this minimal
// private version stops at that decision and logs it.
type PushNotices struct {
	Users         *UsersHelper
	Subscriptions *PushSubscriptionsHelper
}

// Notify answers whether the notice is settled for the message's recipient
// (kind is "new-mail" or "unsnoozed").
func (notices PushNotices) Notify(email map[string]any, kind string) bool {
	settings := Settings{}
	if settings.Value("WEB_PUSH_DISABLED") == "true" || settings.Value("VAPID_PUBLIC_KEY") == "" || settings.Value("VAPID_PRIVATE_KEY") == "" {
		return true
	}
	address := RecordValues{}.Text(email["recipient"])
	_, user, err := notices.Users.FindByEmail(address)
	if err != nil || user == nil {
		return err == nil
	}
	userEmail := RecordValues{}.Text(RecordValues{}.Or(user["email"], address))
	subscriptions, err := notices.Subscriptions.ListForAddress(userEmail)
	if err != nil || len(subscriptions) == 0 {
		return err == nil
	}
	log.Printf("[push] %s notice for %s is not delivered: Web Push delivery belongs to notifications", kind, RecordValues{}.Text(email["id"]))
	return false
}
