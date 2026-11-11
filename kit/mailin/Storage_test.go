package mailin

import "testing"

func TestEnsureCreatesTheCollectionOnce(t *testing.T) {
	memory := newMemory()
	first := memory.storage().Ensure("letters")
	second := memory.storage().Ensure("letters")
	check(t, first == nil && second == nil, "ensure failed")
	check(t, memory.created == 1, "the collection was created twice")
}

func TestEnsureReportsAFailedCheck(t *testing.T) {
	memory := newMemory()
	memory.failOn = "exists"
	check(t, isUnavailable(memory.storage().Ensure("letters"), "collection check"), "want collection check")
}

func TestEnsureReportsAFailedCreate(t *testing.T) {
	memory := newMemory()
	memory.failOn = "create"
	check(t, isUnavailable(memory.storage().Ensure("letters"), "create collection"), "want create collection")
}

func TestInsertFindAndReplaceReportFailures(t *testing.T) {
	memory := newMemory()
	memory.storage().Ensure("letters")
	memory.failOn = "insert"
	_, inserted := memory.storage().Insert("letters", CollectionRecord{Name: "a"})
	memory.failOn = "find"
	_, found := memory.storage().Find("letters", "D1", &CollectionRecord{})
	memory.failOn = "patch"
	replaced := memory.storage().Replace("letters", "D1", CollectionRecord{Name: "b"})
	check(t, isUnavailable(inserted, "insert"), "want insert")
	check(t, isUnavailable(found, "find"), "want find")
	check(t, isUnavailable(replaced, "patch"), "want patch")
}

func TestAllReadsEveryPage(t *testing.T) {
	memory := newMemory()
	storage := memory.storage()
	storage.Ensure("letters")
	storage.Insert("letters", CollectionRecord{Name: "a"})
	storage.Insert("letters", CollectionRecord{Name: "b"})
	storage.Insert("letters", CollectionRecord{Name: "a"})
	records := []CollectionRecord{}
	err := storage.All("letters", map[string]any{"name": "a"}, &records)
	check(t, err == nil, "all failed")
	check(t, len(records) == 2, "want two records")
	check(t, records[1].Id == "D3", "want the id from the page")
}

func TestAllOfAnEmptyCollectionIsAnEmptyList(t *testing.T) {
	memory := newMemory()
	memory.storage().Ensure("letters")
	records := []CollectionRecord{}
	err := memory.storage().All("letters", map[string]any{}, &records)
	check(t, err == nil && records != nil && len(records) == 0, "want an empty list")
}

func TestAllReportsAFailedPage(t *testing.T) {
	memory := newMemory()
	memory.failOn = "page"
	check(t, isUnavailable(memory.storage().All("letters", map[string]any{}, &[]CollectionRecord{}), "scroll"), "want scroll")
}

func TestAllReportsUnreadableContent(t *testing.T) {
	memory := newMemory()
	memory.badContent = true
	check(t, isUnavailable(memory.storage().All("letters", map[string]any{}, &[]CollectionRecord{}), "scroll"), "want scroll")
}

func TestAllReportsATargetThatDoesNotFit(t *testing.T) {
	memory := newMemory()
	memory.storage().Ensure("letters")
	memory.storage().Insert("letters", CollectionRecord{Name: "a"})
	target := []int{}
	check(t, isUnavailable(memory.storage().All("letters", map[string]any{}, &target), "decode"), "want decode")
}

func TestUnavailableNamesTheOperation(t *testing.T) {
	check(t, Unavailable{Operation: "insert"}.Error() == "inbound storage failed: insert", "wrong text")
}
