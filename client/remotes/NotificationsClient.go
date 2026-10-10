// @HTTPClient("/notifications")
type NotificationsClient interface {
	// @GetRequest("/subscriptions")
	Subscriptions(
		// @RequestHeader("Authorization")
		authorization string,
	) ([]PushSubscriptionRecord, error)

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
		body BehaviorValue,
	) (BehaviorValue, error)

	// @DeleteRequest("/subscriptions")
	Unsubscribe(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		body BehaviorValue,
	) (BehaviorValue, error)
}
