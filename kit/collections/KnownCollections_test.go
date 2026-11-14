package collections

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type countingStore struct {
	existing     map[string]bool
	checks       int
	creates      int
	failChecks   bool
	failCreates  bool
	createsLater bool
}

func newCountingStore(names ...string) *countingStore {
	store := &countingStore{existing: map[string]bool{}}
	for _, name := range names {
		store.existing[name] = true
	}
	return store
}

func (store *countingStore) CollectionExists(name string) (bool, error) {
	store.checks++
	if store.failChecks {
		return false, errors.New("store is not available")
	}
	return store.existing[name], nil
}

func (store *countingStore) CreateCollection(name string) error {
	store.creates++
	if store.createsLater {
		store.existing[name] = true
	}
	if store.failCreates {
		return errors.New("create failed")
	}
	store.existing[name] = true
	return nil
}

func (store *countingStore) missingError(name string) error {
	if !store.existing[name] {
		return errors.New("collection not found")
	}
	return nil
}

func knownIn(t *testing.T) KnownCollections {
	return KnownCollections{Folder: filepath.Join(t.TempDir(), "known")}
}

func TestFirstCheckAsksTheStoreAndLaterChecksDoNot(t *testing.T) {
	known, store := knownIn(t), newCountingStore("members")
	first, err := known.Exists(store, "members")
	second, _ := known.Exists(store, "members")
	third, _ := known.Exists(store, "members")
	if err != nil || !first || !second || !third || store.checks != 1 {
		t.Fatalf("exists %v %v %v, err %v, checks %d", first, second, third, err, store.checks)
	}
}

func TestAnotherValueWithTheSameFolderSharesTheMarkers(t *testing.T) {
	folder := filepath.Join(t.TempDir(), "known")
	store := newCountingStore("members")
	KnownCollections{Folder: folder}.Exists(store, "members")
	exists, _ := KnownCollections{Folder: folder}.Exists(store, "members")
	if !exists || store.checks != 1 {
		t.Fatalf("exists %v, checks %d", exists, store.checks)
	}
}

func TestAMissingCollectionIsAskedAgain(t *testing.T) {
	known, store := knownIn(t), newCountingStore()
	known.Exists(store, "members")
	exists, _ := known.Exists(store, "members")
	if exists || store.checks != 2 {
		t.Fatalf("exists %v, checks %d", exists, store.checks)
	}
}

func TestAFailedCheckIsReportedAndNotRemembered(t *testing.T) {
	known, store := knownIn(t), newCountingStore("members")
	store.failChecks = true
	_, err := known.Exists(store, "members")
	store.failChecks = false
	exists, _ := known.Exists(store, "members")
	if err == nil || !exists || store.checks != 2 {
		t.Fatalf("err %v, exists %v, checks %d", err, exists, store.checks)
	}
}

func TestEnsureCreatesOnceAndThenNeedsNoStoreCall(t *testing.T) {
	known, store := knownIn(t), newCountingStore()
	first := known.Ensure(store, "members")
	second := known.Ensure(store, "members")
	if first != nil || second != nil || store.creates != 1 || store.checks != 1 {
		t.Fatalf("errors %v %v, creates %d, checks %d", first, second, store.creates, store.checks)
	}
}

func TestEnsureOfAnExistingCollectionDoesNotCreate(t *testing.T) {
	known, store := knownIn(t), newCountingStore("members")
	err := known.Ensure(store, "members")
	if err != nil || store.creates != 0 || store.checks != 1 {
		t.Fatalf("err %v, creates %d, checks %d", err, store.creates, store.checks)
	}
}

func TestEnsureReportsAFailedCheck(t *testing.T) {
	known, store := knownIn(t), newCountingStore()
	store.failChecks = true
	err := known.Ensure(store, "members")
	if err == nil || store.creates != 0 {
		t.Fatalf("err %v, creates %d", err, store.creates)
	}
}

func TestEnsureAcceptsACollectionThatAnotherCallCreatedFirst(t *testing.T) {
	known, store := knownIn(t), newCountingStore()
	store.failCreates, store.createsLater = true, true
	err := known.Ensure(store, "members")
	if err != nil || store.checks != 2 {
		t.Fatalf("err %v, checks %d", err, store.checks)
	}
}

func TestEnsureReportsACreateThatFailed(t *testing.T) {
	known, store := knownIn(t), newCountingStore()
	store.failCreates = true
	store.existing = map[string]bool{}
	err := known.Ensure(store, "members")
	if err == nil {
		t.Fatal("a failed create must be reported")
	}
}

func TestForgetMakesTheNextCheckAskTheStore(t *testing.T) {
	known, store := knownIn(t), newCountingStore("members")
	known.Exists(store, "members")
	known.Forget("members")
	delete(store.existing, "members")
	exists, _ := known.Exists(store, "members")
	if exists || store.checks != 2 {
		t.Fatalf("exists %v, checks %d", exists, store.checks)
	}
}

func TestForgetAllForgetsEveryCollection(t *testing.T) {
	known, store := knownIn(t), newCountingStore("members", "domains")
	known.Exists(store, "members")
	known.Exists(store, "domains")
	known.ForgetAll()
	known.Exists(store, "members")
	known.Exists(store, "domains")
	if store.checks != 4 {
		t.Fatalf("checks %d", store.checks)
	}
}

func TestReadOfAMissingCollectionAnswersEmptyWithoutCreating(t *testing.T) {
	known, store := knownIn(t), newCountingStore()
	reads := 0
	err := known.Read(store, "members", func() error { reads++; return nil })
	if err != nil || reads != 0 || store.creates != 0 || store.existing["members"] {
		t.Fatalf("err %v, reads %d, creates %d", err, reads, store.creates)
	}
}

