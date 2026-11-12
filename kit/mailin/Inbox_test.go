package mailin

import (
	"testing"

	"mesail.dev/packages/events"
	"mesail.dev/packages/outbox"
)

func storedRecord(t *testing.T, inbox Inbox, key string) InboundRaw {
	collection, _ := inbox.Collection("example.com")
	found, err := inbox.FindByBlobKey(collection, key)
	check(t, err == nil && len(found) == 1, "want one stored record")
	return found[0]
}

func TestAcceptStoresTheMessageOnce(t *testing.T) {
	inbox := testInbox(newMemory(), outbox.NewRecordingPusher(), goodSettings())
	first, err := inbox.Accept(testEnvelope("example.com", "example.com/inbound-1"))
	second, again := inbox.Accept(testEnvelope("example.com", "example.com/inbound-1"))
	stored := storedRecord(t, inbox, "example.com/inbound-1")
	check(t, err == nil && again == nil, "accept failed")
	check(t, first == "in_D1" && second == first, "want the same inbound id")
	check(t, stored.State == "accepted" && stored.ReceivedAt == "2026-11-10T22:00:00Z", "wrong record")
	check(t, stored.Recipients[0] == "ana@example.com" && stored.FromAddress == "dan@outside.test", "wrong record")
}

func TestAcceptRegistersTheCollectionForTheOutbox(t *testing.T) {
	inbox := testInbox(newMemory(), outbox.NewRecordingPusher(), goodSettings())
	inbox.Accept(testEnvelope("example.com", "example.com/inbound-1"))
	collection, _ := inbox.Collection("example.com")
	check(t, inbox.Registry.Check(collection) == nil, "collection not registered")
	check(t, collection[:len("inbound-raw-")] == "inbound-raw-", "want the inbound-raw base")
}

func TestAcceptPushesOneEmailReceivedEventAtOnce(t *testing.T) {
	pusher := outbox.NewRecordingPusher()
	inbox := testInbox(newMemory(), pusher, goodSettings())
	inboundId, _ := inbox.Accept(testEnvelope("example.com", "example.com/inbound-1"))
	inbox.Accept(testEnvelope("example.com", "example.com/inbound-1"))
	pushed := pusher.PushedTo("messaging")
	payload := events.EmailReceived{}
	check(t, len(pushed) == 1, "want one push")
	check(t, events.PayloadReader{}.Read(pushed[0], &payload) == nil && payload.InboundId == inboundId, "wrong payload")
	check(t, storedRecord(t, inbox, "example.com/inbound-1").OutboxPending == 0, "the event is still pending")
}

func TestAcceptPushesTheContentSoMessagingNeedsNoCallBack(t *testing.T) {
	memory := newMemory()
	store := newBlobs()
	store.objects["example.com/inbound-1"] = []byte(textMessage())
	pusher := outbox.NewRecordingPusher()
	inbox := testInbox(memory, pusher, goodSettings())
	inbox.Contents = testShelf(memory, store)
	inbox.Accept(testEnvelope("example.com", "example.com/inbound-1"))
	payload := events.EmailReceived{}
	check(t, events.PayloadReader{}.Read(pusher.PushedTo("messaging")[0], &payload) == nil, "payload unreadable")
	check(t, payload.Content != nil && payload.Content.Text == "Lunch at noon?", "want the content in the event")
	check(t, payload.Content.BodyFormat == "text" && len(payload.Content.Attachments) == 0, "wrong content")
}

func TestAcceptKeepsTheContentForALaterGet(t *testing.T) {
	memory := newMemory()
	store := newBlobs()
	inboundId := acceptedMessage(t, memory, store, "example.com/inbound-1", textMessage())
	delete(store.objects, "example.com/inbound-1")
	content, err := testMessageDesk(memory, store, "example.com").Content("Bearer ok", inboundId)
	check(t, err == nil && content.Text == "Lunch at noon?", "want the kept content")
}

func TestAcceptFailsWhenTheRawMessageCannotBeRead(t *testing.T) {
	memory := newMemory()
	pusher := outbox.NewRecordingPusher()
	inbox := testInbox(memory, pusher, goodSettings())
	inbox.Contents = testShelf(memory, newBlobs())
	_, err := inbox.Accept(testEnvelope("example.com", "example.com/inbound-1"))
	check(t, isUnavailable(err, "raw message"), "want raw message")
	check(t, len(pusher.Attempted()) == 0, "no event may leave without its content")
}

