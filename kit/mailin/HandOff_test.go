package mailin

import (
	"errors"
	"testing"
)

func TestAHandOffIsAcceptedAfterTheCopy(t *testing.T) {
	memory, store, domains := newMemory(), newBlobs(), activeDomains("sender.test")
	store.objects["sender.test/inbound-1"] = []byte("raw")
	inbox := handOffInbox(memory, store, domains)
	inboundId, err := inbox.Accept(handOffEnvelope("sender.test/inbound-1"))
	copied := BlobKeys{}.HandOffCopy("example.com", "sender.test/inbound-1")
	check(t, err == nil && inboundId == "in_D1", "accept failed")
	check(t, string(store.objects[copied.Path()]) == "raw", "the blob was not copied")
	check(t, storedRecord(t, inbox, copied.Path()).RawBlobKey == copied.Path(), "want the recipient key stored")
	check(t, domains.asked[0] == "sender.test", "want the sender domain checked")
}

func TestARetriedHandOffStoresTheMessageOnce(t *testing.T) {
	memory, store := newMemory(), newBlobs()
	store.objects["sender.test/inbound-1"] = []byte("raw")
	inbox := handOffInbox(memory, store, activeDomains("sender.test"))
	first, _ := inbox.Accept(handOffEnvelope("sender.test/inbound-1"))
	second, err := inbox.Accept(handOffEnvelope("sender.test/inbound-1"))
	check(t, err == nil && first == second, "want the same inbound id")
}

func TestAHandOffFromAnInactiveDomainIsRefused(t *testing.T) {
	store, domains := newBlobs(), activeDomains()
	domains.domains["sender.test"] = Domain{DomainId: "sender.test", State: "disabled"}
	store.objects["sender.test/inbound-1"] = []byte("raw")
	_, err := handOffInbox(newMemory(), store, domains).Accept(handOffEnvelope("sender.test/inbound-1"))
	check(t, invalidField(err) == "envelopeFrom", "want envelopeFrom")
	check(t, len(store.objects) == 1, "nothing may be copied")
}

func TestAHandOffFromAnUnknownDomainIsRefused(t *testing.T) {
	_, err := handOffInbox(newMemory(), newBlobs(), activeDomains()).Accept(handOffEnvelope("sender.test/inbound-1"))
	check(t, invalidField(err) == "envelopeFrom", "want envelopeFrom")
}

func TestADirectoryFailureMakesTheHandOffUnavailable(t *testing.T) {
	domains := activeDomains("sender.test")
	domains.failure = errors.New("down")
	_, err := handOffInbox(newMemory(), newBlobs(), domains).Accept(handOffEnvelope("sender.test/inbound-1"))
	check(t, isUnavailable(err, "directory"), "want directory unavailable")
}

func TestAHandOffWithoutTheSenderBlobIsRefused(t *testing.T) {
	_, err := handOffInbox(newMemory(), newBlobs(), activeDomains("sender.test")).Accept(handOffEnvelope("sender.test/inbound-1"))
	check(t, invalidField(err) == "rawBlobKey", "want rawBlobKey")
}

func TestABlobFailureMakesTheHandOffUnavailable(t *testing.T) {
	store := newBlobs()
	store.failOn = "copy"
	_, err := handOffInbox(newMemory(), store, activeDomains("sender.test")).Accept(handOffEnvelope("sender.test/inbound-1"))
	check(t, isUnavailable(err, "blob copy"), "want blob copy unavailable")
}

func TestAnEdgeMessageIsNotCopied(t *testing.T) {
	store, domains := newBlobs(), activeDomains()
	envelope, err := HandOff{Domains: domains, Blobs: store}.Received(testEnvelope("example.com", "example.com/inbound-1"), "example.com")
	check(t, err == nil && envelope.RawBlobKey == "example.com/inbound-1", "the key changed")
	check(t, len(domains.asked) == 0, "the directory was asked")
}
