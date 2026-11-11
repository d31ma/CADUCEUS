import (
	"errors"
	"time"

	"mesail.dev/packages/events"
)

// @Test(NotifyTestSetup, {"partition":"tests"})
type EventsServiceTest struct {
	behaviors *BehaviorContext
}

func (test *EventsServiceTest) TestReceiveAppliesAMemberUpsertOnce() {
	setup := NotifyTestSetup{}
	domainId := setup.Domain()
	member, _ := setup.Realms(domainId)
	envelope := setup.Envelope("directory", events.Types{}.MemberUpserted(), domainId, events.MemberUpserted{MemberId: "member-1", DisplayName: "Ana", PrimaryAddress: "ana@" + domainId, State: "active"})
	first, err := setup.Events().Receive(setup.Service(member), envelope)
	second, again := setup.Events().Receive(setup.Service(member), envelope)
	record, _ := setup.SettingsStore().Get(domainId, "member-1")
	Assert.IsTrue(err == nil && again == nil, "receive failed")
	Assert.Equals("applied", first.Status)
	Assert.Equals("duplicate", second.Status)
	Assert.Equals("ana@"+domainId, record.PrimaryAddress)
}

func (test *EventsServiceTest) TestReceiveAcceptsTheOperatorRealm() {
	setup := NotifyTestSetup{}
	domainId := setup.Domain()
	_, operators := setup.Realms(domainId)
	envelope := setup.Envelope("messaging", events.Types{}.ReactionChanged(), domainId, map[string]any{"messageId": "msg_1"})
	receipt, err := setup.Events().Receive(setup.Service(operators), envelope)
	Assert.IsTrue(err == nil, "receive failed")
	Assert.Equals("applied", receipt.Status)
}

func (test *EventsServiceTest) TestReceiveHidesAnotherDomain() {
	setup := NotifyTestSetup{}
	member, _ := setup.Realms(setup.Domain())
	envelope := setup.Envelope("directory", events.Types{}.MemberUpserted(), setup.Domain(), events.MemberUpserted{MemberId: "member-1"})
	_, err := setup.Events().Receive(setup.Service(member), envelope)
	missing := &NotFound{}
	Assert.IsTrue(errors.As(err, &missing))
}

func (test *EventsServiceTest) TestReceiveRefusesAMember() {
	setup := NotifyTestSetup{}
	domainId := setup.Domain()
	member, _ := setup.Realms(domainId)
	envelope := setup.Envelope("directory", events.Types{}.MemberUpserted(), domainId, events.MemberUpserted{MemberId: "member-1"})
	_, err := setup.Events().Receive(setup.Member(member, "member-1"), envelope)
	forbidden := &NotService{}
	Assert.IsTrue(errors.As(err, &forbidden))
}

func (test *EventsServiceTest) TestReceiveRefusesAnEventNotifyDoesNotConsume() {
	setup := NotifyTestSetup{}
	domainId := setup.Domain()
	member, _ := setup.Realms(domainId)
	envelope := setup.Envelope("channels", events.Types{}.ChannelCreated(), domainId, map[string]any{"channelId": "ch_1"})
	_, err := setup.Events().Receive(setup.Service(member), envelope)
	invalid := &InvalidRequest{}
	Assert.IsTrue(errors.As(err, &invalid))
}

func (test *EventsServiceTest) TestHandleRegistersTheDomain() {
	setup := NotifyTestSetup{}
	domainId := setup.Domain()
	envelope := setup.Kit(setup.Envelope("messaging", events.Types{}.MessageEdited(), domainId, map[string]any{"messageId": "msg_1"}))
	err := setup.Events().Handle(envelope)
	domains, _ := setup.Domains().All(100000)
	Assert.IsTrue(err == nil, "handle failed")
	Assert.IsTrue(test.contains(domains, domainId), "domain not registered")
}

