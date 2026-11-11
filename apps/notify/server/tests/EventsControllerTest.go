import (
	"errors"

	"mesail.dev/packages/events"
)

// @Test(NotifyTestSetup, {"partition":"tests"})
type EventsControllerTest struct {
	behaviors *BehaviorContext
}

func (test *EventsControllerTest) TestPostEventAppliesTheEvent() {
	setup := NotifyTestSetup{}
	domainId := setup.Domain()
	member, _ := setup.Realms(domainId)
	envelope := setup.Envelope("messaging", events.Types{}.MessageEdited(), domainId, events.MessageEdited{MessageId: "msg_1"})
	receipt, err := (&EventsController{events: setup.Events()}).PostEvent(setup.Service(member), envelope)
	Assert.IsTrue(err == nil, "post failed")
	Assert.Equals(envelope.EventId, receipt.EventId)
}

func (test *EventsControllerTest) TestPostEventRefusesABadEnvelope() {
	setup := NotifyTestSetup{}
	domainId := setup.Domain()
	member, _ := setup.Realms(domainId)
	_, err := (&EventsController{events: setup.Events()}).PostEvent(setup.Service(member), EventEnvelope{DomainId: domainId, Type: "message.edited"})
	invalid := &InvalidRequest{}
	Assert.IsTrue(errors.As(err, &invalid))
}
