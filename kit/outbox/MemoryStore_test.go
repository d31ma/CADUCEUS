package outbox

import (
	"errors"
	"testing"
)

func TestMemoryStoreListsPendingDocumentsOldestFirstUpToTheLimit(t *testing.T) {
	store := NewMemoryStore()
	first := Ref{Collection: "channels", Id: "1"}
	second := Ref{Collection: "channels", Id: "2"}
	store.Put(first, Fields{OutboxPending: 1, OutboxWaiting: true})
	store.Put(Ref{Collection: "channels", Id: "done"}, Fields{})
	store.Put(second, Fields{OutboxPending: 2, OutboxWaiting: true})
	store.Put(first, Fields{OutboxPending: 3, OutboxWaiting: true})

	all, err := store.Pending(10)
	one, _ := store.Pending(1)
	none, _ := store.Pending(-1)

	if err != nil || len(all) != 2 || all[0].Ref != first || all[0].Fields.OutboxPending != 3 || all[1].Ref != second {
		t.Fatalf("documents %+v, error %v", all, err)
	}
	if len(one) != 1 || len(none) != 0 {
		t.Fatalf("limited %+v and %+v", one, none)
	}
}

func TestMemoryStoreLoadsAndSavesFields(t *testing.T) {
	store := NewMemoryStore()
	ref := Ref{Collection: "channels", Id: "1"}

	err := store.Save(ref, Fields{OutboxDead: 1})
	fields, loadErr := store.Load(ref)

	if err != nil || loadErr != nil || fields.OutboxDead != 1 || store.Fields(ref).OutboxDead != 1 {
		t.Fatalf("fields %+v, errors %v %v", fields, err, loadErr)
	}
}

func TestMemoryStoreReportsAMissingDocument(t *testing.T) {
	_, err := NewMemoryStore().Load(Ref{Collection: "channels", Id: "9"})

	if err == nil || err.Error() != "the document channels/9 was not found" {
		t.Fatalf("error %v", err)
	}
}

func TestMemoryStoreFailsOnlyTheNextCall(t *testing.T) {
	store := NewMemoryStore()
	ref := Ref{Collection: "channels", Id: "1"}
	store.Put(ref, Fields{})
	store.FailNextSave(errors.New("save failed"))
	store.FailNextLoad(errors.New("load failed"))
	store.FailNextPending(errors.New("pending failed"))

	saveErr := store.Save(ref, Fields{})
	_, loadErr := store.Load(ref)
	_, pendingErr := store.Pending(1)
	if saveErr == nil || loadErr == nil || pendingErr == nil {
		t.Fatalf("errors %v %v %v", saveErr, loadErr, pendingErr)
	}
	_, loadAgain := store.Load(ref)
	if store.Save(ref, Fields{}) != nil || loadAgain != nil {
		t.Fatal("the failures should apply once")
	}
}
