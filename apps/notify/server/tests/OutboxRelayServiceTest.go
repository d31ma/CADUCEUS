import (
	"errors"

	"mesail.dev/packages/outbox"
)

// @Test(NotifyTestSetup, {"partition":"tests"})
type OutboxRelayServiceTest struct {
	behaviors *BehaviorContext
}

func (test *OutboxRelayServiceTest) TestRelayDocumentPushesTheStagedEvent() {
	setup := NotifyTestSetup{}
	domainId := setup.Domain()
	setup.Realms(domainId)
	stored, _ := setup.SummaryStore().InsertIfAbsent(setup.StagedSummary(domainId))
	pusher := outbox.NewRecordingPusher()
	report, err := setup.Relay(pusher).RelayDocument(setup.SummaryStore().Collection(domainId), stored.Id)
	Assert.IsTrue(err == nil, "relay failed")
	Assert.Equals(1, report.Delivered)
	Assert.Equals(1, len(pusher.PushedTo("mail-out")))
}

func (test *OutboxRelayServiceTest) TestRunOnceNeedsAnOperator() {
	setup := NotifyTestSetup{}
	member, _ := setup.Realms(setup.Domain())
	_, err := setup.Relay(outbox.NewRecordingPusher()).RunOnce(setup.Service(member))
	missing := &NotFound{}
	Assert.IsTrue(errors.As(err, &missing))
}

func (test *OutboxRelayServiceTest) TestRunOnceRelaysPendingDocuments() {
	NotifyTestSetup{}.FreshRegistries()
	setup := NotifyTestSetup{}
	domainId := setup.Domain()
	_, operators := setup.Realms(domainId)
	setup.SummaryStore().InsertIfAbsent(setup.StagedSummary(domainId))
	pusher := outbox.NewRecordingPusher()
	report, err := setup.Relay(pusher).RunOnce(setup.Service(operators))
	Assert.IsTrue(err == nil, "run failed")
	Assert.IsTrue(report.Delivered >= 1, "nothing delivered")
}

func (test *OutboxRelayServiceTest) TestDeadEventsListsALetterAndRetryDeadMovesItBack() {
	NotifyTestSetup{}.FreshRegistries()
	setup := NotifyTestSetup{}
	domainId := setup.Domain()
	_, operators := setup.Realms(domainId)
	stored, _ := setup.SummaryStore().InsertIfAbsent(setup.StagedSummary(domainId))
	pusher := outbox.NewRecordingPusher()
	pusher.FailFor("mail-out", outbox.DeliveryFailure{Subscriber: "mail-out", Status: 400, Permanent: true, Detail: "refused"})
	relay := setup.Relay(pusher)
	collection := setup.SummaryStore().Collection(domainId)
	relay.RelayDocument(collection, stored.Id)
	dead, err := relay.DeadEvents(setup.Service(operators))
	letter := test.letterOf(dead, stored.Id)
	retried := relay.RetryDead(setup.Service(operators), RetryRequest{Collection: collection, DocumentId: stored.Id, EventId: letter.EventId, Subscriber: "mail-out"})
	fields, _ := setup.OutboxStore().Load(outbox.Ref{Collection: collection, Id: stored.Id})
	Assert.IsTrue(err == nil, "dead list failed")
	Assert.Equals("mail-out", letter.Subscriber)
	Assert.IsTrue(retried == nil, "retry failed")
	Assert.Equals(1, fields.OutboxPending)
	Assert.Equals(0, fields.OutboxDead)
}

func (test *OutboxRelayServiceTest) TestRetryDeadRefusesAnUnknownLetter() {
	setup := NotifyTestSetup{}
	domainId := setup.Domain()
	_, operators := setup.Realms(domainId)
	stored, _ := setup.SummaryStore().InsertIfAbsent(setup.StagedSummary(domainId))
	err := setup.Relay(outbox.NewRecordingPusher()).RetryDead(setup.Service(operators), RetryRequest{Collection: setup.SummaryStore().Collection(domainId), DocumentId: stored.Id, EventId: "evt_none", Subscriber: "mail-out"})
	missing := &NotFound{}
	Assert.IsTrue(errors.As(err, &missing))
}

func (test *OutboxRelayServiceTest) letterOf(dead []DeadEvent, documentId string) DeadEvent {
	for _, letter := range dead {
		if letter.DocumentId == documentId {
			return letter
		}
	}
	return DeadEvent{}
}
