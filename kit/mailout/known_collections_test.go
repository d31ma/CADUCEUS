package mailout

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"mesail.dev/packages/events"
	"mesail.dev/packages/outbox"
)

var knownCollectionsBase = filepath.Join(os.TempDir(), "mesail-kit-tests")

func isolateKnownCollections() {
	os.MkdirAll(knownCollectionsBase, 0o700)
	folder, _ := os.MkdirTemp(knownCollectionsBase, "mailout")
	os.Setenv("TMPDIR", folder)
}

func TestEachCollectionIsCreatedOnceAndNotCheckedAfterTheFirstCall(t *testing.T) {
	documents := newMemoryDocuments()
	store := DocumentStore{Documents: documents}
	names := []string{store.Name(Queue{}.Base(), "example.com"), Registry{}.collection(), store.Name(AttemptLog{}.base(), "example.com"), store.Name(SeenEvents{}.base(), "example.com")}
	writeMany(t, store, 0, 1)
	first := checksOf(documents, names)
	writeMany(t, store, 1, 100)
	equal(t, first, checksOf(documents, names))
	equal(t, []int{1, 2, 1, 1}, first)
	equal(t, []int{1, 1, 1, 1}, createsOf(documents, names))
}

func TestAReadOfAMissingCollectionAnswersEmptyWithoutCreatingIt(t *testing.T) {
	documents := newMemoryDocuments()
	store := DocumentStore{Documents: documents}
	suppressed, containsErr := Suppressions{Store: store}.Contains("example.com", "a@b.c")
	entries, collectionsErr := Registry{Store: store}.Collections(Queue{}.Base())
	_, suppressionCreates := documents.calls(store.Name(Suppressions{}.base(), "example.com"))
	_, registryCreates := documents.calls(Registry{}.collection())
	equal(t, nil, containsErr)
	equal(t, false, suppressed)
	equal(t, nil, collectionsErr)
	equal(t, 0, len(entries))
	equal(t, 0, suppressionCreates)
	equal(t, 0, registryCreates)
}

func writeMany(t *testing.T, store DocumentStore, from int, to int) {
	queue := Queue{Store: store, Registry: Registry{Store: store}, Outbox: outbox.Settings{Producer: "mail-out"}}
	attempts := AttemptLog{Store: store}
	seen := SeenEvents{Store: store, Now: time.Now}
	for index := from; index < to; index++ {
		change := events.Envelope{EventID: "e" + strconv.Itoa(index), Type: events.Types{}.EmailDeliveryChanged(), DomainId: "example.com", Producer: "mail-out", OccurredAt: Clock{}.Format(time.Now()), Payload: []byte(`{}`)}
		_, err := queue.Insert(Item{DomainId: "example.com", State: "queued"}, change)
		equal(t, nil, err)
		equal(t, nil, attempts.Record(DeliveryAttempt{DomainId: "example.com", ItemId: "item"}))
		equal(t, nil, seen.Remember("example.com", change.EventID))
	}
}

func checksOf(documents *memoryDocuments, names []string) []int {
	counts := []int{}
	for _, name := range names {
		checks, _ := documents.calls(name)
		counts = append(counts, checks)
	}
	return counts
}

func createsOf(documents *memoryDocuments, names []string) []int {
	counts := []int{}
	for _, name := range names {
		_, creates := documents.calls(name)
		counts = append(counts, creates)
	}
	return counts
}
