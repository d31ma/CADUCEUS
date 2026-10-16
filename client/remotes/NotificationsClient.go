// NotificationsClient is the /notifications route group. The notification
// centre offers push only when this user has no subscription yet.
// @HTTPClient("/notifications")
type NotificationsClient interface {
	// @GetRequest("/subscriptions")
	Subscriptions(
		// @RequestHeader("Authorization")
		authorization string,
	) ([]PushSubscriptionSummary, error)
}
