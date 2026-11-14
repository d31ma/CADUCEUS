import "errors"

// @Test(HealthTestSetup, {"partition":"tests"})
type EventsControllerTest struct {
	behaviors *BehaviorContext
}

func (test *EventsControllerTest) TestPostEventRefusesAMissingToken() {
	controller := &EventsController{events: &EventService{failures: &ErrorMappingService{}}}
	_, failure := controller.PostEvent("", EventEnvelope{EventId: "evt_1", Type: "channel.archived", DomainId: "example.com"})
	refused := &Unauthenticated{}
	Assert.IsTrue(errors.As(failure, &refused), "an event was accepted without a token")
	Assert.Equals(5011, refused.Number())
}
