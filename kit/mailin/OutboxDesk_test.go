package mailin

import (
	"testing"

	"mesail.dev/packages/outbox"
)

func deadLetterInbox(t *testing.T) (*memoryDocuments, *outbox.RecordingPusher, Inbox) {
	memory := newMemory()
	pusher := outbox.NewRecordingPusher()
	pusher.FailFor("messaging", outbox.DeliveryFailure{Subscriber: "messaging", Status: 400, Permanent: true, Detail: "bad"})
	inbox := testInbox(memory, pusher, goodSettings())
	_, err := inbox.Accept(testEnvelope("example.com", "example.com/inbound-1"))
	check(t, err == nil, "accept failed")
	return memory, pusher, inbox
}

func TestARefusedEventBecomesADeadLetterThatTheRelayDeliversAfterARetry(t *testing.T) {
	_, pusher, inbox := deadLetterInbox(t)
	dead, err := inbox.Outbox.DeadEvents()
	check(t, err == nil && len(dead) == 1, "want one dead letter")
	check(t, dead[0].Subscriber == "messaging" && dead[0].Type == "email.received" && dead[0].DocumentId == "D1", "wrong dead letter")
	pusher.Recover("messaging")
	retried := inbox.Outbox.Retry(RetryRequest{Collection: dead[0].Collection, DocumentId: dead[0].DocumentId, EventId: dead[0].EventId, Subscriber: "messaging"})
	report, relayed := inbox.Outbox.RunOnce()
	after, _ := inbox.Outbox.DeadEvents()
	check(t, retried == nil && relayed == nil, "retry or relay failed")
	check(t, report.Delivered == 1 && len(pusher.PushedTo("messaging")) == 1, "not delivered")
	check(t, len(after) == 0, "the dead letter stayed")
}

func TestRetryOfAnUnknownLetterIsMissing(t *testing.T) {
	_, _, inbox := deadLetterInbox(t)
	collection, _ := inbox.Collection("example.com")
	err := inbox.Outbox.Retry(RetryRequest{Collection: collection, DocumentId: "D1", EventId: "evt_missing", Subscriber: "messaging"})
	check(t, isMissing(err), "want missing")
}

func TestRetryOfAnUnknownCollectionIsMissing(t *testing.T) {
	_, _, inbox := deadLetterInbox(t)
	err := inbox.Outbox.Retry(RetryRequest{Collection: "inbound-raw-other", DocumentId: "D1", EventId: "evt_1", Subscriber: "messaging"})
	check(t, isMissing(err), "want missing")
}

func TestRetryOfUnreadableFieldsIsUnavailable(t *testing.T) {
	memory, _, inbox := deadLetterInbox(t)
	collection, _ := inbox.Collection("example.com")
	memory.rows[collection]["D1"]["deadEvents"] = []string{"not json"}
	err := inbox.Outbox.Retry(RetryRequest{Collection: collection, DocumentId: "D1", EventId: "evt_1", Subscriber: "messaging"})
	check(t, isUnavailable(err, "outbox document"), "want outbox document")
}

func TestDeadEventsReportsUnreadableFields(t *testing.T) {
	memory, _, inbox := deadLetterInbox(t)
	collection, _ := inbox.Collection("example.com")
	memory.rows[collection]["D1"]["deadEvents"] = []string{"not json"}
	_, err := inbox.Outbox.DeadEvents()
	check(t, err != nil, "want an error")
}

func TestDeadEventsReportsAFailedStore(t *testing.T) {
	memory, _, inbox := deadLetterInbox(t)
	memory.failOn = "page"
	dead, err := inbox.Outbox.DeadEvents()
	check(t, err != nil && dead != nil && len(dead) == 0, "want an error and an empty list")
}

func TestRunOnceReportsMissingSettings(t *testing.T) {
	desk := testDesk(newMemory(), outbox.NewRecordingPusher(), testEnvironment(nil))
	_, err := desk.RunOnce()
	check(t, isUnavailable(err, "outbox settings"), "want outbox settings")
}

func TestDrainWithoutSettingsPushesNothing(t *testing.T) {
	pusher := outbox.NewRecordingPusher()
	testDesk(newMemory(), pusher, testEnvironment(nil)).Drain(outbox.Ref{Collection: "inbound-raw-a", Id: "D1"})
	check(t, len(pusher.Attempted()) == 0, "want no push")
}

func TestTheSweepSizeIsOneHundred(t *testing.T) {
	check(t, OutboxDesk{}.SweepSize() == 100, "want 100")
}
