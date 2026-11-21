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
	memory.storage().Insert("inbound-raw-a", outbox.Fields{PendingEvents: []string{}, DeadEvents: []string{}})
	memory.storage().Insert("inbound-raw-a", outbox.Fields{OutboxPending: 1, OutboxWaiting: true})
	memory.storage().Insert("inbound-raw-a", outbox.Fields{OutboxPending: 1, OutboxDead: 1, OutboxWaiting: true, OutboxHasDead: true})
	return memory, store
}

func TestWaitingKeepsEveryWaitingRow(t *testing.T) {
	_, store := seededStore(t)
	all, err := store.Waiting()
	check(t, err == nil, "waiting failed")
	check(t, len(all) == 2 && all[0].Ref.Id == "D2", "want two waiting rows")
}

func TestWaitingReadsEveryCollection(t *testing.T) {
	memory, store := seededStore(t)
	memory.storage().Ensure("inbound-raw-b")
	store.Registry.Register("inbound-raw-b")
	memory.storage().Insert("inbound-raw-b", outbox.Fields{OutboxPending: 1, OutboxWaiting: true})
	waiting, err := store.Waiting()
	check(t, err == nil && len(waiting) == 3, "want three rows")
	check(t, waiting[2].Ref == outbox.Ref{Collection: "inbound-raw-b", Id: "D1"}, "want the row of the second collection")
}

func TestWaitingIgnoresRowsWithoutTheFlag(t *testing.T) {
	memory, store := seededStore(t)
	memory.storage().Insert("inbound-raw-a", OutboxRow{Fields: outbox.Fields{OutboxPending: 1}})
	pending, err := store.Waiting()
	check(t, err == nil && len(pending) == 2, "want only the flagged rows")
}

func TestDeadKeepsRowsWithDeadLetters(t *testing.T) {
	_, store := seededStore(t)
	dead, err := store.Dead()
	check(t, err == nil && len(dead) == 1, "want one dead row")
	check(t, dead[0].Ref == outbox.Ref{Collection: "inbound-raw-a", Id: "D3"}, "wrong ref")
	check(t, dead[0].Fields.OutboxDead == 1, "wrong fields")
}

func TestWaitingReportsAFailedRegistry(t *testing.T) {
	memory, store := seededStore(t)
	memory.failOn = "page"
	documents, err := store.Waiting()
	check(t, err != nil && len(documents) == 0, "want an error and no rows")
}

func TestWaitingSurvivesARegisteredCollectionThatIsMissing(t *testing.T) {
	memory, store := seededStore(t)
	store.Registry.Register("inbound-raw-missing")
	documents, err := store.Waiting()
	check(t, err == nil && len(documents) == 2, "the sweep failed")
	check(t, memory.created == 3, "the missing collection was not created")
}

func TestWaitingReportsACollectionThatCannotBeCreated(t *testing.T) {
	memory, store := seededStore(t)
	store.Registry.Register("inbound-raw-missing")
	memory.failOn = "create"
	documents, err := store.Waiting()
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
	check(t, missingCase(err) == Missing{}.NoOutboxCollection(), "want Missing{}.NoOutboxCollection")
}

func TestLoadRefusesAMissingDocument(t *testing.T) {
	_, store := seededStore(t)
	_, err := store.Load(outbox.Ref{Collection: "inbound-raw-a", Id: "D9"})
	check(t, missingCase(err) == Missing{}.NoOutboxDocument(), "want Missing{}.NoOutboxDocument")
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

func TestSaveWritesTheCodecFlagsInOnePatch(t *testing.T) {
	memory, store := seededStore(t)
	ref := outbox.Ref{Collection: "inbound-raw-a", Id: "D1"}
	fields, _ := outbox.Codec{}.Write(outbox.OutboxState{Pending: []outbox.PendingEvent{{}}, Dead: []outbox.DeadLetter{{}}})
	err := store.Save(ref, fields)
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
	check(t, missingCase(store.Save(outbox.Ref{Collection: "inbound-raw-b", Id: "D1"}, outbox.Fields{})) == Missing{}.NoOutboxCollection(), "want Missing{}.NoOutboxCollection")
}