func (test *EventsServiceTest) TestDispatchRefusesAnUnknownType() {
	envelope := events.Envelope{Type: "something.else", DomainId: "example.com"}
	err := NotifyTestSetup{}.Handlers().Dispatch(envelope)
	invalid := &InvalidRequest{}
	Assert.IsTrue(errors.As(err, &invalid))
}

func (test *EventsServiceTest) TestReadPositionMovedReadsOlderNotifications() {
	setup := NotifyTestSetup{}
	domainId := setup.Domain()
	stored, _ := setup.NotificationStore().InsertIfAbsent(setup.Notification(domainId, "member-1", "msg_1"))
	later := time.Now().Add(time.Minute).UTC().Format(time.RFC3339Nano)
	envelope := setup.Kit(setup.Envelope("messaging", events.Types{}.ReadPositionMoved(), domainId, events.ReadPositionMoved{ConversationId: "cnv_1", MemberId: "member-1", MessageId: "msg_1", ReadAt: later}))
	err := setup.EventActions().ReadPositionMoved(envelope)
	found, _, _ := setup.NotificationStore().Find(domainId, stored.Id)
	Assert.IsTrue(err == nil, "read position failed")
	Assert.IsTrue(found.ReadAt != "", "not read")
}

func (test *EventsServiceTest) TestReadPositionMovedKeepsNewerNotifications() {
	setup := NotifyTestSetup{}
	domainId := setup.Domain()
	stored, _ := setup.NotificationStore().InsertIfAbsent(setup.Notification(domainId, "member-1", "msg_2"))
	earlier := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)
	envelope := setup.Kit(setup.Envelope("messaging", events.Types{}.ReadPositionMoved(), domainId, events.ReadPositionMoved{ConversationId: "cnv_1", MemberId: "member-1", MessageId: "msg_0", ReadAt: earlier}))
	err := setup.EventActions().ReadPositionMoved(envelope)
	found, _, _ := setup.NotificationStore().Find(domainId, stored.Id)
	Assert.IsTrue(err == nil, "read position failed")
	Assert.Equals("", found.ReadAt)
}

func (test *EventsServiceTest) TestMessageDeletedReadsItsNotifications() {
	setup := NotifyTestSetup{}
	domainId := setup.Domain()
	stored, _ := setup.NotificationStore().InsertIfAbsent(setup.Notification(domainId, "member-1", "msg_3"))
	envelope := setup.Kit(setup.Envelope("messaging", events.Types{}.MessageDeleted(), domainId, events.MessageDeleted{MessageId: "msg_3", ConversationId: "cnv_1", MemberIds: []string{"member-1"}, DeletedAt: time.Now().UTC().Format(time.RFC3339)}))
	err := setup.EventActions().MessageDeleted(envelope)
	found, _, _ := setup.NotificationStore().Find(domainId, stored.Id)
	Assert.IsTrue(err == nil, "delete failed")
	Assert.Equals("skipped", found.PushState)
}

func (test *EventsServiceTest) TestMemberUpsertedRefusesABadPayload() {
	envelope := events.Envelope{Type: events.Types{}.MemberUpserted(), DomainId: "example.com", Payload: []byte(`{"unknown":1}`)}
	err := NotifyTestSetup{}.EventActions().MemberUpserted(envelope)
	Assert.IsTrue(err != nil, "a bad payload was accepted")
}

func (test *EventsServiceTest) TestMessageCreatedReachesTheApplyStep() {
	setup := NotifyTestSetup{}
	envelope := setup.Kit(setup.Envelope("messaging", events.Types{}.MessageCreated(), setup.Domain(), events.MessageCreated{MessageId: "msg_4", ConversationId: "cnv_1", MemberIds: []string{"member-1"}}))
	Assert.IsTrue(setup.EventActions().MessageCreated(envelope) == nil)
}

func (test *EventsServiceTest) contains(domains []string, domainId string) bool {
	for _, found := range domains {
		if found == domainId {
			return true
		}
	}
	return false
}
