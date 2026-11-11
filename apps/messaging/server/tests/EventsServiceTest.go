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
	_, err := (&EventsService{core: world.Core, failures: &FailureService{}}).Receive(service, envelope)
	Assert.IsTrue(world.IsNotFound(err), "want NotFound")
}

func (test *EventsServiceTest) TestReceiveRefusesACallerWithoutTheServiceRole() {
	world := MessagingTestSetup{}.World()
	envelope := events.Envelope{EventID: "evt_1", Type: "member.upserted", DomainId: world.OtherDomain(), Producer: "directory", OccurredAt: "2026-11-10T12:00:00Z", Payload: json.RawMessage(`{}`)}
	_, err := (&EventsService{core: world.Core, failures: &FailureService{}}).Receive(world.Member("bo"), envelope)
	_, notService := err.(events.NotService)
	Assert.IsTrue(notService, "want NotService")
}

func (test *EventsServiceTest) TestReceiveRefusesAnEventThatMessagingDoesNotConsume() {
	world := MessagingTestSetup{}.World()
	service := oidc.Caller{DomainId: world.Domain, Roles: []string{"Service"}}
	envelope := events.Envelope{EventID: "evt_1", Type: "message.created", DomainId: world.Domain, Producer: "messaging", OccurredAt: "2026-11-10T12:00:00Z", Payload: json.RawMessage(`{}`)}
	_, err := (&EventsService{core: world.Core, failures: &FailureService{}}).Receive(service, envelope)
	_, notSubscribed := err.(events.NotSubscribed)
	Assert.IsTrue(notSubscribed, "want NotSubscribed")
}