func TestARetriedAcceptStagesNoSecondEvent(t *testing.T) {
	pusher := outbox.NewRecordingPusher()
	pusher.FailFor("messaging", outbox.DeliveryFailure{Subscriber: "messaging", Status: 503, Detail: "down"})
	inbox := testInbox(newMemory(), pusher, goodSettings())
	inbox.Accept(testEnvelope("example.com", "example.com/inbound-1"))
	inbox.Accept(testEnvelope("example.com", "example.com/inbound-1"))
	stored := storedRecord(t, inbox, "example.com/inbound-1")
	staged, err := outbox.Published{}.Staged(stored.OutboxFields(), events.Types{}.EmailReceived())
	check(t, err == nil && len(staged) == 1, "want one staged event")
	check(t, stored.EventQueued && len(pusher.Attempted()) == 1, "want one attempt")
}

func TestAcceptRefusesAnInvalidEnvelope(t *testing.T) {
	inbox := testInbox(newMemory(), outbox.NewRecordingPusher(), goodSettings())
	_, err := inbox.Accept(testEnvelope("example.com", "example.org/inbound-1"))
	check(t, invalidField(err) == "rawBlobKey", "want rawBlobKey")
}

func TestAcceptReportsAFailedCollection(t *testing.T) {
	memory := newMemory()
	memory.failOn = "exists"
	_, err := testInbox(memory, outbox.NewRecordingPusher(), goodSettings()).Accept(testEnvelope("example.com", "example.com/inbound-1"))
	check(t, isUnavailable(err, "collection check"), "want collection check")
}

func TestAcceptReportsAFailedInsert(t *testing.T) {
	memory := newMemory()
	inbox := testInbox(memory, outbox.NewRecordingPusher(), goodSettings())
	inbox.Collection("example.com")
	memory.failOn = "insert"
	_, err := inbox.Accept(testEnvelope("example.com", "example.com/inbound-1"))
	check(t, isUnavailable(err, "insert"), "want insert")
}

func TestAcceptReportsAFailedSearch(t *testing.T) {
	memory := newMemory()
	inbox := testInbox(memory, outbox.NewRecordingPusher(), goodSettings())
	collection, _ := inbox.Collection("example.com")
	memory.failPagesIn = collection
	_, err := inbox.Accept(testEnvelope("example.com", "example.com/inbound-1"))
	check(t, isUnavailable(err, "scroll"), "want scroll")
}

func TestAcceptReportsMissingSettingsAfterStoring(t *testing.T) {
	inbox := testInbox(newMemory(), outbox.NewRecordingPusher(), testEnvironment(nil))
	inboundId, err := inbox.Accept(testEnvelope("example.com", "example.com/inbound-1"))
	check(t, inboundId == "in_D1", "want the inbound id")
	check(t, isUnavailable(err, "outbox settings"), "want outbox settings")
}

func TestAcceptInsertsTheRecordWithBothFlagsOff(t *testing.T) {
	memory := newMemory()
	inbox := testInbox(memory, outbox.NewRecordingPusher(), testEnvironment(nil))
	inbox.Accept(testEnvelope("example.com", "example.com/inbound-1"))
	collection, _ := inbox.Collection("example.com")
	row := memory.rows[collection]["D1"]
	check(t, row["outboxWaiting"] == false && row["outboxHasDead"] == false, "want both flags off")
}

func TestAcceptMarksAnUndeliveredEventAsWaiting(t *testing.T) {
	pusher := outbox.NewRecordingPusher()
	pusher.FailFor("messaging", outbox.DeliveryFailure{Subscriber: "messaging", Status: 503, Detail: "down"})
	inbox := testInbox(newMemory(), pusher, goodSettings())
	inbox.Accept(testEnvelope("example.com", "example.com/inbound-1"))
	stored := storedRecord(t, inbox, "example.com/inbound-1")
	check(t, stored.OutboxWaiting && !stored.OutboxHasDead, "want waiting and no dead letters")
}

func TestAcceptReportsAFailedPatch(t *testing.T) {
	memory := newMemory()
	inbox := testInbox(memory, outbox.NewRecordingPusher(), goodSettings())
	inbox.Collection("example.com")
	memory.failOn = "patch"
	_, err := inbox.Accept(testEnvelope("example.com", "example.com/inbound-1"))
	check(t, isUnavailable(err, "patch"), "want patch")
}
