import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
	"mesail.dev/packages/fakes"
	"mesail.dev/packages/oidc"
	"mesail.dev/packages/outbox"
)

// @Test(StorageTestSetup, {"partition":"tests"})
type EditEventsTest struct {
	behaviors *BehaviorContext
}

func (test *EditEventsTest) TestManyEditsWhileSearchIsDownLeaveOneWaitingEdit() {
	world := StorageTestSetup{}.World()
	test.retryAfterOneSecond()
	service := test.service(world)
	test.down(world, "search")
	messageId := test.sent(world, service)
	last := test.edited(world, service, messageId, 300, strings.Repeat("words ", 700))
	message := test.stored(world, messageId)
	encoded, _ := json.Marshal(message)
	waiting := test.waiting(world, message, "message.edited")
	Assert.IsTrue(len(encoded) < 512*1024, fmt.Sprintf("the message document has %d bytes", len(encoded)))
	Assert.Equals(1, len(waiting))
	Assert.Equals(last, test.editedBody(world, waiting[0].Envelope))
	Assert.Equals(0, world.Pushed("search", "message.edited"))
	world.Pusher.Recover("search")
	time.Sleep(1100 * time.Millisecond)
	_, firstError := world.Relay.RunOnce()
	_, secondError := world.Relay.RunOnce()
	pushed := world.Pusher.PushedTo("search")
	Assert.IsTrue(firstError == nil && secondError == nil, "the relay failed")
	Assert.Equals(1, world.Pushed("search", "message.edited"))
	Assert.Equals(last, test.editedBody(world, pushed[len(pushed)-1]))
	Assert.Equals(0, test.stored(world, messageId).OutboxPending)
}

func (test *EditEventsTest) TestADeleteDropsTheWaitingEdit() {
	world := StorageTestSetup{}.World()
	service := test.service(world)
	test.down(world, "search")
	messageId := test.sent(world, service)
	test.edited(world, service, messageId, 3, "short")
	deleteError := service.Delete(test.member(world, "ana"), messageId)
	message := test.stored(world, messageId)
	Assert.IsTrue(deleteError == nil, "the delete failed")
	Assert.Equals(0, len(test.waiting(world, message, "message.edited")))
	Assert.Equals(1, len(test.waiting(world, message, "message.deleted")))
}

func (test *EditEventsTest) TestAnEditThatWouldPassTheDocumentCapIsUnavailable() {
	world := StorageTestSetup{}.World()
	service := test.service(world)
	test.down(world, "realtime", "notify", "search")
	messageId := test.sent(world, service)
	message := test.stored(world, messageId)
	large := strings.Repeat("a", 250*1024)
	message.OutboxFields = world.Staged(message.OutboxFields, "message.created", map[string]string{"messageId": "msg_other_1", "body": large})
	message.OutboxFields = world.Staged(message.OutboxFields, "message.created", map[string]string{"messageId": "msg_other_2", "body": large})
	world.Must(world.Messages().Save(message))
	_, err := service.Edit(test.member(world, "ana"), messageId, strings.Repeat("b", 39000))
	Assert.IsTrue(world.IsUnavailable(err), fmt.Sprintf("the edit must be unavailable, got %v", err))
	Assert.Equals(message.Body, test.stored(world, messageId).Body)
}

func (test *EditEventsTest) retryAfterOneSecond() {
	variable := "YON_APPLICATION_JSON"
	retry := `"outbox":{"retry":{"basedelayseconds":1,"maxdelayseconds":1},`
	os.Setenv(variable, strings.Replace(os.Getenv(variable), `"outbox":{`, retry, 1))
}

func (test *EditEventsTest) service(world *StorageTestSetup) *MessagingService {
	fake := fakes.NewFakeWorld()
	settings, err := world.Settings.Outbox()
	world.Must(err)
	ports := fake.Ports
	ports.Stager = core.NewStager(settings, time.Now, rand.Reader)
	for _, memberId := range []string{"ana", "bo"} {
		address := memberId + "@" + world.Domain
		fake.Directory.Member(address, memberId, memberId)
		world.Must(world.Members().Save(core.Member{DomainId: world.Domain, MemberId: memberId, LocalPart: memberId, DisplayName: memberId, PrimaryAddress: address}))
	}
	coreService := &CoreService{storage: world.Storage, settings: world.Settings, fixedPorts: &ports}
	return &MessagingService{core: coreService, failures: &FailureService{}}
}

func (test *EditEventsTest) member(world *StorageTestSetup, memberId string) oidc.Caller {
	return oidc.Caller{DomainId: world.Domain, MemberId: memberId, DisplayName: memberId, Email: memberId + "@" + world.Domain, Roles: []string{"Member"}}
}

func (test *EditEventsTest) sent(world *StorageTestSetup, service *MessagingService) string {
	recipient := core.RecipientChoice{Address: "bo@" + world.Domain, Role: "to", Delivery: "chat", ExpectedKind: "member"}
	request := core.SendRequest{ClientMessageId: "c-" + world.RandomText(), Body: "first", Recipients: []core.RecipientChoice{recipient}}
	outcome, err := service.Send(test.member(world, "ana"), "Bearer unused", request)
	world.Must(err)
	return core.MessageIds{}.Wire(outcome.Message.Message.Month, outcome.Message.Message.Id)
}

func (test *EditEventsTest) edited(world *StorageTestSetup, service *MessagingService, messageId string, count int, filler string) string {
	last := ""
	for index := 0; index < count; index++ {
		last = fmt.Sprintf("edit %04d %s", index, filler)
		_, err := service.Edit(test.member(world, "ana"), messageId, last)
		world.Must(err)
	}
	return last
}

func (test *EditEventsTest) down(world *StorageTestSetup, subscribers ...string) {
	for _, subscriber := range subscribers {
		world.Pusher.FailFor(subscriber, outbox.DeliveryFailure{Subscriber: subscriber, Detail: "down"})
	}
}

func (test *EditEventsTest) stored(world *StorageTestSetup, messageId string) core.Message {
	month, storedId, err := core.MessageIds{}.Parse(messageId)
	world.Must(err)
	message, found, err := world.Messages().Find(world.Domain, month, storedId)
	world.Must(err)
	Assert.IsTrue(found, "the message must be stored")
	return message
}

func (test *EditEventsTest) waiting(world *StorageTestSetup, message core.Message, eventType string) []outbox.PendingEvent {
	pending, err := outbox.Codec{}.DecodePending(message.OutboxFields.Fields().PendingEvents)
	world.Must(err)
	kept := []outbox.PendingEvent{}
	for _, event := range pending {
		kept = append(kept, test.ifType(event, eventType)...)
	}
	return kept
}

func (test *EditEventsTest) ifType(event outbox.PendingEvent, eventType string) []outbox.PendingEvent {
	if event.Envelope.Type != eventType {
		return []outbox.PendingEvent{}
	}
	return []outbox.PendingEvent{event}
}

func (test *EditEventsTest) editedBody(world *StorageTestSetup, envelope events.Envelope) string {
	payload := events.MessageEdited{}
	world.Must(events.PayloadReader{}.Read(envelope, &payload))
	return payload.Body
}
