package mailin

import (
	"slices"
	"testing"
)

func TestRegisterAddsACollectionOnce(t *testing.T) {
	memory := newMemory()
	first := memory.registry().Register("inbound-raw-a")
	second := memory.registry().Register("inbound-raw-a")
	known, err := memory.registry().All()
	check(t, first == nil && second == nil && err == nil, "register failed")
	check(t, slices.Equal(known, []string{"inbound-raw-a"}), "want one name")
}

func TestRegisterReportsAFailedRead(t *testing.T) {
	memory := newMemory()
	memory.failOn = "exists"
	check(t, isUnavailable(memory.registry().Register("inbound-raw-a"), "collection check"), "want collection check")
}

func TestCheckRefusesAnUnknownCollection(t *testing.T) {
	memory := newMemory()
	check(t, isMissing(memory.registry().Check("inbound-raw-a")), "want missing")
	check(t, Missing{Message: "gone"}.Error() == "gone", "wrong text")
}

func TestCheckAcceptsARegisteredCollection(t *testing.T) {
	memory := newMemory()
	memory.registry().Register("inbound-raw-a")
	check(t, memory.registry().Check("inbound-raw-a") == nil, "want no error")
}

func TestCheckReportsAFailedRead(t *testing.T) {
	memory := newMemory()
	memory.failOn = "create"
	check(t, isUnavailable(memory.registry().Check("inbound-raw-a"), "create collection"), "want create collection")
}

func TestTheRegistryNameIsOutboxCollections(t *testing.T) {
	check(t, CollectionRegistry{}.Name() == "outbox-collections", "wrong name")
}
