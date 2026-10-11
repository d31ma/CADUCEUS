// NotificationsClient is the /notifications route group. The notification
// centre offers push only when this user has no subscription yet; settings
// registers and removes this device's Web Push subscription.
// @HTTPClient("/notifications")
type NotificationsClient interface {
	// @GetRequest("/subscriptions")
	Subscriptions(
		// @RequestHeader("Authorization")
		authorization string,
	) ([]PushSubscriptionSummary, error)

	// @GetRequest("/vapid-public-key")
	VapidKey(
		// @RequestHeader("Authorization")
		authorization string,
	) (VapidKey, error)

	// @PostRequest("/subscriptions")
	Subscribe(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		subscription BehaviorValue,
	) (BehaviorValue, error)

	// @DeleteRequest("/subscriptions")
	Unsubscribe(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		subscription BehaviorValue,
	) (BehaviorValue, error)
}
