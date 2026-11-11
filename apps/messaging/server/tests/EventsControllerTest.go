import (
	"encoding/json"
	"errors"
)

// @Test(MessagingTestSetup, {"partition":"tests"})
type EventsControllerTest struct {
	behaviors *BehaviorContext
}

func (test *EventsControllerTest) TestPostEventForAnotherDomainIsNotFound() {
	world := MessagingTestSetup{}.World()
	envelope := EventEnvelope{EventId: "evt_1", Type: "member.upserted", DomainId: world.OtherDomain(), Producer: "directory", OccurredAt: "2026-11-10T12:00:00Z", Payload: json.RawMessage(`{}`)}
	_, err := world.Events.PostEvent(world.ServiceBearer(world.Home), envelope)
	Assert.IsTrue(world.IsNotFound(err), "want NotFound")
}

func (test *EventsControllerTest) TestPostEventRefusesAMemberToken() {
	world := MessagingTestSetup{}.World()
	envelope := EventEnvelope{EventId: "evt_1", Type: "member.upserted", DomainId: world.Domain, Producer: "directory", OccurredAt: "2026-11-10T12:00:00Z", Payload: json.RawMessage(`{}`)}
	_, err := world.Events.PostEvent(world.MemberBearer("bo"), envelope)
	Assert.IsTrue(world.IsForbidden(err), "want Forbidden")
}

func (test *EventsControllerTest) TestPostEventRefusesAMissingToken() {
	world := MessagingTestSetup{}.World()
	_, err := world.Events.PostEvent("", EventEnvelope{})
	Assert.IsTrue(world.IsUnauthenticated(err), "want Unauthenticated")
}

func (test *EventsControllerTest) TestPostEventRefusesAnEnvelopeWithoutAnEventId() {
	world := MessagingTestSetup{}.World()
	envelope := EventEnvelope{Type: "member.upserted", DomainId: world.Domain, Producer: "directory", OccurredAt: "2026-11-10T12:00:00Z", Payload: json.RawMessage(`{}`)}
	_, err := world.Events.PostEvent(world.ServiceBearer(world.Home), envelope)
	invalid := &InvalidRequest{}
	Assert.IsTrue(errors.As(err, &invalid), "want InvalidRequest")
	Assert.Equals("the event envelope is not valid: eventId must not be empty", invalid.Message)
}
