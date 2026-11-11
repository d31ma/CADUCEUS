// @RequestRoute("/api/v1/notify/push-subscriptions")
// @SignedIn
type PushSubscriptionsController struct {
	// @Needs
	subscriptions *PushSubscriptionsService
}

// @PostRoute
// @ResponseStatus(201)
func (controller *PushSubscriptionsController) PostSubscription(
	// @FromHeader("authorization")
	authorization string,
	// @FromBody
	subscription PushSubscriptionRequest,
) error {
	return controller.subscriptions.Subscribe(authorization, subscription)
}

// @DeleteRoute
// @ResponseStatus(204)
func (controller *PushSubscriptionsController) DeleteSubscription(
	// @FromHeader("authorization")
	authorization string,
	// @FromBody
	subscription PushSubscriptionRequest,
) error {
	return controller.subscriptions.Unsubscribe(authorization, subscription)
}
