import (
	"encoding/json"

	"mesail.dev/packages/events"
	"mesail.dev/packages/oidc"
)

// @Test(MessagingTestSetup, {"partition":"tests"})
type EventsServiceTest struct {
	behaviors *BehaviorContext
}

func (test *EventsServiceTest) TestReceiveRefusesAServiceOfAnotherDomain() {
	world := MessagingTestSetup{}.World()
	service := oidc.Caller{DomainId: world.Domain, Roles: []string{"Service"}}
	envelope := events.Envelope{EventID: "evt_1", Type: "member.upserted", DomainId: world.OtherDomain(), Producer: "directory", OccurredAt: "2026-11-10T12:00:00Z", Payload: json.RawMessage(`{}`)}
	_, err := world.EventsService().Receive(service, envelope)
	Assert.IsTrue(world.IsNotFound(err), "want NotFound")
}

func (test *EventsServiceTest) TestReceiveRefusesACallerWithoutTheServiceRole() {
	world := MessagingTestSetup{}.World()
	envelope := events.Envelope{EventID: "evt_1", Type: "member.upserted", DomainId: world.OtherDomain(), Producer: "directory", OccurredAt: "2026-11-10T12:00:00Z", Payload: json.RawMessage(`{}`)}
	_, err := world.EventsService().Receive(world.Member("bo"), envelope)
	Assert.IsTrue(world.IsForbidden(err), "want Forbidden")
	Assert.Equals(events.NotService{}.Error(), err.Error())
}

func (test *EventsServiceTest) TestReceiveRefusesAnEventThatMessagingDoesNotConsume() {
	world := MessagingTestSetup{}.World()
	service := oidc.Caller{DomainId: world.Domain, Roles: []string{"Service"}}
	envelope := events.Envelope{EventID: "evt_1", Type: "message.created", DomainId: world.Domain, Producer: "messaging", OccurredAt: "2026-11-10T12:00:00Z", Payload: json.RawMessage(`{}`)}
	_, err := world.EventsService().Receive(service, envelope)
	Assert.Equals("", world.InvalidField(err))
	Assert.Equals(events.NotSubscribed{Service: "messaging", Type: "message.created"}.Error(), err.Error())
}
