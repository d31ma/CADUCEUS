// @Test(HealthTestSetup, {"partition":"tests"})
type EventsControllerTest struct {
	behaviors *BehaviorContext
}

func (test *EventsControllerTest) TestPostEventRefusesAMissingToken() {
	_, failure := (&EventsController{}).PostEvent("", EventEnvelope{EventId: "evt_1", Type: "channel.archived", DomainId: "example.com"})
	Assert.IsTrue(failure != nil, "an event was accepted without a token")
}
