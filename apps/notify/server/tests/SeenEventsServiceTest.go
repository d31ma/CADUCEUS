// @Test(NotifyTestSetup, {"partition":"tests"})
type SeenEventsServiceTest struct {
	behaviors *BehaviorContext
}

func (test *SeenEventsServiceTest) TestRememberedEventIsSeen() {
	domainId := NotifyTestSetup{}.Domain()
	seen := NotifyTestSetup{}.Seen()
	before, err := seen.Seen(domainId, "evt_1")
	remembered := seen.Remember(domainId, "evt_1")
	again := seen.Remember(domainId, "evt_1")
	after, _ := seen.Seen(domainId, "evt_1")
	Assert.IsTrue(err == nil && remembered == nil && again == nil, "seen store failed")
	Assert.IsTrue(!before, "seen too early")
	Assert.IsTrue(after, "not seen")
}

func (test *SeenEventsServiceTest) TestEventsAreSeenPerDomain() {
	seen := NotifyTestSetup{}.Seen()
	seen.Remember(NotifyTestSetup{}.Domain(), "evt_2")
	other, _ := seen.Seen(NotifyTestSetup{}.Domain(), "evt_2")
	Assert.IsTrue(!other, "another domain saw the event")
}
