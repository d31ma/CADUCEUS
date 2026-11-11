// @Test(NotificationTestSetup, {"partition":"tests"})
type NotificationPresenceApiServiceTest struct {
	behaviors *BehaviorContext
}

func (test *NotificationPresenceApiServiceTest) build(domainId string) *NotificationPresenceApiService {
	return NotificationTestSetup{}.presenceApi(test.behaviors, NotificationTestSetup{}.access(domainId))
}

func (test *NotificationPresenceApiServiceTest) TestAnnounce() {
	Assert.Equals("", test.build("example.com").Announce(PresenceRequest{State: "active"}))
	Assert.Equals("error", test.build("broken.example").Announce(PresenceRequest{State: "active"}))
}

func (test *NotificationPresenceApiServiceTest) TestFindSendsTheMemberIds() {
	presences, problem := test.build("example.com").Find([]string{"m-2", "m-3", "m-x"})
	Assert.Equals("", problem)
	Assert.Equals(2, len(presences))
	Assert.Equals("dnd", presences[1].State)
}

func (test *NotificationPresenceApiServiceTest) TestFindWithoutMembersSendsNothing() {
	presences, problem := test.build("broken.example").Find([]string{})
	Assert.Equals("", problem)
	Assert.Equals(0, len(presences))
}
