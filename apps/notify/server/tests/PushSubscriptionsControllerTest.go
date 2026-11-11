import "errors"

// @Test(NotifyTestSetup, {"partition":"tests"})
type PushSubscriptionsControllerTest struct {
	behaviors *BehaviorContext
}

func (test *PushSubscriptionsControllerTest) TestPostSubscriptionRefusesPlainHttp() {
	setup := NotifyTestSetup{}
	member, _ := setup.Realms(setup.Domain())
	err := (&PushSubscriptionsController{subscriptions: setup.Subscriptions()}).PostSubscription(setup.Member(member, "member-1"), PushSubscriptionRequest{Endpoint: "http://fcm.googleapis.com/x", PublicKey: "k", Auth: "a"})
	invalid := &InvalidRequest{}
	Assert.IsTrue(errors.As(err, &invalid))
}

func (test *PushSubscriptionsControllerTest) TestDeleteSubscriptionOfNothingSucceeds() {
	setup := NotifyTestSetup{}
	member, _ := setup.Realms(setup.Domain())
	err := (&PushSubscriptionsController{subscriptions: setup.Subscriptions()}).DeleteSubscription(setup.Member(member, "member-1"), PushSubscriptionRequest{Endpoint: "https://fcm.googleapis.com/none"})
	Assert.IsTrue(err == nil, "delete failed")
}
