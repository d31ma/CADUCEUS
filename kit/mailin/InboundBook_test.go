package mailin

import "testing"

func TestTheBookFindsAMessageOfTheDomain(t *testing.T) {
	memory, store := newMemory(), newBlobs()
	inboundId := acceptedMessage(t, memory, store, "example.com/inbound-1", textMessage())
	stored, err := InboundBook{Storage: memory.storage()}.Find("example.com", inboundId)
	check(t, err == nil && stored.WireId() == inboundId && stored.DomainId == "example.com", "want the message")
}

func TestTheBookHidesAMessageOfAnotherDomain(t *testing.T) {
	memory, store := newMemory(), newBlobs()
	inboundId := acceptedMessage(t, memory, store, "example.com/inbound-1", textMessage())
	_, err := InboundBook{Storage: memory.storage()}.Find("example.org", inboundId)
	check(t, isMissing(err), "want missing")
}

func TestTheBookRefusesAnUnknownId(t *testing.T) {
	memory, store := newMemory(), newBlobs()
	acceptedMessage(t, memory, store, "example.com/inbound-1", textMessage())
	_, err := InboundBook{Storage: memory.storage()}.Find("example.com", "in_D9")
	check(t, isMissing(err), "want missing")
}

func TestTheBookRefusesAnotherKindOfId(t *testing.T) {
	_, err := InboundBook{Storage: newMemory().storage()}.Find("example.com", "att_D1")
	check(t, isMissing(err), "want missing")
}

func TestTheBookRefusesAnIdWithAPath(t *testing.T) {
	_, err := InboundBook{Storage: newMemory().storage()}.Find("example.com", "in_../D1")
	check(t, isMissing(err), "want missing")
}

func TestTheBookReportsAFailedCollectionCheck(t *testing.T) {
	memory := newMemory()
	memory.failOn = "exists"
	_, err := InboundBook{Storage: memory.storage()}.Find("example.com", "in_D1")
	check(t, isUnavailable(err, "collection check"), "want collection check")
}

func TestTheBookReportsAFailedRead(t *testing.T) {
	memory, store := newMemory(), newBlobs()
	inboundId := acceptedMessage(t, memory, store, "example.com/inbound-1", textMessage())
	memory.failOn = "find"
	_, err := InboundBook{Storage: memory.storage()}.Find("example.com", inboundId)
	check(t, isUnavailable(err, "find"), "want find")
}
