import "errors"

// @Test(NotifyTestSetup, {"partition":"tests"})
type PushSubscriptionsServiceTest struct {
	behaviors *BehaviorContext
}

func (test *PushSubscriptionsServiceTest) request(endpoint string) PushSubscriptionRequest {
	return PushSubscriptionRequest{Endpoint: endpoint, PublicKey: "BPublicKey", Auth: "authsecret"}
}

func (test *PushSubscriptionsServiceTest) TestSubscribeStoresAKnownPushService() {
	setup := NotifyTestSetup{}
	domainId := setup.Domain()
	member, _ := setup.Realms(domainId)
	err := setup.Subscriptions().Subscribe(setup.Member(member, "member-1"), test.request("https://fcm.googleapis.com/fcm/send/abc"))
	listed, _ := setup.PushStore().ListForMember(domainId, "member-1")
	Assert.IsTrue(err == nil, "subscribe failed")
	Assert.Equals(1, len(listed))
	Assert.Equals("web", listed[0].Platform)
}

func (test *PushSubscriptionsServiceTest) TestSubscribeRefusesAnUnknownHost() {
	setup := NotifyTestSetup{}
	member, _ := setup.Realms(setup.Domain())
	err := setup.Subscriptions().Subscribe(setup.Member(member, "member-1"), test.request("https://127.0.0.1/push"))
	invalid := &InvalidRequest{}
	Assert.IsTrue(errors.As(err, &invalid))
	Assert.Equals("endpoint", invalid.Field)
}

func (test *PushSubscriptionsServiceTest) TestSubscribeRefusesMissingKeys() {
	setup := NotifyTestSetup{}
	member, _ := setup.Realms(setup.Domain())
	err := setup.Subscriptions().Subscribe(setup.Member(member, "member-1"), PushSubscriptionRequest{Endpoint: "https://fcm.googleapis.com/fcm/send/abc"})
	invalid := &InvalidRequest{}
	Assert.IsTrue(errors.As(err, &invalid))
}

func (test *PushSubscriptionsServiceTest) TestUnsubscribeRemovesOnlyTheCallersSubscription() {
	setup := NotifyTestSetup{}
	domainId := setup.Domain()
	member, _ := setup.Realms(domainId)
	setup.Subscriptions().Subscribe(setup.Member(member, "member-1"), test.request("https://updates.push.services.mozilla.com/wpush/v2/x"))
	other := setup.Subscriptions().Unsubscribe(setup.Member(member, "member-2"), test.request("https://updates.push.services.mozilla.com/wpush/v2/x"))
	kept, _ := setup.PushStore().ListForMember(domainId, "member-1")
	err := setup.Subscriptions().Unsubscribe(setup.Member(member, "member-1"), test.request("https://updates.push.services.mozilla.com/wpush/v2/x"))
	left, _ := setup.PushStore().ListForMember(domainId, "member-1")
	Assert.IsTrue(other == nil && err == nil, "unsubscribe failed")
	Assert.Equals(1, len(kept))
	Assert.Equals(0, len(left))
}
