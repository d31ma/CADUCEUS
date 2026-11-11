import (
	"encoding/json"

	"mesail.dev/packages/events"
)

// @Test(MessagingTestSetup, {"partition":"tests"})
type EventsControllerTest struct {
	behaviors *BehaviorContext
}

func (test *EventsControllerTest) TestPostEventForAnotherDomainIsNotFound() {
	world := MessagingTestSetup{}.World()
	envelope := events.Envelope{EventID: "evt_1", Type: "member.upserted", DomainId: world.OtherDomain(), Producer: "directory", OccurredAt: "2026-11-10T12:00:00Z", Payload: json.RawMessage(`{}`)}
	_, err := world.Events.PostEvent(world.ServiceBearer(world.Home), envelope)
	Assert.IsTrue(world.IsNotFound(err), "want NotFound")
}

func (test *EventsControllerTest) TestPostEventRefusesAMemberToken() {
	world := MessagingTestSetup{}.World()
	envelope := events.Envelope{EventID: "evt_1", Type: "member.upserted", DomainId: world.Domain, Producer: "directory", OccurredAt: "2026-11-10T12:00:00Z", Payload: json.RawMessage(`{}`)}
	_, err := world.Events.PostEvent(world.MemberBearer("bo"), envelope)
	Assert.IsTrue(world.IsForbidden(err), "want Forbidden")
}

func (test *EventsControllerTest) TestPostEventRefusesAMissingToken() {
	world := MessagingTestSetup{}.World()
	_, err := world.Events.PostEvent("", events.Envelope{})
	Assert.IsTrue(world.IsUnauthenticated(err), "want Unauthenticated")
}

func (test *EventsControllerTest) TestPostEventRefusesAnEnvelopeWithoutAnEventId() {
	world := MessagingTestSetup{}.World()
	envelope := events.Envelope{Type: "member.upserted", DomainId: world.Domain, Producer: "directory", OccurredAt: "2026-11-10T12:00:00Z", Payload: json.RawMessage(`{}`)}
	_, err := world.Events.PostEvent(world.ServiceBearer(world.Home), envelope)
	_, invalid := err.(events.InvalidEnvelope)
	Assert.IsTrue(invalid, "want InvalidEnvelope")
}
