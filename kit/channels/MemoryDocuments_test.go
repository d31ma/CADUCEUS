package channels

import (
	"math"
	"testing"
)

func TestMemoryDocumentsRefuseACollectionThatWasNeverEnsured(t *testing.T) {
	documents := NewMemoryDocuments()
	_, insertErr := documents.Insert("notes", map[string]any{"a": 1})
	_, findErr := documents.Find("notes", "1", &map[string]any{})
	wantContains(t, insertErr.Error(), "FYLO collection notes does not exist")
	wantContains(t, findErr.Error(), "does not exist")
	wantContains(t, documents.All("notes", map[string]any{}, &[]map[string]any{}).Error(), "does not exist")
	wantContains(t, documents.Replace("notes", "1", map[string]any{}).Error(), "does not exist")
}

func TestMemoryDocumentsStoreFindAndPatchADocument(t *testing.T) {
	documents := NewMemoryDocuments()
	wantNoError(t, documents.Ensure("notes"))
	id, err := documents.Insert("notes", map[string]any{"title": "first", "body": "hello"})
	wantNoError(t, err)
	wantNoError(t, documents.Replace("notes", id, map[string]any{"title": "second"}))
	found := map[string]any{}
	exists, err := documents.Find("notes", id, &found)
	wantNoError(t, err)
	wantEqual(t, exists, true)
	wantEqual(t, found, map[string]any{"id": id, "title": "second", "body": "hello"})
	wantEqual(t, documents.Collections(), []string{"notes"})
}

func TestMemoryDocumentsReportAMissingDocument(t *testing.T) {
	documents := NewMemoryDocuments()
	wantNoError(t, documents.Ensure("notes"))
	exists, err := documents.Find("notes", "7", &map[string]any{})
	wantNoError(t, err)
	wantEqual(t, exists, false)
	wantEqual(t, documents.Replace("notes", "7", map[string]any{}).Error(), "document 7 does not exist")
}

func TestMemoryDocumentsReturnOnlyTheDocumentsThatMatchInInsertOrder(t *testing.T) {
	documents := NewMemoryDocuments()
	wantNoError(t, documents.Ensure("notes"))
	_, _ = documents.Insert("notes", map[string]any{"kind": "a", "n": 1})
	_, _ = documents.Insert("notes", map[string]any{"kind": "b", "n": 2})
	_, _ = documents.Insert("notes", map[string]any{"kind": "a", "n": 3})
	found := []map[string]any{}
	wantNoError(t, documents.All("notes", map[string]any{"kind": "a"}, &found))
	wantEqual(t, len(found), 2)
	wantEqual(t, found[1]["n"], float64(3))
}

func TestMemoryDocumentsRefuseADocumentThatIsNotAnObject(t *testing.T) {
	documents := NewMemoryDocuments()
	wantNoError(t, documents.Ensure("notes"))
	_, notJson := documents.Insert("notes", make(chan int))
	_, notObject := documents.Insert("notes", "just text")
	wantTrue(t, notJson != nil, "a channel value was stored")
	wantTrue(t, notObject != nil, "a plain string was stored")
}

func TestMemoryDocumentsReportATargetOfTheWrongShape(t *testing.T) {
	documents := NewMemoryDocuments()
	wantNoError(t, documents.Ensure("notes"))
	_, _ = documents.Insert("notes", map[string]any{"n": 1})
	wantTrue(t, documents.All("notes", map[string]any{}, &[]int{}) != nil, "documents were decoded into numbers")
}

func TestMemoryDocumentsReportAStoredValueThatCannotBeEncoded(t *testing.T) {
	documents := NewMemoryDocuments()
	wantNoError(t, documents.Ensure("notes"))
	id, _ := documents.Insert("notes", map[string]any{"n": 1})
	documents.collections["notes"].documents[id]["n"] = math.Inf(1)
	_, err := documents.Find("notes", id, &map[string]any{})
	wantContains(t, err.Error(), "cannot encode documents")
}

func TestMemoryDocumentsFailTheQueuedCallsInOrder(t *testing.T) {
	documents := NewMemoryDocuments()
	documents.FailNext(nil)
	documents.FailNext(storageDown{})
	wantNoError(t, documents.Ensure("notes"))
	wantEqual(t, documents.Ensure("notes"), error(storageDown{}))
	wantNoError(t, documents.Ensure("notes"))
}
