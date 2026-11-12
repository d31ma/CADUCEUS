package mailin

import (
	"strings"
	"testing"

	"mesail.dev/packages/outbox"
)

func storedMessage(t *testing.T, memory *memoryDocuments, store *memoryBlobs, raw string) InboundRaw {
	store.objects["example.com/inbound-1"] = []byte(raw)
	inbox := testInbox(memory, outbox.NewRecordingPusher(), goodSettings())
	collection, err := inbox.Collection("example.com")
	check(t, err == nil, "collection missing")
	stored, err := inbox.findOrInsert(collection, "example.com", testEnvelope("example.com", "example.com/inbound-1"))
	check(t, err == nil, "message missing")
	return stored
}

func testShelf(memory *memoryDocuments, store *memoryBlobs) ContentShelf {
	return ContentShelf{Storage: memory.storage(), Builder: ContentBuilder{Blobs: store}}
}

func TestTheShelfKeepsTheContentInADomainCollection(t *testing.T) {
	memory, store := newMemory(), newBlobs()
	stored := storedMessage(t, memory, store, textMessage())
	testShelf(memory, store).ForMessage(stored)
	collection := ContentShelf{}.Collection("example.com")
	row := memory.rows[collection]["D1"]
	check(t, strings.HasPrefix(collection, "inbound-content-"), "wrong collection")
	check(t, row["inboundId"] == stored.WireId() && row["bodyFormat"] == "text", "wrong record")
	check(t, strings.HasPrefix(row["blocks"].(string), "[{"), "want the blocks as one JSON text")
	check(t, row["attachments"] == "[]", "want the attachments as one JSON text")
}

func TestTheShelfReportsAFailedCollection(t *testing.T) {
	memory, store := newMemory(), newBlobs()
	stored := storedMessage(t, memory, store, textMessage())
	memory.failOn = "create"
	_, err := testShelf(memory, store).ForMessage(stored)
	check(t, isUnavailable(err, "create collection"), "want create collection")
}

func TestTheShelfReportsAFailedLookup(t *testing.T) {
	memory, store := newMemory(), newBlobs()
	stored := storedMessage(t, memory, store, textMessage())
	memory.failPagesIn = ContentShelf{}.Collection("example.com")
	_, err := testShelf(memory, store).ForMessage(stored)
	check(t, isUnavailable(err, "scroll"), "want scroll")
}

func TestTheShelfReportsAMissingRawMessage(t *testing.T) {
	memory, store := newMemory(), newBlobs()
	stored := storedMessage(t, memory, store, textMessage())
	delete(store.objects, stored.RawBlobKey)
	_, err := testShelf(memory, store).ForMessage(stored)
	check(t, isUnavailable(err, "raw message"), "want raw message")
}

func TestTheShelfReportsAFailedInsert(t *testing.T) {
	memory, store := newMemory(), newBlobs()
	stored := storedMessage(t, memory, store, textMessage())
	testShelf(memory, store).Storage.Ensure(ContentShelf{}.Collection("example.com"))
	memory.failOn = "insert"
	_, err := testShelf(memory, store).ForMessage(stored)
	check(t, isUnavailable(err, "insert"), "want insert")
}

func TestTheShelfReportsAnUnreadableStoredRecord(t *testing.T) {
	memory, store := newMemory(), newBlobs()
	stored := storedMessage(t, memory, store, textMessage())
	collection := ContentShelf{}.Collection("example.com")
	memory.storage().Ensure(collection)
	memory.storage().Insert(collection, ContentRecord{InboundId: stored.WireId(), Blocks: "{", Attachments: "[]"})
	_, err := testShelf(memory, store).ForMessage(stored)
	check(t, isUnavailable(err, "stored content"), "want stored content")
}
