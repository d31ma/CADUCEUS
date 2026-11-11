package mailin

import (
	"testing"

	"mesail.dev/packages/outbox"
)

func seededStore(t *testing.T) (*memoryDocuments, OutboxStore) {
	memory := newMemory()
	store := memory.store()
	memory.storage().Ensure("inbound-raw-a")
	store.Registry.Register("inbound-raw-a")
	memory.storage().Insert("inbound-raw-a", OutboxPatch{}.From(outbox.Fields{PendingEvents: []string{}, DeadEvents: []string{}}))
	memory.storage().Insert("inbound-raw-a", OutboxPatch{}.From(outbox.Fields{OutboxPending: 1}))
	memory.storage().Insert("inbound-raw-a", OutboxPatch{}.From(outbox.Fields{OutboxPending: 1, OutboxDead: 1}))
	return memory, store
}

func TestPendingKeepsWaitingRowsUpToTheLimit(t *testing.T) {
	_, store := seededStore(t)
	all, err := store.Pending(10)
	limited, _ := store.Pending(1)
	check(t, err == nil, "pending failed")
	check(t, len(all) == 2, "want two waiting rows")
	check(t, len(limited) == 1 && limited[0].Ref.Id == "D2", "want the first waiting row")
}

func TestPendingStopsReadingAtTheLimit(t *testing.T) {
	memory, store := seededStore(t)
	memory.storage().Ensure("inbound-raw-b")
	store.Registry.Register("inbound-raw-b")
	memory.storage().Insert("inbound-raw-b", OutboxPatch{}.From(outbox.Fields{OutboxPending: 1}))
	memory.pageReads = map[string]int{}
	limited, err := store.Pending(1)
	check(t, err == nil && len(limited) == 1, "want one row")
	check(t, memory.pageReads["inbound-raw-a"] == 1, "want one page of the first collection")
	check(t, memory.pageReads["inbound-raw-b"] == 0, "the second collection was read")
}

func TestPendingReadsTheNextCollectionUntilTheLimit(t *testing.T) {
	memory, store := seededStore(t)
	memory.storage().Ensure("inbound-raw-b")
	store.Registry.Register("inbound-raw-b")
	memory.storage().Insert("inbound-raw-b", OutboxPatch{}.From(outbox.Fields{OutboxPending: 1}))
	pending, err := store.Pending(3)
	check(t, err == nil && len(pending) == 3, "want three rows")
	check(t, pending[2].Ref == outbox.Ref{Collection: "inbound-raw-b", Id: "D1"}, "want the row of the second collection")
}

func TestPendingIgnoresRowsWithoutTheFlag(t *testing.T) {
	memory, store := seededStore(t)
	memory.storage().Insert("inbound-raw-a", OutboxRow{OutboxPending: 1})
	pending, err := store.Pending(10)
	check(t, err == nil && len(pending) == 2, "want only the flagged rows")
}

func TestDeadKeepsRowsWithDeadLetters(t *testing.T) {
	_, store := seededStore(t)
	dead, err := store.Dead()
	check(t, err == nil && len(dead) == 1, "want one dead row")
	check(t, dead[0].Ref == outbox.Ref{Collection: "inbound-raw-a", Id: "D3"}, "wrong ref")
	check(t, dead[0].Fields.OutboxDead == 1, "wrong fields")
}

func TestPendingReportsAFailedRegistry(t *testing.T) {
	memory, store := seededStore(t)
	memory.failOn = "exists"
	documents, err := store.Pending(10)
	check(t, err != nil && len(documents) == 0, "want an error and no rows")
}

func TestPendingSurvivesARegisteredCollectionThatIsMissing(t *testing.T) {
	memory, store := seededStore(t)
	store.Registry.Register("inbound-raw-missing")
	documents, err := store.Pending(10)
	check(t, err == nil && len(documents) == 2, "the sweep failed")
	check(t, memory.created == 3, "the missing collection was not created")
}

func TestPendingReportsACollectionThatCannotBeCreated(t *testing.T) {
	memory, store := seededStore(t)
	store.Registry.Register("inbound-raw-missing")
	memory.failOn = "create"
	documents, err := store.Pending(10)
	check(t, isUnavailable(err, "create collection"), "want create collection")
	check(t, len(documents) == 2, "want the readable rows")
}

func TestLoadReadsTheFields(t *testing.T) {
	_, store := seededStore(t)
	fields, err := store.Load(outbox.Ref{Collection: "inbound-raw-a", Id: "D3"})
	check(t, err == nil && fields.OutboxPending == 1 && fields.OutboxDead == 1, "wrong fields")
}

func TestLoadRefusesAnUnknownCollection(t *testing.T) {
	_, store := seededStore(t)
	_, err := store.Load(outbox.Ref{Collection: "inbound-raw-b", Id: "D1"})
	check(t, isMissing(err), "want missing")
}

func TestLoadRefusesAMissingDocument(t *testing.T) {
	_, store := seededStore(t)
	_, err := store.Load(outbox.Ref{Collection: "inbound-raw-a", Id: "D9"})
	check(t, isMissing(err), "want missing")
}

func TestLoadReportsAFailedFind(t *testing.T) {
	memory, store := seededStore(t)
	memory.failOn = "find"
	_, err := store.Load(outbox.Ref{Collection: "inbound-raw-a", Id: "D1"})
	check(t, isUnavailable(err, "find"), "want find")
}

func TestSaveWritesTheFields(t *testing.T) {
	_, store := seededStore(t)
	err := store.Save(outbox.Ref{Collection: "inbound-raw-a", Id: "D2"}, outbox.Fields{PendingEvents: []string{}, DeadEvents: []string{}})
	fields, _ := store.Load(outbox.Ref{Collection: "inbound-raw-a", Id: "D2"})
	check(t, err == nil && fields.OutboxPending == 0, "save failed")
}

func TestSaveSetsTheFlagsInOnePatch(t *testing.T) {
	memory, store := seededStore(t)
	ref := outbox.Ref{Collection: "inbound-raw-a", Id: "D1"}
	err := store.Save(ref, outbox.Fields{PendingEvents: []string{"e"}, DeadEvents: []string{"d"}, OutboxPending: 1, OutboxDead: 1})
	row := memory.rows["inbound-raw-a"]["D1"]
	check(t, err == nil && memory.patches == 1, "want one patch")
	check(t, row["outboxWaiting"] == true && row["outboxHasDead"] == true, "want both flags set")
	check(t, row["outboxPending"] == float64(1) && row["outboxDead"] == float64(1), "want the counts")
}

func TestSaveClearsTheFlags(t *testing.T) {
	memory, store := seededStore(t)
	err := store.Save(outbox.Ref{Collection: "inbound-raw-a", Id: "D3"}, outbox.Fields{PendingEvents: []string{}, DeadEvents: []string{}})
	row := memory.rows["inbound-raw-a"]["D3"]
	check(t, err == nil && row["outboxWaiting"] == false && row["outboxHasDead"] == false, "want both flags cleared")
}

func TestSaveRefusesAnUnknownCollection(t *testing.T) {
	_, store := seededStore(t)
	check(t, isMissing(store.Save(outbox.Ref{Collection: "inbound-raw-b", Id: "D1"}, outbox.Fields{})), "want missing")
}
