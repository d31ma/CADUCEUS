// PushSubscriptionsRecords identifies web push subscriptions: one document
// per endpoint, keyed by the endpoint's SHA-256 hash.
type PushSubscriptionsRecords struct{}

// SubscriptionId answers the id of the subscription to an endpoint.
func (PushSubscriptionsRecords) SubscriptionId(endpoint string) string {
	return Security{}.Sha256Hex(endpoint)
}