func TestReadOfAnExistingCollectionReadsAndIsCheckedOnce(t *testing.T) {
	known, store := knownIn(t), newCountingStore("members")
	reads := 0
	known.Read(store, "members", func() error { reads++; return nil })
	err := known.Read(store, "members", func() error { reads++; return nil })
	if err != nil || reads != 2 || store.checks != 1 {
		t.Fatalf("err %v, reads %d, checks %d", err, reads, store.checks)
	}
}

func TestReadReportsAFailedCheck(t *testing.T) {
	known, store := knownIn(t), newCountingStore("members")
	store.failChecks = true
	err := known.Read(store, "members", func() error { return nil })
	if err == nil {
		t.Fatal("a failed check must be reported")
	}
}

func TestReadAfterAWipeForgetsTheMarkerAndAnswersEmpty(t *testing.T) {
	known, store := knownIn(t), newCountingStore("members")
	known.Exists(store, "members")
	delete(store.existing, "members")
	err := known.Read(store, "members", func() error { return store.missingError("members") })
	exists, _ := known.Exists(store, "members")
	if err != nil || exists || store.checks != 3 || store.creates != 0 {
		t.Fatalf("err %v, exists %v, checks %d, creates %d", err, exists, store.checks, store.creates)
	}
}

func TestReadFailureOfAnExistingCollectionIsReportedAndKeepsTheMarker(t *testing.T) {
	known, store := knownIn(t), newCountingStore("members")
	err := known.Read(store, "members", func() error { return errors.New("deadline") })
	known.Exists(store, "members")
	if err == nil || store.checks != 2 {
		t.Fatalf("err %v, checks %d", err, store.checks)
	}
}

func TestWriteCreatesTheCollectionOnceAndWrites(t *testing.T) {
	known, store := knownIn(t), newCountingStore()
	writes := 0
	first := known.Write(store, "members", func() error { writes++; return nil })
	second := known.Write(store, "members", func() error { writes++; return nil })
	if first != nil || second != nil || writes != 2 || store.creates != 1 || store.checks != 1 {
		t.Fatalf("errors %v %v, writes %d, creates %d, checks %d", first, second, writes, store.creates, store.checks)
	}
}

func TestWriteReportsAFailedCheckWithoutWriting(t *testing.T) {
	known, store := knownIn(t), newCountingStore()
	store.failChecks = true
	writes := 0
	err := known.Write(store, "members", func() error { writes++; return nil })
	if err == nil || writes != 0 {
		t.Fatalf("err %v, writes %d", err, writes)
	}
}

func TestWriteAfterAWipeCreatesAgainAndRetriesOnce(t *testing.T) {
	known, store := knownIn(t), newCountingStore("members")
	known.Exists(store, "members")
	delete(store.existing, "members")
	writes := 0
	err := known.Write(store, "members", func() error { writes++; return store.missingError("members") })
	if err != nil || writes != 2 || store.creates != 1 {
		t.Fatalf("err %v, writes %d, creates %d", err, writes, store.creates)
	}
}

func TestWriteFailureOfAnExistingCollectionIsNotRetried(t *testing.T) {
	known, store := knownIn(t), newCountingStore("members")
	writes := 0
	err := known.Write(store, "members", func() error { writes++; return errors.New("deadline") })
	if err == nil || writes != 1 || store.creates != 0 {
		t.Fatalf("err %v, writes %d, creates %d", err, writes, store.creates)
	}
}

func TestWriteRetryReportsAFailedCreate(t *testing.T) {
	known, store := knownIn(t), newCountingStore("members")
	known.Exists(store, "members")
	delete(store.existing, "members")
	store.failCreates = true
	writes := 0
	err := known.Write(store, "members", func() error { writes++; return store.missingError("members") })
	if err == nil || writes != 1 {
		t.Fatalf("err %v, writes %d", err, writes)
	}
}

func TestANameFyloWouldRefuseIsNeverMarked(t *testing.T) {
	known, store := knownIn(t), newCountingStore("../escape")
	known.Exists(store, "../escape")
	known.Exists(store, "../escape")
	if store.checks != 2 {
		t.Fatalf("checks %d", store.checks)
	}
}

func TestTheContainerFolderDependsOnTheAppSettings(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	t.Setenv("YON_APPLICATION_JSON", `{"server":{"port":8107}}`)
	first := KnownCollections{}.folder()
	t.Setenv("YON_APPLICATION_JSON", `{"server":{"port":8110}}`)
	second := KnownCollections{}.folder()
	if first == second || filepath.Dir(first) != filepath.Join(os.TempDir(), "mesail-known-collections") {
		t.Fatalf("folders %q %q", first, second)
	}
}

func TestMarkersHoldNoData(t *testing.T) {
	known, store := knownIn(t), newCountingStore("members")
	known.Exists(store, "members")
	info, err := os.Stat(filepath.Join(known.Folder, "members"))
	if err != nil || info.Size() != 0 {
		t.Fatalf("err %v, info %v", err, info)
	}
}

func TestReadFailureAnswersNothingWhenTheCollectionIsGone(t *testing.T) {
	known, store := knownIn(t), newCountingStore("members")
	known.Exists(store, "members")
	delete(store.existing, "members")
	err := known.ReadFailure(store, "members", errors.New("collection not found"))
	exists, _ := known.Exists(store, "members")
	if err != nil || exists || store.checks != 3 {
		t.Fatalf("err %v, exists %v, checks %d", err, exists, store.checks)
	}
}

func TestReadFailureWithoutAFailureAsksNothing(t *testing.T) {
	known, store := knownIn(t), newCountingStore("members")
	err := known.ReadFailure(store, "members", nil)
	if err != nil || store.checks != 0 {
		t.Fatalf("err %v, checks %d", err, store.checks)
	}
}
