import "mesail.dev/packages/webpush"

// @Test(JobsTestSetup, {"partition":"tests"})
type PushSenderServiceTest struct {
	behaviors *BehaviorContext
}

func (test *PushSenderServiceTest) TestSendRefusesAPlatformWithoutATarget() {
	setup := JobsTestSetup{}
	domainId := setup.Domain()
	setup.Realms(domainId)
	JobsTestSetup{}.Subscribe(domainId, "member-1", "apns")
	record := JobsTestSetup{}.Stored(setup.Notification(domainId, "member-1", "msg_1"))
	result, err := JobsTestSetup{}.Sender().Send(record, map[string]webpush.Target{}, 1)
	Assert.IsTrue(err == nil, "send failed")
	Assert.Equals([]string{webpush.Outcomes{}.Refused()}, result.Outcomes)
	Assert.Equals(0, result.Written)
}

func (test *PushSenderServiceTest) TestSendKeepsAGoneSubscriptionWithoutWriteRoom() {
	setup := JobsTestSetup{}
	domainId := setup.Domain()
	setup.Realms(domainId)
	server, _ := JobsTestSetup{}.PushServer(410)
	JobsTestSetup{}.UseNotify([]string{"example.com"}, "")
	JobsTestSetup{}.Subscribe(domainId, "member-1", "web")
	record := JobsTestSetup{}.Stored(setup.Notification(domainId, "member-1", "msg_1"))
	targets, _ := JobsTestSetup{}.Targets(JobsTestSetup{}.ClientFor(server)).ForPlatforms()
	result, err := JobsTestSetup{}.Sender().Send(record, targets, 0)
	left, _ := setup.PushStore().ListForMember(domainId, "member-1")
	Assert.IsTrue(err == nil, "send failed")
	Assert.Equals([]string{webpush.Outcomes{}.Gone()}, result.Outcomes)
	Assert.Equals(1, len(left))
}
